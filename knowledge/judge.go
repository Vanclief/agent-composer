package knowledge

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/vanclief/ez"
	"gopkg.in/yaml.v3"
)

// Judged rules: a rule no command can check names who is trusted to judge
// it by reading, harness agents or people, and agc can ask a trusted agent
// for a verdict on a diff or a file. An LLM is a great parser and a
// terrible sensor, so a verdict is an opinion: recorded, it is a belief by
// the agent with no evidence under it, never evidence.

// Judge says who is trusted to judge a rule and what to ask them. Ask
// defaults to the rule text.
type Judge struct {
	Trusted []string `yaml:"trusted" json:"trusted"`
	Ask     string   `yaml:"ask" json:"ask,omitempty"`
}

// harnessIDs are the harness ids in runtime/harnesses/catalog.go, the
// agents agc can run as judges; keep them in step with the catalog.
var harnessIDs = []string{"codex_cli", "claude_code", "pi"}

// IsHarness says whether name is a harness agc can run as a judge.
func IsHarness(name string) bool {
	return slices.Contains(harnessIDs, name)
}

// IsAgent says whether name is a model or an agent harness rather than a
// person, as the evidence validator decides it.
func IsAgent(name string) bool {
	return isModel(name)
}

// JudgeNote says who a rule needs: the trusted judges of a judged rule, a
// human for a manual one, and for a mechanical rule with a judge block who
// may judge it as well. A mechanical rule without one needs nobody.
func JudgeNote(rule Rule) string {
	switch rule.Kind() {
	case Judged:
		return "needs " + joinOr(rule.Judge.Trusted)
	case Manual:
		return "needs a human"
	}

	if rule.Judge != nil {
		return "may also be judged by " + joinOr(rule.Judge.Trusted)
	}

	return ""
}

func joinOr(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}

	return strings.Join(items[:len(items)-1], ", ") + " or " + items[len(items)-1]
}

// Judges lists every name trusted to judge a loaded rule, in load order.
func (s *Set) Judges() []string {
	names := []string{}
	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			if rule.Judge == nil {
				continue
			}
			for _, name := range rule.Judge.Trusted {
				if !slices.Contains(names, name) {
					names = append(names, name)
				}
			}
		}
	}

	return names
}

// Verdict is a judge's answer about one rule.
type Verdict string

const (
	// Pass means the judge thinks the rule held.
	Pass Verdict = "pass"
	// Fail means the judge thinks the rule did not hold.
	Fail Verdict = "fail"
	// Unclear means the answer had no verdict line agc could read, or the
	// judge failed to answer.
	Unclear Verdict = "unclear"
)

// Judgment is one rule judged by By, or skipped because By is not trusted
// for it. Reason is the judge's sentence; Answer its whole answer.
type Judgment struct {
	Rule    Rule    `json:"rule"`
	By      string  `json:"by"`
	Skipped bool    `json:"skipped,omitempty"`
	Verdict Verdict `json:"verdict,omitempty"`
	Reason  string  `json:"reason,omitempty"`
	Answer  string  `json:"answer,omitempty"`
}

// Subject is what rules are judged on: Name says it in a sentence ("the
// diff against HEAD~1"), Text is what the judge reads.
type Subject struct {
	Name string
	Text string
}

// Asker asks the agent judging about one rule and returns its answer.
type Asker func(ctx context.Context, rule Rule, prompt string) (string, error)

// Judge asks by about every loaded rule that trusts it, in order, and lists
// the rules with a judge block that do not trust it as skipped. by must be
// a harness id or a name in some trusted list; ask is nil when by is a
// person, whom agc cannot run. A judge that fails to answer gives an
// unclear verdict, so the other rules are still judged.
func (s *Set) Judge(ctx context.Context, by string, subject Subject, ask Asker) ([]Judgment, error) {
	if !IsHarness(by) && !slices.Contains(s.Judges(), by) {
		msg := "unknown judge " + by + ": not a harness id (" + strings.Join(harnessIDs, ", ") + ") nor named in any judge: trusted list"
		return nil, ez.New(ez.EINVALID, msg, nil)
	}

	if ask == nil {
		msg := by + " is a person, and agc only runs harness agents: read the rules with agc rules show and judge them by hand"
		return nil, ez.New(ez.EINVALID, msg, nil)
	}

	judgments := []Judgment{}
	for _, ruleset := range s.Rulesets {
		for _, rule := range ruleset.Rules {
			if rule.Judge == nil {
				continue
			}

			if !slices.Contains(rule.Judge.Trusted, by) {
				judgments = append(judgments, Judgment{Rule: rule, By: by, Skipped: true})
				continue
			}

			judgment := Judgment{Rule: rule, By: by}
			answer, err := ask(ctx, rule, Prompt(ruleset.Subject, rule, subject))
			if err != nil {
				judgment.Verdict = Unclear
				judgment.Reason = "the judge failed to answer: " + fold(failure(err))
				judgments = append(judgments, judgment)
				continue
			}

			judgment.Answer = strings.TrimSpace(answer)
			judgment.Verdict, judgment.Reason = ParseVerdict(answer)
			judgments = append(judgments, judgment)
		}
	}

	return judgments, nil
}

// failure says why a judge failed: an ez error's message, which carries
// what the harness reported, rather than its stack of operations.
func failure(err error) string {
	_, isEz := err.(*ez.Error)
	if isEz && ez.ErrorMessage(err) != "" {
		return ez.ErrorMessage(err)
	}

	return err.Error()
}

// Prompt asks a judge whether rule, from the rules of about, holds on
// subject, and for exactly one verdict line.
func Prompt(about string, rule Rule, subject Subject) string {
	question := fold(rule.Rule)
	if rule.Judge != nil && strings.TrimSpace(rule.Judge.Ask) != "" {
		question = fold(rule.Judge.Ask)
	}

	var b strings.Builder
	b.WriteString("Judge whether one rule of " + fold(about) + " holds on the subject below. Read only: change nothing.\n\n")
	b.WriteString("Rule: " + fold(rule.Rule) + "\n")
	b.WriteString("Question: " + question + "\n\n")
	b.WriteString("If the subject does not touch what the rule is about, the rule holds.\n\n")
	b.WriteString("The subject, " + subject.Name + ":\n")
	b.WriteString("<<<SUBJECT\n" + subject.Text + "\nSUBJECT>>>\n\n")
	b.WriteString("Answer with exactly one line and nothing else, either\n")
	b.WriteString("VERDICT: pass — <one sentence saying why the rule holds>\n")
	b.WriteString("or\n")
	b.WriteString("VERDICT: fail — <one sentence naming where the rule is broken>\n")

	return b.String()
}

var verdictRe = regexp.MustCompile(`(?i)^VERDICT:\s*(pass|fail)\s*(?:—|–|-|:)\s*(\S.*)$`)

// ParseVerdict reads the one VERDICT line of an answer, ignoring text
// around it. No verdict line, more than one, or one without pass or fail
// and a sentence is unclear.
func ParseVerdict(answer string) (Verdict, string) {
	found := []string{}
	for _, line := range strings.Split(answer, "\n") {
		line = strings.Trim(strings.ReplaceAll(strings.TrimSpace(line), "**", ""), "`_ ")
		if strings.HasPrefix(strings.ToUpper(line), "VERDICT:") {
			found = append(found, line)
		}
	}

	if len(found) != 1 {
		return Unclear, ""
	}

	match := verdictRe.FindStringSubmatch(found[0])
	if match == nil {
		return Unclear, ""
	}

	return Verdict(strings.ToLower(match[1])), strings.TrimSpace(match[2])
}

// subjectCap is the most a subject sends to a judge.
const subjectCap = 200 << 10

// ReadSubject reads what to judge. on is empty for the diff of this branch
// against origin/master, - for stdin, a path for a file or a directory (its
// listing, then its files' contents) or else a git ref to diff against.
// Text past 200 KB is cut.
func ReadSubject(ctx context.Context, on string, stdin io.Reader) (Subject, error) {
	subject, err := readSubject(ctx, on, stdin)
	if err != nil {
		return Subject{}, ez.Wrap(err)
	}

	if strings.TrimSpace(subject.Text) == "" {
		return Subject{}, ez.New(ez.EINVALID, "nothing to judge: "+subject.Name+" is empty", nil)
	}

	if len(subject.Text) > subjectCap {
		subject.Text = subject.Text[:subjectCap] + "\n[cut at 200 KB]"
	}

	return subject, nil
}

func readSubject(ctx context.Context, on string, stdin io.Reader) (Subject, error) {
	if on == "" {
		text, err := git(ctx, "diff", "origin/master...HEAD")
		if err != nil {
			return Subject{}, ez.Wrap(err)
		}
		return Subject{Name: "the diff origin/master...HEAD", Text: text}, nil
	}

	if on == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return Subject{}, ez.New(ez.EINVALID, "cannot read stdin", err)
		}
		return Subject{Name: "stdin", Text: string(data)}, nil
	}

	info, err := os.Stat(on)
	if err == nil && !info.IsDir() {
		data, err := os.ReadFile(on)
		if err != nil {
			return Subject{}, ez.New(ez.EINVALID, "cannot read "+on, err)
		}
		return Subject{Name: on, Text: string(data)}, nil
	}
	if err == nil {
		text, err := readDir(on)
		if err != nil {
			return Subject{}, ez.Wrap(err)
		}
		return Subject{Name: on, Text: text}, nil
	}

	_, err = git(ctx, "rev-parse", "--verify", "--quiet", on+"^{commit}")
	if err != nil {
		return Subject{}, ez.New(ez.EINVALID, "--on "+on+" is neither a path nor a git ref", nil)
	}

	text, err := git(ctx, "diff", on)
	if err != nil {
		return Subject{}, ez.Wrap(err)
	}

	return Subject{Name: "the diff against " + on, Text: text}, nil
}

// readDir lists the files under dir, then their contents, skipping .git,
// node_modules and binary files, until the cap.
func readDir(dir string) (string, error) {
	files := []string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && (entry.Name() == ".git" || entry.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return "", ez.New(ez.EINVALID, "cannot list "+dir, err)
	}

	var b strings.Builder
	b.WriteString("Files:\n" + strings.Join(files, "\n") + "\n")
	for _, path := range files {
		if b.Len() > subjectCap {
			break
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return "", ez.New(ez.EINVALID, "cannot read "+path, err)
		}
		if bytes.IndexByte(data, 0) >= 0 {
			continue
		}
		b.WriteString("\n--- " + path + " ---\n" + string(data))
	}

	return b.String(), nil
}

func git(ctx context.Context, args ...string) (string, error) {
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		msg := "git " + strings.Join(args, " ") + " failed"
		detail := strings.TrimSpace(stderr.String())
		if detail != "" {
			msg += ": " + detail
		}
		return "", ez.New(ez.EINVALID, msg, err)
	}

	return string(output), nil
}

// ShortHead is the short sha of HEAD, or empty outside a git repository.
func ShortHead(ctx context.Context) string {
	sha, err := git(ctx, "rev-parse", "--short", "HEAD")
	if err != nil {
		return ""
	}

	return strings.TrimSpace(sha)
}

// judgedBelief is the shape a recorded judgment is written in: a belief by
// the judge with no evidence, so always an opinion.
type judgedBelief struct {
	ID     string `yaml:"id"`
	Belief string `yaml:"belief"`
	By     string `yaml:"by"`
	When   string `yaml:"when"`
}

// Record appends the pass and fail verdicts in judgments to the beliefs
// file at path, creating it if missing, as opinions held by the judge:
// never evidence. Ids are judged-<rule id>-<suffix>, made unique against
// taken and the file. Unclear and skipped judgments are not recorded. It
// returns what it wrote.
func Record(path string, judgments []Judgment, subject Subject, suffix string, when string, taken map[string]bool) ([]Belief, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		data = []byte("# Verdicts of agents asked to judge rules (agc rules judge --record).\n# Each is an opinion held by the agent, never evidence (ADR 0006).\nbeliefs: []\n")
		err = nil
	}
	if err != nil {
		return nil, ez.New(ez.EINVALID, "cannot read "+path, err)
	}

	var doc yaml.Node
	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		return nil, ez.New(ez.EINVALID, path+": "+err.Error(), err)
	}

	list := beliefsList(&doc)
	if list == nil {
		return nil, ez.New(ez.EINVALID, path+": not a beliefs file", nil)
	}

	used := map[string]bool{}
	for id := range taken {
		used[id] = true
	}
	for _, item := range list.Content {
		var existing judgedBelief
		err = item.Decode(&existing)
		if err == nil {
			used[existing.ID] = true
		}
	}

	written := []Belief{}
	for _, judgment := range judgments {
		if judgment.Skipped || (judgment.Verdict != Pass && judgment.Verdict != Fail) {
			continue
		}

		held := "held"
		if judgment.Verdict == Fail {
			held = "did not hold"
		}

		record := judgedBelief{
			ID:     uniqueID(judgment.Rule.ID, suffix, used),
			Belief: fmt.Sprintf("%s judged that “%s” %s on %s: %s", judgment.By, trimPeriod(fold(judgment.Rule.Rule)), held, subject.Name, judgment.Reason),
			By:     judgment.By,
			When:   when,
		}

		var item yaml.Node
		err = item.Encode(record)
		if err != nil {
			return nil, ez.New(ez.EINTERNAL, "cannot encode "+record.ID, err)
		}
		// An empty beliefs: or beliefs: [] becomes a block list.
		list.Kind = yaml.SequenceNode
		list.Tag = "!!seq"
		list.Value = ""
		list.Style = 0
		list.Content = append(list.Content, &item)
		written = append(written, Belief{ID: record.ID, Belief: record.Belief, By: record.By, When: record.When, File: path})
	}

	if len(written) == 0 {
		return written, nil
	}

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	err = encoder.Encode(&doc)
	if err != nil {
		return nil, ez.New(ez.EINTERNAL, "cannot encode "+path, err)
	}

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "cannot create "+filepath.Dir(path), err)
	}

	err = os.WriteFile(path, out.Bytes(), 0o644)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "cannot write "+path, err)
	}

	return written, nil
}

// beliefsList finds the beliefs: sequence of a parsed file, or nil.
func beliefsList(doc *yaml.Node) *yaml.Node {
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}

	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := root.Content[i]
		value := root.Content[i+1]
		if key.Value != "beliefs" {
			continue
		}
		if value.Kind == yaml.SequenceNode || (value.Kind == yaml.ScalarNode && value.Tag == "!!null") {
			return value
		}
		return nil
	}

	return nil
}

// uniqueID is judged-<rule id>-<suffix>, shortened to fit an id and
// numbered if already used.
func uniqueID(ruleID string, suffix string, used map[string]bool) string {
	room := 63 - len("judged-") - len(suffix) - len("--99")
	if len(ruleID) > room {
		ruleID = strings.TrimRight(ruleID[:room], "-")
	}

	base := "judged-" + ruleID + "-" + suffix
	id := base
	for n := 2; used[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	used[id] = true

	return id
}
