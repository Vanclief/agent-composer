package agents

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/sandboxes"
	"github.com/vanclief/agent-composer/models/agent"
)

// sessionFile is one harness session file on a volume.
type sessionFile struct {
	path    string
	harness string
	parse   parseFunc
}

// parsedFile is what a parser found in one session file, turns in file
// order. A light parse (full false) fills only each turn's StartedAt and
// Session: enough to number every turn on the volume.
type parsedFile struct {
	session string
	turns   []Turn
	skipped int
}

type parseFunc func(r io.Reader, full bool) parsedFile

// harnessFiles maps the transcript directories of ADR 0008 to the harness
// that writes them and its parser.
var harnessFiles = []struct {
	prefix  string
	harness string
	parse   parseFunc
}{
	{".claude/projects/", string(agent.HarnessClaudeCode), parseClaude},
	{".codex/sessions/", string(agent.HarnessCodexCLI), parseCodex},
}

// turnRef places one turn of the volume: which file, which turn in it.
type turnRef struct {
	file    int
	index   int
	start   time.Time
	session string
}

// volumeScan is every turn on a volume, numbered, from a light parse of
// every session file.
type volumeScan struct {
	root    *os.Root
	files   []sessionFile
	light   []parsedFile
	refs    []turnRef
	labels  map[string]threadLabel
	skipped int
}

// scanVolume lists the session files on the volume at dir and numbers
// their turns: n is 1-based across all sessions, by start time.
//
// Every file is read on every request. A turn's n depends on how many
// turns started before it in every session (DM threads are concurrent
// sessions whose turns interleave), so file mtimes alone cannot place a
// page: a file last written before the page's oldest turn still shifts its
// numbers. Instead every file gets a light parse that decodes only each
// line's envelope to find where turns start, and only the files holding
// the requested page get the full parse (tool calls, results, answers,
// tokens). On a volume of a few hundred turns that keeps the expensive
// part bounded by the page, not the volume.
func scanVolume(dir string) (*volumeScan, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot open volume "+dir, err)
	}

	transcripts, err := sandboxes.VolumeTranscripts(root)
	if err != nil {
		root.Close()
		return nil, ez.New(ez.EINTERNAL, "cannot list session files on volume "+dir, err)
	}

	scan := &volumeScan{root: root, labels: readThreadLabels(root)}
	for _, transcript := range transcripts {
		for _, h := range harnessFiles {
			if strings.HasPrefix(transcript.Path, h.prefix) {
				scan.files = append(scan.files, sessionFile{path: transcript.Path, harness: h.harness, parse: h.parse})
				break
			}
		}
	}

	for i := range scan.files {
		parsed, err := scan.parse(i, false)
		if err != nil {
			root.Close()
			return nil, ez.Wrap(err)
		}

		scan.light = append(scan.light, parsed)
		scan.skipped += parsed.skipped
		for j, turn := range parsed.turns {
			scan.refs = append(scan.refs, turnRef{file: i, index: j, start: turn.StartedAt, session: turn.Session})
		}
	}

	sort.SliceStable(scan.refs, func(i, j int) bool {
		a := scan.refs[i]
		b := scan.refs[j]
		if !a.start.Equal(b.start) {
			return a.start.Before(b.start)
		}
		if a.session != b.session {
			return a.session < b.session
		}
		if a.file != b.file {
			return scan.files[a.file].path < scan.files[b.file].path
		}
		return a.index < b.index
	})

	return scan, nil
}

func (s *volumeScan) close() {
	s.root.Close()
}

// parse reads one session file. A file that went away since the listing
// reads as empty; a link is refused like the transcript routes refuse it.
func (s *volumeScan) parse(i int, full bool) (parsedFile, error) {
	file := s.files[i]

	info, err := s.root.Lstat(file.path)
	if errors.Is(err, fs.ErrNotExist) {
		return parsedFile{}, nil
	}
	if err != nil {
		return parsedFile{}, ez.New(ez.EINTERNAL, "cannot read session file "+file.path, err)
	}
	if !info.Mode().IsRegular() {
		return parsedFile{}, nil
	}

	f, err := s.root.Open(file.path)
	if errors.Is(err, fs.ErrNotExist) {
		return parsedFile{}, nil
	}
	if err != nil {
		return parsedFile{}, ez.New(ez.EINTERNAL, "cannot open session file "+file.path, err)
	}
	defer f.Close()

	parsed := file.parse(f, full)
	if parsed.session == "" {
		parsed.session = sessionFromPath(file.path)
	}

	for j := range parsed.turns {
		turn := &parsed.turns[j]
		turn.Harness = file.harness
		if turn.Session == "" {
			turn.Session = parsed.session
		}

		label, ok := s.labels[turn.Session]
		if ok {
			turn.Channel = label.channel
			turn.Thread = label.thread
		}
	}

	return parsed, nil
}

// turns returns the full turns the refs point at, numbered, reading each
// file they live in once.
func (s *volumeScan) turns(refs []turnRef, numbers []int) ([]Turn, error) {
	full := map[int]parsedFile{}
	out := make([]Turn, 0, len(refs))

	for k, ref := range refs {
		parsed, ok := full[ref.file]
		if !ok {
			var err error
			parsed, err = s.parse(ref.file, true)
			if err != nil {
				return nil, ez.Wrap(err)
			}
			full[ref.file] = parsed
		}

		// A file the harness appended to between the two reads only gains
		// turns at its end; one that shrank falls back to the light turn.
		var turn Turn
		if ref.index < len(parsed.turns) && parsed.turns[ref.index].StartedAt.Equal(ref.start) {
			turn = parsed.turns[ref.index]
		} else {
			turn = s.light[ref.file].turns[ref.index]
		}

		turn.N = numbers[k]
		if turn.ToolCalls == nil {
			turn.ToolCalls = []ToolCall{}
		}
		out = append(out, turn)
	}

	return out, nil
}

func listTurns(dir string, page TurnsPage) (*ListTurnsResponse, error) {
	scan, err := scanVolume(dir)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	defer scan.close()

	limit := page.Limit
	if limit == 0 {
		limit = DefaultTurnsLimit
	}

	end := len(scan.refs)
	if page.Before > 0 && page.Before-1 < end {
		end = page.Before - 1
	}

	refs := []turnRef{}
	numbers := []int{}
	for i := end - 1; i >= 0 && len(refs) < limit; i-- {
		refs = append(refs, scan.refs[i])
		numbers = append(numbers, i+1)
	}

	turns, err := scan.turns(refs, numbers)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	summaries := make([]TurnSummary, 0, len(turns))
	for _, turn := range turns {
		summaries = append(summaries, turn.Summary())
	}

	return &ListTurnsResponse{Turns: summaries, Skipped: scan.skipped}, nil
}

func getTurn(dir string, n int) (*Turn, error) {
	scan, err := scanVolume(dir)
	if err != nil {
		return nil, ez.Wrap(err)
	}
	defer scan.close()

	if n < 1 || n > len(scan.refs) {
		return nil, ez.New(ez.ENOTFOUND, "turn "+strconv.Itoa(n)+" not found: the volume has "+strconv.Itoa(len(scan.refs))+" turns", nil)
	}

	turns, err := scan.turns([]turnRef{scan.refs[n-1]}, []int{n})
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return &turns[0], nil
}

// threadLabel is where agc routed a harness session: the channel and the
// thread root of ADR 0016 item 7, or thread `main` for the main role's one
// session.
type threadLabel struct {
	channel string
	thread  string
}

// readThreadLabels reads SessionsFile, `{ "<channel>:<root id>": "<session
// id>", "main": "<session id>" }`, into session id → label. Missing or
// unreadable, there are no labels: turns still list, unlabeled.
func readThreadLabels(root *os.Root) map[string]threadLabel {
	labels := map[string]threadLabel{}

	info, err := root.Lstat(SessionsFile)
	if err != nil || !info.Mode().IsRegular() {
		return labels
	}

	body, err := root.ReadFile(SessionsFile)
	if err != nil {
		return labels
	}

	entries := map[string]json.RawMessage{}
	err = json.Unmarshal(body, &entries)
	if err != nil {
		return labels
	}

	for key, value := range entries {
		session := ""
		err = json.Unmarshal(value, &session)
		if err != nil || session == "" {
			continue
		}

		label := threadLabel{thread: key}
		at := strings.LastIndex(key, ":")
		if at >= 0 {
			label = threadLabel{channel: key[:at], thread: key[at+1:]}
		}
		labels[session] = label
	}

	return labels
}

// sessionFromPath is the session id a file name carries: Claude Code's
// <session>.jsonl, Codex's rollout-<time>-<session>.jsonl.
func sessionFromPath(p string) string {
	name := strings.TrimSuffix(p[strings.LastIndex(p, "/")+1:], ".jsonl")
	if !strings.HasPrefix(name, "rollout-") || len(name) < 36 {
		return name
	}

	return name[len(name)-36:]
}

// eachLine calls fn with every non-blank line of r, of any length, and
// counts the lines that are not a JSON object: those are skipped.
func eachLine(r io.Reader, fn func(line []byte)) int {
	skipped := 0
	reader := bufio.NewReaderSize(r, 64*1024)

	for {
		line, err := reader.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			if trimmed[0] == '{' && json.Valid(trimmed) {
				fn(trimmed)
			} else {
				skipped++
			}
		}
		if err != nil {
			return skipped
		}
	}
}

// parseTime reads an RFC 3339 timestamp, zero when there is none.
func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}

	return t.UTC()
}

// cutResult keeps the first MaxToolResult bytes of s, on a rune boundary.
func cutResult(s string) (string, bool) {
	if len(s) <= MaxToolResult {
		return s, false
	}

	end := MaxToolResult
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}

	return s[:end], true
}

// rawArgs is a tool call's arguments as JSON: the object itself, or a
// string of JSON (Codex) decoded, or else the text as a JSON string.
func rawArgs(raw json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return json.RawMessage("null")
	}

	if trimmed[0] != '"' {
		return json.RawMessage(trimmed)
	}

	text := ""
	err := json.Unmarshal(trimmed, &text)
	if err != nil {
		return json.RawMessage(trimmed)
	}

	inner := bytes.TrimSpace([]byte(text))
	if len(inner) > 0 && (inner[0] == '{' || inner[0] == '[') && json.Valid(inner) {
		return json.RawMessage(inner)
	}

	return json.RawMessage(trimmed)
}

// textOf flattens a tool result or message content: a string, a list of
// blocks with text, or an object with content or output.
func textOf(raw json.RawMessage) string {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return ""
	}

	switch trimmed[0] {
	case '"':
		text := ""
		_ = json.Unmarshal(trimmed, &text)
		return text
	case '[':
		blocks := []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}{}
		_ = json.Unmarshal(trimmed, &blocks)
		parts := []string{}
		for _, block := range blocks {
			if block.Text != "" {
				parts = append(parts, block.Text)
			} else if block.Type == "image" || block.Type == "input_image" {
				parts = append(parts, "[image]")
			}
		}
		return strings.Join(parts, "\n")
	case '{':
		object := struct {
			Content json.RawMessage `json:"content"`
			Output  json.RawMessage `json:"output"`
		}{}
		_ = json.Unmarshal(trimmed, &object)
		if len(object.Content) > 0 {
			return textOf(object.Content)
		}
		if len(object.Output) > 0 {
			return textOf(object.Output)
		}
		return string(trimmed)
	}

	return string(trimmed)
}
