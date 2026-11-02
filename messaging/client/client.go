// Package client calls the messaging HTTP API, over vsock from inside a
// Task VM or over TCP from the host.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func (c *Client) Join(ctx context.Context, name string) (*msgmodels.Participant, error) {
	p := &msgmodels.Participant{}

	err := c.do(ctx, http.MethodPost, "/join", nil, httpapi.JoinRequest{Name: name}, p)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return p, nil
}

func (c *Client) Leave(ctx context.Context, participantID string) (*httpapi.LeaveResponse, error) {
	res := &httpapi.LeaveResponse{}

	err := c.do(ctx, http.MethodPost, "/leave", nil, httpapi.LeaveRequest{ParticipantID: participantID}, res)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return res, nil
}

func (c *Client) Post(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error) {
	return c.post(ctx, httpapi.PostRequest{ParticipantID: participantID, Channel: channel, Body: body, ReplyTo: replyTo})
}

// Ask posts body in channel as a question, which waits on whoever reads
// the channel until someone else replies to it.
func (c *Client) Ask(ctx context.Context, participantID, channel, body string) (*msgmodels.Message, error) {
	return c.post(ctx, httpapi.PostRequest{ParticipantID: participantID, Channel: channel, Body: body, Kind: msgmodels.KindQuestion})
}

func (c *Client) post(ctx context.Context, req httpapi.PostRequest) (*msgmodels.Message, error) {
	m := &msgmodels.Message{}

	err := c.do(ctx, http.MethodPost, "/post", nil, req, m)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return m, nil
}

func (c *Client) Read(ctx context.Context, channel string, after int64, limit int) (*httpapi.MessagesResponse, error) {
	query := url.Values{}
	query.Set("channel", channel)
	query.Set("after", strconv.FormatInt(after, 10))
	query.Set("limit", strconv.Itoa(limit))

	res := &httpapi.MessagesResponse{}

	err := c.do(ctx, http.MethodGet, "/read", query, nil, res)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return res, nil
}

func (c *Client) Wait(ctx context.Context, channel string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error) {
	query := url.Values{}
	query.Set("channel", channel)
	query.Set("after", strconv.FormatInt(after, 10))
	query.Set("timeout", timeout.String())

	res := &httpapi.MessagesResponse{}

	err := c.do(ctx, http.MethodGet, "/wait", query, nil, res)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return res, nil
}

func (c *Client) Channels(ctx context.Context) (*httpapi.ChannelsResponse, error) {
	res := &httpapi.ChannelsResponse{}

	err := c.do(ctx, http.MethodGet, "/channels", nil, nil, res)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return res, nil
}

// Questions lists questions newest first with their answers. channel and
// forName may be empty; open keeps the unanswered ones.
func (c *Client) Questions(ctx context.Context, channel string, open bool, forName string) (*httpapi.QuestionsResponse, error) {
	query := url.Values{}
	if channel != "" {
		query.Set("channel", channel)
	}
	if open {
		query.Set("open", "1")
	}
	if forName != "" {
		query.Set("for", forName)
	}

	res := &httpapi.QuestionsResponse{}

	err := c.do(ctx, http.MethodGet, "/questions", query, nil, res)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return res, nil
}

// Deliver posts the task's delivery with summary on a sandbox's socket
// (agc task deliver, ADR 0010 item 2) and decodes the task's view into out.
func (c *Client) Deliver(ctx context.Context, summary string, out any) error {
	err := c.do(ctx, http.MethodPost, httpapi.DeliverPath, nil, httpapi.DeliverRequest{Summary: summary}, out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// Agent reads GET /agent on an agent's home socket into out: the agent,
// the channels it reads now and its idle (ADR 0016).
func (c *Client) Agent(ctx context.Context, out any) error {
	err := c.do(ctx, http.MethodGet, httpapi.AgentPath, nil, nil, out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// AgentSleep asks agc to stop the agent's home, POST /agent/sleep. agc
// answers before it stops the VM.
func (c *Client) AgentSleep(ctx context.Context) error {
	err := c.do(ctx, http.MethodPost, httpapi.AgentSleepPath, nil, struct{}{}, nil)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// CreateTask creates a task from an agent's home, with the agent as its
// manager; request is a task create request and out gets the task.
func (c *Client) CreateTask(ctx context.Context, request any, out any) error {
	err := c.do(ctx, http.MethodPost, httpapi.TasksPath, nil, request, out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// AcceptTask accepts, from an agent's home, a task the agent manages.
func (c *Client) AcceptTask(ctx context.Context, name string, out any) error {
	path := strings.Replace(httpapi.TaskAcceptPath, "{name}", url.PathEscape(name), 1)

	err := c.do(ctx, http.MethodPost, path, nil, struct{}{}, out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// do sends one request and decodes the JSON answer into out. Error
// answers come back as ez errors with the code the status maps to.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	target := strings.TrimSuffix(c.BaseURL, "/") + path
	if query != nil {
		target += "?" + query.Encode()
	}

	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return ez.Wrap(err)
		}

		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return ez.Wrap(err)
	}

	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return ez.New(ez.EUNAVAILABLE, "agc messaging is unreachable at "+c.BaseURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		failure := httpapi.ErrorResponse{}
		_ = json.NewDecoder(res.Body).Decode(&failure)
		if failure.Error == "" {
			failure.Error = res.Status
		}

		return ez.New(ez.HTTPStatusToError(res.StatusCode), failure.Error, nil)
	}

	// A home's sleep answers 204 with no body.
	if out == nil || res.StatusCode == http.StatusNoContent {
		return nil
	}

	err = json.NewDecoder(res.Body).Decode(out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
