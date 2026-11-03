// Package client calls the messaging HTTP API, over vsock from inside a
// Task VM or over TCP from the host.
package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging/httpapi"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
	// Stall is how long a request may make no progress before it fails
	// (DefaultStall when zero): sending its body, and, for a delivery and
	// its shots, waiting for the answer. It never ends a long poll.
	Stall time.Duration
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

// PostImages is Post with images (ADR 0005 "Images"); body may be empty
// when there are images.
func (c *Client) PostImages(ctx context.Context, participantID, channel, body string, replyTo *int64, images []msgmodels.ImageUpload) (*msgmodels.Message, error) {
	return c.post(ctx, httpapi.PostRequest{ParticipantID: participantID, Channel: channel, Body: body, ReplyTo: replyTo, Images: images})
}

// Image downloads the image called name of message messageID, as the
// caller may read it.
func (c *Client) Image(ctx context.Context, messageID int64, name string) ([]byte, error) {
	path := strings.Replace(httpapi.ImagePath, "{id}", strconv.FormatInt(messageID, 10), 1)
	path = strings.Replace(path, "{name}", url.PathEscape(name), 1)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimSuffix(c.BaseURL, "/")+path, nil)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	res, err := c.httpClient().Do(req)
	if err != nil {
		return nil, ez.New(ez.EUNAVAILABLE, "agc messaging is unreachable at "+c.BaseURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, responseError(res)
	}

	data, err := io.ReadAll(io.LimitReader(res.Body, msgmodels.MaxImageBytes+1))
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if len(data) > msgmodels.MaxImageBytes {
		return nil, ez.New(ez.EINVALID, "image "+name+" is larger than 5 MB", nil)
	}

	return data, nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP == nil {
		return http.DefaultClient
	}

	return c.HTTP
}

// responseError turns an error answer into an ez error with the code the
// status maps to.
func responseError(res *http.Response) error {
	failure := httpapi.ErrorResponse{}
	_ = json.NewDecoder(res.Body).Decode(&failure)
	if failure.Error == "" {
		failure.Error = res.Status
	}

	return ez.New(ez.HTTPStatusToError(res.StatusCode), failure.Error, nil)
}

// Ask posts body in channel as a question, which waits on whoever reads
// the channel until someone else replies to it.
func (c *Client) Ask(ctx context.Context, participantID, channel, body string) (*msgmodels.Message, error) {
	return c.post(ctx, httpapi.PostRequest{ParticipantID: participantID, Channel: channel, Body: body, Kind: msgmodels.KindQuestion})
}

// AskReply is Ask inside a thread: the question replies to replyTo.
func (c *Client) AskReply(ctx context.Context, participantID, channel, body string, replyTo *int64) (*msgmodels.Message, error) {
	return c.post(ctx, httpapi.PostRequest{ParticipantID: participantID, Channel: channel, Body: body, ReplyTo: replyTo, Kind: msgmodels.KindQuestion})
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
	return c.WaitChannels(ctx, []string{channel}, after, timeout)
}

// WaitChannels blocks until any of channels has a message after the
// cursor, and returns those of all of them in id order (ADR 0016).
func (c *Client) WaitChannels(ctx context.Context, channels []string, after int64, timeout time.Duration) (*httpapi.MessagesResponse, error) {
	query := url.Values{}
	query.Set("channels", strings.Join(channels, ","))
	query.Set("after", strconv.FormatInt(after, 10))
	query.Set("timeout", timeout.String())

	res := &httpapi.MessagesResponse{}

	err := c.do(ctx, http.MethodGet, "/wait", query, nil, res)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return res, nil
}

// Channels lists the channels the caller may read; archived adds the
// archived ones (ADR 0005 "Archived channels").
func (c *Client) Channels(ctx context.Context, archived bool) (*httpapi.ChannelsResponse, error) {
	var query url.Values
	if archived {
		query = url.Values{"archived": {"1"}}
	}

	res := &httpapi.ChannelsResponse{}

	err := c.do(ctx, http.MethodGet, "/channels", query, nil, res)
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

// Deliver posts the task's delivery with summary and report, the report
// JSON as is (nil for none), and files, the PNGs of its shots under their
// labels, on a sandbox's socket (agc task deliver, ADR 0010 item 2,
// "Delivery report") and decodes the task's view into out.
//
// Over vsock a body larger than 256 KiB never gets through, so a sandbox
// sends its shots first with SendShot and calls Deliver with no files.
func (c *Client) Deliver(ctx context.Context, summary string, report json.RawMessage, files map[string][]byte, out any) error {
	err := c.doWatched(ctx, http.MethodPost, httpapi.DeliverPath, nil, httpapi.DeliverRequest{Summary: summary, Report: report, Files: files}, out, true)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// SendShot sends the PNG of the shot called label in pieces of at most
// httpapi.MaxShotPieceBytes, for the delivery that follows to take (ADR
// 0010, "Delivery report").
func (c *Client) SendShot(ctx context.Context, label string, data []byte) error {
	path := strings.Replace(httpapi.ShotPiecePath, "{label}", url.PathEscape(label), 1)

	offset := 0
	for {
		end := min(offset+httpapi.MaxShotPieceBytes, len(data))

		call := request{
			method:      http.MethodPost,
			path:        path + "?offset=" + strconv.Itoa(offset),
			body:        data[offset:end],
			contentType: "image/png",
			step:        fmt.Sprintf("sending shot %s (%s, piece at %s) to %s", label, Size(len(data)), Size(offset), c.where()),
			watchAnswer: true,
		}

		res := &httpapi.ShotPieceResponse{}

		err := c.send(ctx, call, res)
		if err != nil {
			return ez.Wrap(err)
		}

		if res.Size != end {
			return ez.New(ez.EINTERNAL, fmt.Sprintf("the socket has %d bytes of shot %s, %d were sent", res.Size, label, end), nil)
		}

		offset = end
		if offset >= len(data) {
			return nil
		}
	}
}

// AgentView is the agent row as GET /agent returns it; only the fields
// the loop reads.
type AgentView struct {
	Name  string `json:"name"`
	Role  string `json:"role"`
	Owner string `json:"owner"`
}

// AgentResponse is GET /agent on a home sandbox's socket: the agent, the
// channels it reads right now, the channels outside those where it
// started a thread (only the replies in its threads there are its turns)
// and how long it may sit idle ("15m").
type AgentResponse struct {
	Agent    AgentView `json:"agent"`
	Channels []string  `json:"channels"`
	Asked    []string  `json:"asked"`
	Idle     string    `json:"idle"`
}

// Agent reads the agent whose home this is, on the sandbox's socket.
func (c *Client) Agent(ctx context.Context) (*AgentResponse, error) {
	res := &AgentResponse{}

	err := c.do(ctx, http.MethodGet, httpapi.AgentPath, nil, nil, res)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return res, nil
}

// Sleep asks agc to stop the home sandbox this runs in.
func (c *Client) Sleep(ctx context.Context) error {
	err := c.do(ctx, http.MethodPost, httpapi.AgentSleepPath, nil, nil, nil)
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
	err := c.do(ctx, http.MethodPost, taskPath(httpapi.TaskAcceptPath, name), nil, struct{}{}, out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// GetTask reads, from an agent's home, a task the agent manages.
func (c *Client) GetTask(ctx context.Context, name string, out any) error {
	err := c.do(ctx, http.MethodGet, taskPath(httpapi.TaskPath, name), nil, nil, out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// RejectTask sends back, from an agent's home, a task the agent manages.
func (c *Client) RejectTask(ctx context.Context, name string, why string, out any) error {
	err := c.do(ctx, http.MethodPost, taskPath(httpapi.TaskRejectPath, name), nil, httpapi.RejectBody{Why: why}, out)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// taskPath fills {name} in a task route pattern.
func taskPath(pattern string, name string) string {
	return strings.Replace(pattern, "{name}", url.PathEscape(name), 1)
}

// do sends one request and decodes the JSON answer into out. Error
// answers come back as ez errors with the code the status maps to.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	return c.doWatched(ctx, method, path, query, in, out, false)
}

// doWatched is do; watchAnswer also fails the request when its answer
// does not come within the stall time once the body is sent.
func (c *Client) doWatched(ctx context.Context, method, path string, query url.Values, in, out any, watchAnswer bool) error {
	target := path
	if query != nil {
		target += "?" + query.Encode()
	}

	call := request{method: method, path: target, watchAnswer: watchAnswer}

	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return ez.Wrap(err)
		}

		call.body = encoded
		call.contentType = "application/json"
	}

	return c.send(ctx, call, out)
}

// request is one call to the messaging API: path holds the query, body
// is sent as contentType when it is not nil.
type request struct {
	method      string
	path        string
	body        []byte
	contentType string
	// step says what the request does, for the error when it stalls;
	// empty, "sending <size> to <where>".
	step        string
	watchAnswer bool
}

// DefaultStall is how long a request may make no progress, sending its
// body or, when watched, waiting for its answer, before it fails.
const DefaultStall = 60 * time.Second

func (c *Client) stall() time.Duration {
	if c.Stall <= 0 {
		return DefaultStall
	}

	return c.Stall
}

// where names the server in errors: the sandbox socket over vsock, else
// its URL.
func (c *Client) where() string {
	if strings.HasPrefix(c.BaseURL, vsockBaseURL) {
		return "the sandbox socket"
	}

	return c.BaseURL
}

// vsockBaseURL is the BaseURL of a client that dials vsock; its host is
// ignored.
const vsockBaseURL = "http://vsock"

func (c *Client) send(ctx context.Context, call request, out any) error {
	target := strings.TrimSuffix(c.BaseURL, "/") + call.path

	watched, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)

	body := &progress{data: call.body, last: time.Now()}

	var reader io.Reader
	if call.body != nil {
		reader = body
	}

	req, err := http.NewRequestWithContext(watched, call.method, target, reader)
	if err != nil {
		return ez.Wrap(err)
	}

	if call.body != nil {
		req.ContentLength = int64(len(call.body))
	}

	if call.contentType != "" {
		req.Header.Set("Content-Type", call.contentType)
	}

	done := make(chan struct{})
	go c.watch(watched, cancel, body, call, done)

	res, err := c.httpClient().Do(req)
	close(done)
	if err != nil {
		var stalled *stallError
		if errors.As(context.Cause(watched), &stalled) {
			return ez.New(ez.EUNAVAILABLE, stalled.what, err)
		}

		return ez.New(ez.EUNAVAILABLE, "agc messaging is unreachable at "+c.BaseURL, err)
	}
	defer res.Body.Close()

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return responseError(res)
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

// stallError is the cause of a request the watchdog cancelled: what
// says which step made no progress.
type stallError struct {
	what string
}

func (e *stallError) Error() string {
	return e.what
}

// watch cancels the request when its body stops being read for the
// stall time, or, when call.watchAnswer, when the answer does not come
// for the stall time after the body is all read. done closes once the
// answer's headers are in.
func (c *Client) watch(ctx context.Context, cancel context.CancelCauseFunc, body *progress, call request, done <-chan struct{}) {
	stall := c.stall()

	tick := time.NewTicker(max(stall/20, 10*time.Millisecond))
	defer tick.Stop()

	for {
		select {
		case <-done:
			return
		case <-ctx.Done():
			return
		case <-tick.C:
		}

		sent, last := body.state()

		sending := sent < len(call.body)
		if !sending && !call.watchAnswer {
			continue
		}

		if time.Since(last) < stall {
			continue
		}

		step := call.step
		if step == "" {
			step = "sending " + Size(len(call.body)) + " to " + c.where()
		}

		what := fmt.Sprintf("%s: no progress for %s, %s of %s sent (%s %s)", step, stall, Size(sent), Size(len(call.body)), call.method, call.path)
		if !sending {
			what = fmt.Sprintf("%s: no answer for %s after the whole body was sent (%s %s)", step, stall, call.method, call.path)
		}

		cancel(&stallError{what: what})

		return
	}
}

// progress is a request body that records how much of it the transport
// has read, and when it last read.
type progress struct {
	mu   sync.Mutex
	data []byte
	sent int
	last time.Time
}

func (p *progress) Read(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.sent >= len(p.data) {
		return 0, io.EOF
	}

	n := copy(b, p.data[p.sent:])
	p.sent += n
	p.last = time.Now()

	return n, nil
}

func (p *progress) state() (int, time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.sent, p.last
}

// Size writes a byte count for people: "1.6 MB", "192.0 KB", "512 B".
func Size(n int) string {
	switch {
	case n >= 1000*1000:
		return fmt.Sprintf("%.1f MB", float64(n)/1e6)
	case n >= 1000:
		return fmt.Sprintf("%.1f KB", float64(n)/1e3)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
