// Package httpapi serves the messaging service as HTTP+JSON. The same
// handler is mounted on each Task's vsock listener and on agc's REST
// API. The caller func is the only thing that differs between them.
package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
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

type ParticipantsResponse struct {
	Participants []messaging.ParticipantView `json:"participants"`
}

// DeliverPath is where a sandbox's socket takes agc task deliver (ADR
// 0010 item 2). Only sandbox sockets serve it, for their own task; the
// handler here does not.
const DeliverPath = "/task/deliver"

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

	m, err := a.svc.Post(r.Context(), caller, req.ParticipantID, req.Channel, req.Body, req.ReplyTo)
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

	messages, err := a.svc.Read(r.Context(), caller, query.Get("channel"), after, int(limit))
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

	messages, err := a.svc.Wait(r.Context(), caller, query.Get("channel"), after, timeout)
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

	participants, err := a.svc.Participants(r.Context(), caller, r.URL.Query().Get("channel"))
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
