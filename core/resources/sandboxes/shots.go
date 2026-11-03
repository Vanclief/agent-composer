package sandboxes

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging/httpapi"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// A sandbox cannot send a body larger than 256 KiB: microsandbox's vsock
// forwarder gives the guest no more credit until the host answers. So
// agc task deliver sends each shot first, in pieces, to
// httpapi.ShotPiecePath, and the delivery takes the shots its report
// names from those (ADR 0005 item 2, ADR 0010 "Delivery
// report"). The pieces live with the socket's listener, in memory, until
// a delivery succeeds.

// shotPieces holds the shots a sandbox has sent so far, by label.
type shotPieces struct {
	mu    sync.Mutex
	shots map[string][]byte
}

func newShotPieces() *shotPieces {
	return &shotPieces{shots: map[string][]byte{}}
}

// add appends data to the shot called label at offset, which must be
// where the shot ends; offset 0 starts it over. It returns the shot's
// size.
func (p *shotPieces) add(label string, offset int, data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	current, ok := p.shots[label]
	if offset == 0 {
		current = nil
	}

	if !ok && len(p.shots) >= taskmodels.MaxShots {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("at most %d shots: deliver, or send fewer", taskmodels.MaxShots), nil)
	}

	if offset != len(current) {
		return 0, ez.New(ez.ECONFLICT, fmt.Sprintf("shot %s has %d bytes, the piece starts at %d: send it again from offset 0", label, len(current), offset), nil)
	}

	if len(current)+len(data) > taskmodels.MaxShotBytes {
		return 0, ez.New(ez.EINVALID, fmt.Sprintf("shot %s is larger than %d bytes (2 MB): take a smaller screenshot", label, taskmodels.MaxShotBytes), nil)
	}

	p.shots[label] = append(current, data...)

	return len(p.shots[label]), nil
}

// take returns the sent shots named by the report's shots, for files to
// fill in where it has none under that label.
func (p *shotPieces) take(report json.RawMessage, files map[string][]byte) map[string][]byte {
	var shaped struct {
		Shots []taskmodels.Shot `json:"shots"`
	}

	if len(report) == 0 {
		return files
	}

	err := json.Unmarshal(report, &shaped)
	if err != nil {
		return files
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	for _, shot := range shaped.Shots {
		data, ok := p.shots[shot.Label]
		if !ok {
			continue
		}

		_, sent := files[shot.Label]
		if sent {
			continue
		}

		if files == nil {
			files = map[string][]byte{}
		}

		files[shot.Label] = data
	}

	return files
}

// clear forgets every shot sent, once a delivery took them.
func (p *shotPieces) clear() {
	p.mu.Lock()
	p.shots = map[string][]byte{}
	p.mu.Unlock()
}

func (api *API) serveShotPiece(sandboxName string, pieces *shotPieces) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		label := r.PathValue("label")
		if label == "" {
			httpapi.WriteError(w, ez.New(ez.EINVALID, "a shot needs a label", nil))
			return
		}

		offset, err := strconv.Atoi(r.URL.Query().Get("offset"))
		if err != nil || offset < 0 {
			httpapi.WriteError(w, ez.New(ez.EINVALID, "offset must be a byte offset, 0 or more", nil))
			return
		}

		data, err := readBodyWatched(w, r, sandboxName, httpapi.MaxShotPieceBytes)
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		size, err := pieces.add(label, offset, data)
		if err != nil {
			httpapi.WriteError(w, err)
			return
		}

		httpapi.WriteJSON(w, http.StatusOK, httpapi.ShotPieceResponse{Label: label, Size: size})
	}
}

// slowBodyRead is how long reading a request body on a sandbox socket
// may take before it is logged.
var slowBodyRead = 10 * time.Second

// logSlowBody logs a body read on a sandbox socket that is taking, or
// took, longer than slowBodyRead; done says whether it has ended.
var logSlowBody = func(sandboxName string, path string, read int64, length int64, elapsed time.Duration, done bool) {
	message := "sandbox socket: request body still being read"
	if done {
		message = "sandbox socket: slow request body read"
	}

	log.Warn().
		Str("sandbox", sandboxName).
		Str("path", path).
		Int64("bytes_read", read).
		Int64("content_length", length).
		Dur("elapsed", elapsed).
		Msg(message)
}

// watchBody replaces r's body with one that logs, through logSlowBody,
// when reading it takes longer than slowBodyRead: once while it is still
// being read and once more when it ends. Call the returned func once the
// body is read.
func watchBody(r *http.Request, sandboxName string) func() {
	body := &countingBody{ReadCloser: r.Body}
	r.Body = body

	start := time.Now()
	path := r.URL.Path

	timer := time.AfterFunc(slowBodyRead, func() {
		logSlowBody(sandboxName, path, body.count(), r.ContentLength, time.Since(start), false)
	})

	return func() {
		timer.Stop()

		elapsed := time.Since(start)
		if elapsed > slowBodyRead {
			logSlowBody(sandboxName, path, body.count(), r.ContentLength, elapsed, true)
		}
	}
}

// readBodyWatched reads r's whole body, at most limit bytes, logging a
// slow read.
func readBodyWatched(w http.ResponseWriter, r *http.Request, sandboxName string, limit int64) ([]byte, error) {
	done := watchBody(r, sandboxName)
	defer done()

	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	if err != nil {
		return nil, ez.New(ez.EINVALID, fmt.Sprintf("cannot read the body (at most %d bytes): %s", limit, err.Error()), err)
	}

	return data, nil
}

type countingBody struct {
	io.ReadCloser
	mu   sync.Mutex
	read int64
}

func (b *countingBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)

	b.mu.Lock()
	b.read += int64(n)
	b.mu.Unlock()

	return n, err
}

func (b *countingBody) count() int64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	return b.read
}
