// Package httpapi serves the messaging service as HTTP+JSON. The same
// handler is mounted on each Task's vsock listener and on agc's REST
// API. The caller func is the only thing that differs between them.
package httpapi

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// maxRequestBytes leaves room for a 64 KiB body after JSON escaping.
const maxRequestBytes = 1 << 20

type JoinRequest struct {
	Name string `json:"name"`
}

type LeaveRequest struct {
	ParticipantID string `json:"participant_id"`
}

type LeaveResponse struct {
	ParticipantID string `json:"participant_id"`
	Left          bool   `json:"left"`
}

type PostRequest struct {
	ParticipantID string `json:"participant_id"`
	Channel       string `json:"channel"`
	Body          string `json:"body"`
	ReplyTo       *int64 `json:"reply_to,omitempty"`
	// Kind is message (the default when empty) or question.
	Kind string `json:"kind,omitempty"`
}

// MessagesResponse answers read and wait. Cursor is the id to pass as
// after next time: the last message id, or the given after when empty.
type MessagesResponse struct {
	Messages []msgmodels.Message `json:"messages"`
	Cursor   int64               `json:"cursor"`
}

type ChannelsResponse struct {
	Channels []messaging.ChannelInfo `json:"channels"`
}

// QuestionsResponse answers questions: newest first, each with its
// answer when it has one.
type QuestionsResponse struct {
	Questions []messaging.Question `json:"questions"`
}

type ParticipantsResponse struct {
	Participants []messaging.ParticipantView `json:"participants"`
}

// DeliverPath is where a sandbox's socket takes agc task deliver (ADR
// 0010 item 2). Only sandbox sockets serve it, for their own task; the
// handler here does not.
const DeliverPath = "/task/deliver"

// The routes an agent's home serves on its socket besides messaging (ADR
// 0016); only home sockets serve them. The agent loop reads AgentPath on
// every wait cycle and posts AgentSleepPath once idle. agc task create,
// get, accept and reject inside a home use the task routes, since a home
// cannot reach agc rest; that short list is all a manager needs.
const (
	AgentPath      = "/agent"
	AgentSleepPath = "/agent/sleep"
	TasksPath      = "/tasks"
	// TaskPath, TaskAcceptPath and TaskRejectPath are patterns; {name} is
	// the task's name.
	TaskPath       = "/tasks/{name}"
	TaskAcceptPath = "/tasks/{name}/accept"
	TaskRejectPath = "/tasks/{name}/reject"
)

// RejectBody is the body of a reject from an agent's home: why the task
// goes back.
type RejectBody struct {
	Why string `json:"why"`
}

// DeliverRequest is the body of a delivery: what the agent says it did.
type DeliverRequest struct {
	Summary string `json:"summary"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type api struct {
	svc    *messaging.Service
	caller func(*http.Request) (messaging.Caller, error)
}

// New returns the messaging handler. caller resolves who is asking from
// the transport. The request body never says who the sender is.
func New(svc *messaging.Service, caller func(*http.Request) (messaging.Caller, error)) http.Handler {
	a := &api{svc: svc, caller: caller}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /join", a.join)
	mux.HandleFunc("POST /leave", a.leave)
	mux.HandleFunc("POST /post", a.post)
	mux.HandleFunc("GET /read", a.read)
	mux.HandleFunc("GET /wait", a.wait)
	mux.HandleFunc("GET /channels", a.channels)
	mux.HandleFunc("GET /participants", a.participants)
	mux.HandleFunc("GET /participants/{id}", a.getParticipant)
	mux.HandleFunc("GET /questions", a.questions)

	return mux
}

func (a *api) join(w http.ResponseWriter, r *http.Request) {
	var req JoinRequest

	caller, err := a.decode(w, r, &req)
	if err != nil {
		WriteError(w, err)
		return
	}

	p, err := a.svc.Join(r.Context(), caller, req.Name)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, p)
}

func (a *api) leave(w http.ResponseWriter, r *http.Request) {
	var req LeaveRequest

	caller, err := a.decode(w, r, &req)
	if err != nil {
		WriteError(w, err)
		return
	}

	err = a.svc.Leave(r.Context(), caller, req.ParticipantID)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, LeaveResponse{ParticipantID: req.ParticipantID, Left: true})
}

func (a *api) post(w http.ResponseWriter, r *http.Request) {
	var req PostRequest

	caller, err := a.decode(w, r, &req)
	if err != nil {
		WriteError(w, err)
		return
	}

	m, err := a.svc.Post(r.Context(), caller, req.ParticipantID, req.Channel, req.Body, req.ReplyTo, req.Kind)
	if err != nil {
		WriteError(w, err)
		return
	}

	a.svc.MarkSeen(m.ParticipantID, time.Now())

	WriteJSON(w, http.StatusOK, m)
}

func (a *api) read(w http.ResponseWriter, r *http.Request) {
	caller, err := a.caller(r)
	if err != nil {
		WriteError(w, err)
		return
	}

	query := r.URL.Query()

	after, err := parseInt(query.Get("after"), "after")
	if err != nil {
		WriteError(w, err)
		return
	}

	limit, err := parseInt(query.Get("limit"), "limit")
	if err != nil {
		WriteError(w, err)
		return
	}

	messages, err := a.svc.ReadChannels(r.Context(), caller, channelsParam(query), after, int(limit))
	if err != nil {
		WriteError(w, err)
		return
	}

	a.svc.Seen(r.Context(), caller, query.Get("participant"))

	WriteJSON(w, http.StatusOK, newMessagesResponse(messages, after))
}

func (a *api) wait(w http.ResponseWriter, r *http.Request) {
	caller, err := a.caller(r)
	if err != nil {
		WriteError(w, err)
		return
	}

	query := r.URL.Query()

	after, err := parseInt(query.Get("after"), "after")
	if err != nil {
		WriteError(w, err)
		return
	}

	var timeout time.Duration
	if query.Get("timeout") != "" {
		timeout, err = time.ParseDuration(query.Get("timeout"))
		if err != nil {
			WriteError(w, ez.New(ez.EINVALID, "timeout must be a duration such as 30s", err))
			return
		}
	}

	// Mark before blocking too, so a long wait counts as being here.
	a.svc.Seen(r.Context(), caller, query.Get("participant"))

	messages, err := a.svc.WaitChannels(r.Context(), caller, channelsParam(query), after, timeout)
	if err != nil {
		WriteError(w, err)
		return
	}

	a.svc.Seen(r.Context(), caller, query.Get("participant"))

	WriteJSON(w, http.StatusOK, newMessagesResponse(messages, after))
}

func (a *api) channels(w http.ResponseWriter, r *http.Request) {
	caller, err := a.caller(r)
	if err != nil {
		WriteError(w, err)
		return
	}

	channels, err := a.svc.Channels(r.Context(), caller)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, ChannelsResponse{Channels: channels})
}

func (a *api) participants(w http.ResponseWriter, r *http.Request) {
	caller, err := a.caller(r)
	if err != nil {
		WriteError(w, err)
		return
	}

	// ?all=1: everyone who ever joined, in any channel (humans only).
	query := r.URL.Query()
	var participants []messaging.ParticipantView
	if query.Get("all") == "1" {
		participants, err = a.svc.AllParticipants(r.Context(), caller)
	} else {
		participants, err = a.svc.Participants(r.Context(), caller, query.Get("channel"))
	}
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, ParticipantsResponse{Participants: participants})
}

func (a *api) getParticipant(w http.ResponseWriter, r *http.Request) {
	caller, err := a.caller(r)
	if err != nil {
		WriteError(w, err)
		return
	}

	view, err := a.svc.Participant(r.Context(), caller, r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, view)
}

func (a *api) questions(w http.ResponseWriter, r *http.Request) {
	caller, err := a.caller(r)
	if err != nil {
		WriteError(w, err)
		return
	}

	query := r.URL.Query()

	open, err := parseFlag(query.Get("open"), "open")
	if err != nil {
		WriteError(w, err)
		return
	}

	filter := messaging.QuestionsFilter{Channel: query.Get("channel"), Open: open, For: query.Get("for")}

	questions, err := a.svc.Questions(r.Context(), caller, filter)
	if err != nil {
		WriteError(w, err)
		return
	}

	WriteJSON(w, http.StatusOK, QuestionsResponse{Questions: questions})
}

// decode resolves the caller and reads a JSON body into dst.
func (a *api) decode(w http.ResponseWriter, r *http.Request, dst any) (messaging.Caller, error) {
	caller, err := a.caller(r)
	if err != nil {
		return messaging.Caller{}, ez.Wrap(err)
	}

	err = DecodeBody(w, r, dst)
	if err != nil {
		return messaging.Caller{}, ez.Wrap(err)
	}

	return caller, nil
}

// DecodeBody reads a JSON body of at most a megabyte into dst.
func DecodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBytes)).Decode(dst)
	if err != nil {
		return ez.New(ez.EINVALID, "request body must be a JSON object", err)
	}

	return nil
}

// channelsParam is the channels a read or wait names: `channel` (one) and
// `channels` (comma separated, ADR 0016), together, in that order.
func channelsParam(query url.Values) []string {
	channels := []string{}
	if query.Get("channel") != "" {
		channels = append(channels, query.Get("channel"))
	}

	for _, channel := range strings.Split(query.Get("channels"), ",") {
		trimmed := strings.TrimSpace(channel)
		if trimmed != "" {
			channels = append(channels, trimmed)
		}
	}

	return channels
}

func parseInt(value, name string) (int64, error) {
	if value == "" {
		return 0, nil
	}

	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, ez.New(ez.EINVALID, name+" must be an integer", err)
	}

	return n, nil
}

// parseFlag reads a boolean query parameter: 1 or true, 0, false or empty.
func parseFlag(value, name string) (bool, error) {
	switch value {
	case "", "0", "false":
		return false, nil
	case "1", "true":
		return true, nil
	}

	return false, ez.New(ez.EINVALID, name+" must be 1 or 0", nil)
}

func newMessagesResponse(messages []msgmodels.Message, after int64) MessagesResponse {
	cursor := after
	if len(messages) > 0 {
		cursor = messages[len(messages)-1].ID
	}

	return MessagesResponse{Messages: messages, Cursor: cursor}
}

// WriteError answers err as an ErrorResponse with its HTTP status.
func WriteError(w http.ResponseWriter, err error) {
	WriteJSON(w, ez.ErrorToHTTPStatus(err), ErrorResponse{Error: ez.ErrorMessage(err)})
}

// WriteJSON answers value as JSON with status.
func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
