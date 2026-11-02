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
	m := &msgmodels.Message{}
	req := httpapi.PostRequest{ParticipantID: participantID, Channel: channel, Body: body, ReplyTo: replyTo}

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

// Deliver posts the task's delivery with summary on a sandbox's socket
// (agc task deliver, ADR 0010 item 2) and decodes the task's view into out.
func (c *Client) Deliver(ctx context.Context, summary string, out any) error {
	err := c.do(ctx, http.MethodPost, httpapi.DeliverPath, nil, httpapi.DeliverRequest{Summary: summary}, out)
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

	if res.StatusCode != http.StatusOK {
		failure := httpapi.ErrorResponse{}
		_ = json.NewDecoder(res.Body).Decode(&failure)
		if failure.Error == "" {
			failure.Error = res.Status
		}

		return ez.New(ez.HTTPStatusToError(res.StatusCode), failure.Error, nil)
	}

	err = json.NewDecoder(res.Body).Decode(out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
