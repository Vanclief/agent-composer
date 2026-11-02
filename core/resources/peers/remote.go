package peers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/executor"
	"github.com/vanclief/agent-composer/executor/sandbox"
	"github.com/vanclief/agent-composer/secrets"
	"github.com/vanclief/agent-composer/task"
)

// statusTimeout bounds one status read, so a list does not hang on a peer
// that is down.
const statusTimeout = 10 * time.Second

// Remote is the Runtime of one peer: every call is a request to the peer's
// /api/peer routes with this server's AGC_PEER_TOKEN (ADR 0018). It keeps
// the sandboxes.Runtime interface, so the sandboxes API, the watchdog and
// recovery use a peer's VMs exactly as local ones.
type Remote struct {
	// Name is what this server calls the peer, from servers:.
	Name string
	// URL is the peer's root, such as http://192.168.1.20:1202.
	URL         string
	secretsPath string
	http        *http.Client
}

var (
	_ sandboxes.Runtime       = (*Remote)(nil)
	_ sandboxes.SecretChecker = (*Remote)(nil)
)

// NewRemote returns the runtime of the peer called name at baseURL. The
// token is read from the secrets file at secretsPath on every request.
func NewRemote(name string, baseURL string, secretsPath string) *Remote {
	return &Remote{Name: name, URL: baseURL, secretsPath: secretsPath, http: &http.Client{}}
}

// errorBody is the error shape of the REST API and of the sockets.
type errorBody struct {
	Error json.RawMessage `json:"error"`
}

type standardError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// call sends one request to the peer and decodes the JSON answer into
// out. The peer's error keeps its code and message, which always names
// the server.
func (r *Remote) call(ctx context.Context, method string, path string, in any, out any) error {
	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return ez.Wrap(err)
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, r.URL+path, body)
	if err != nil {
		return ez.New(ez.EINVALID, "bad URL for server "+r.Name+": "+r.URL, err)
	}
	request.Header.Set("Accept", "application/json")
	if in != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	err = r.authorize(request.Header)
	if err != nil {
		return ez.Wrap(err)
	}

	response, err := r.http.Do(request)
	if err != nil {
		return ez.New(ez.EUNAVAILABLE, "cannot reach server "+r.Name+" at "+r.URL+": "+err.Error(), err)
	}
	defer response.Body.Close() // nolint:errcheck // Close errors are not actionable here.

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return ez.New(ez.EUNAVAILABLE, "cannot read the answer from server "+r.Name, err)
	}

	if response.StatusCode < 200 || response.StatusCode > 299 {
		return r.remoteError(response, data)
	}

	if out == nil {
		return nil
	}

	err = json.Unmarshal(data, out)
	if err != nil {
		return ez.New(ez.EINTERNAL, "server "+r.Name+" answered "+method+" "+path+" with something other than JSON", err)
	}

	return nil
}

// authorize adds the peer token. Without one this server cannot call any
// peer.
func (r *Remote) authorize(header http.Header) error {
	token, err := loadToken(r.secretsPath)
	if err != nil {
		return ez.Wrap(err)
	}

	if token == "" {
		return ez.New(ez.EUNAVAILABLE, "cannot call server "+r.Name+": "+secrets.PeerToken+" is not set in "+r.secretsPath, nil)
	}

	header.Set("Authorization", TokenScheme+token)

	return nil
}

func (r *Remote) remoteError(response *http.Response, data []byte) error {
	code := ez.HTTPStatusToError(response.StatusCode)
	message := response.Status

	failure := errorBody{}
	err := json.Unmarshal(data, &failure)
	if err == nil && len(failure.Error) > 0 {
		standard := standardError{}
		plain := ""

		if json.Unmarshal(failure.Error, &standard) == nil && standard.Message != "" {
			message = standard.Message
			if standard.Code != "" {
				code = standard.Code
			}
		} else if json.Unmarshal(failure.Error, &plain) == nil && plain != "" {
			message = plain
		}
	}

	// A peer's own message may already name it, as a missing secret's
	// does.
	if !strings.Contains(message, "server "+r.Name) {
		message = "server " + r.Name + ": " + message
	}

	return ez.New(code, message, nil)
}

func (r *Remote) vmPath(name string) string {
	return "/api/peer/vms/" + url.PathEscape(name)
}

// Create asks the peer to create and boot the VM. Secret values are never
// sent: the peer fills them from its own secrets file. The VM is labelled
// with the peer's name, so the peer finds it again after a restart.
func (r *Remote) Create(ctx context.Context, spec sandbox.Spec) (sandboxes.Sandbox, error) {
	wire := toVMSpec(spec)

	labels := map[string]string{}
	for k, v := range wire.Labels {
		labels[k] = v
	}
	labels[task.LabelServer] = r.Name
	wire.Labels = labels

	err := r.call(ctx, http.MethodPost, "/api/peer/vms", &CreateVMRequest{Server: r.Name, Spec: wire}, &VMResponse{})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &remoteSandbox{remote: r, name: spec.Name}, nil
}

// Connect returns a handle on a running VM. Like the local runtime, it
// does not start a stopped one.
func (r *Remote) Connect(ctx context.Context, name string) (sandboxes.Sandbox, error) {
	status, err := r.Status(ctx, name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if status != sandboxes.StatusRunning {
		return nil, ez.New(ez.EUNAVAILABLE, "sandbox "+name+" on server "+r.Name+" is "+status+", start it first", nil)
	}

	return &remoteSandbox{remote: r, name: name}, nil
}

func (r *Remote) Start(ctx context.Context, name string, vmSecrets []sandbox.Secret) error {
	return r.call(ctx, http.MethodPost, r.vmPath(name)+"/start", &StartVMRequest{Server: r.Name, Secrets: toSecrets(vmSecrets)}, &VMResponse{})
}

func (r *Remote) Stop(ctx context.Context, name string) error {
	return r.call(ctx, http.MethodPost, r.vmPath(name)+"/stop", nil, &VMResponse{})
}

func (r *Remote) Remove(ctx context.Context, name string) error {
	return r.call(ctx, http.MethodDelete, r.vmPath(name), nil, &VMResponse{})
}

// Status is the peer's msb's word, verbatim; a VM the peer does not know
// is ENOTFOUND, as from the local runtime.
func (r *Remote) Status(ctx context.Context, name string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, statusTimeout)
	defer cancel()

	out := &VMResponse{}

	err := r.call(ctx, http.MethodGet, r.vmPath(name), nil, out)
	if err != nil {
		return "", ez.Wrap(err)
	}

	return out.Status, nil
}

// CheckSecrets asks the peer whether its secrets file has every secret;
// the error names the missing ones and the server.
func (r *Remote) CheckSecrets(ctx context.Context, bindings []secrets.Binding) error {
	wire := []Secret{}
	for _, binding := range bindings {
		wire = append(wire, Secret{Name: binding.Name, Hosts: binding.Hosts})
	}

	return r.call(ctx, http.MethodPost, "/api/peer/secrets/check", &CheckSecretsRequest{Server: r.Name, Secrets: wire}, &CheckSecretsResponse{})
}

// Host reads the peer's report of its host.
func (r *Remote) Host(ctx context.Context) (*HostReport, error) {
	out := &HostReport{}

	err := r.call(ctx, http.MethodGet, "/api/peer/host", nil, out)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return out, nil
}

// remoteSandbox is a handle on a VM on a peer. It holds nothing open, so
// Detach has nothing to release.
type remoteSandbox struct {
	remote *Remote
	name   string
}

var _ sandboxes.Sandbox = (*remoteSandbox)(nil)

func (s *remoteSandbox) exec(ctx context.Context, in *ExecRequest) (*ExecResponse, error) {
	out := &ExecResponse{}

	err := s.remote.call(ctx, http.MethodPost, s.remote.vmPath(s.name)+"/exec", in, out)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return out, nil
}

func (s *remoteSandbox) Detach(ctx context.Context) error {
	return nil
}

func (s *remoteSandbox) Run(ctx context.Context, cmd executor.Command) (*executor.Result, error) {
	out, err := s.exec(ctx, &ExecRequest{Op: OpRun, Command: toCommand(cmd)})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if out.Result == nil {
		return nil, ez.New(ez.EINTERNAL, "server "+s.remote.Name+" ran the command and sent no result", nil)
	}

	return &executor.Result{Stdout: out.Result.Stdout, Stderr: out.Result.Stderr, ExitCode: out.Result.ExitCode}, nil
}

func (s *remoteSandbox) LookPath(ctx context.Context, name string) error {
	_, err := s.exec(ctx, &ExecRequest{Op: OpLookPath, Path: name})
	return err
}

func (s *remoteSandbox) WriteFile(ctx context.Context, path string, data []byte) error {
	_, err := s.exec(ctx, &ExecRequest{Op: OpWriteFile, Path: path, Data: data})
	return err
}

func (s *remoteSandbox) ReadFile(ctx context.Context, path string) ([]byte, error) {
	out, err := s.exec(ctx, &ExecRequest{Op: OpReadFile, Path: path})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return out.Data, nil
}

func (s *remoteSandbox) RemoveFile(ctx context.Context, path string) error {
	_, err := s.exec(ctx, &ExecRequest{Op: OpRemoveFile, Path: path})
	return err
}

// TempDir is the guest's /tmp, as for a local VM.
func (s *remoteSandbox) TempDir() string {
	return "/tmp"
}

func (s *remoteSandbox) StartTerminal(ctx context.Context, cmd executor.Command) (*executor.Terminal, error) {
	out, err := s.exec(ctx, &ExecRequest{Op: OpStartTerminal, Command: toCommand(cmd)})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if out.Terminal == nil {
		return nil, ez.New(ez.EINTERNAL, "server "+s.remote.Name+" started the terminal and sent no id", nil)
	}

	terminal := out.Terminal.executor()

	return &terminal, nil
}

func (s *remoteSandbox) ListTerminals(ctx context.Context) ([]executor.Terminal, error) {
	out, err := s.exec(ctx, &ExecRequest{Op: OpListTerminals})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	terminals := []executor.Terminal{}
	for _, terminal := range out.Terminals {
		terminals = append(terminals, terminal.executor())
	}

	return terminals, nil
}

// AttachTerminal opens the peer's terminal WebSocket, which speaks the
// same frames as the panel's: binary bytes both ways, a text resize
// message, normal closure when the job exits.
func (s *remoteSandbox) AttachTerminal(ctx context.Context, id string, replayBytes int) (executor.TerminalStream, error) {
	header := http.Header{}

	err := s.remote.authorize(header)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	target := s.remote.URL + s.remote.vmPath(s.name) + "/terminals/" + url.PathEscape(id) + "/ws?replay=" + strconv.Itoa(replayBytes)

	conn, response, err := websocket.Dial(ctx, target, &websocket.DialOptions{HTTPClient: s.remote.http, HTTPHeader: header})
	if err != nil {
		if response != nil && response.StatusCode >= 400 {
			data, _ := io.ReadAll(io.LimitReader(response.Body, 1024))
			return nil, s.remote.remoteError(response, data)
		}

		return nil, ez.New(ez.EUNAVAILABLE, "cannot attach to terminal "+id+" of sandbox "+s.name+" on server "+s.remote.Name+": "+err.Error(), err)
	}
	conn.SetReadLimit(1 << 20)

	return &remoteStream{conn: conn}, nil
}

// remoteStream is a terminal attachment over the peer's WebSocket.
type remoteStream struct {
	conn *websocket.Conn
}

func (s *remoteStream) Recv(ctx context.Context) ([]byte, error) {
	for {
		typ, data, err := s.conn.Read(ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure {
				return nil, io.EOF
			}

			var closeErr websocket.CloseError
			if errors.As(err, &closeErr) {
				return nil, ez.New(ez.EINTERNAL, closeErr.Reason, err)
			}

			return nil, ez.Wrap(err)
		}

		if typ == websocket.MessageBinary {
			return data, nil
		}
	}
}

func (s *remoteStream) Write(ctx context.Context, data []byte) error {
	return s.conn.Write(ctx, websocket.MessageBinary, data)
}

func (s *remoteStream) Resize(ctx context.Context, rows uint16, cols uint16) error {
	data, err := json.Marshal(ResizeMessage{Type: "resize", Rows: rows, Cols: cols})
	if err != nil {
		return ez.Wrap(err)
	}

	return s.conn.Write(ctx, websocket.MessageText, data)
}

func (s *remoteStream) Close() error {
	return s.conn.Close(websocket.StatusNormalClosure, "detached")
}
