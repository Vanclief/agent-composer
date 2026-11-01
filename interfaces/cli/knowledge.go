package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/executor/host"
	"github.com/vanclief/agent-composer/knowledge"
	"github.com/vanclief/agent-composer/models/agent"
	"github.com/vanclief/agent-composer/runtime/harnesses"
)

// The evidence, rules, beliefs, decisions and actions commands
// (docs/decisions/0006-rules-and-beliefs.md). Each item prints as a
// sentence, with the ids behind it in a dim trailer; --ids prints the
// compact id form for scripts and agents. Nothing is written except by
// rules judge --record, which appends opinions, and whether a belief is
// grounded, an opinion, contested or in doubt is computed as it is
// printed.

var beliefsFlag = &cli.StringSliceFlag{
	Name:  "beliefs",
	Usage: "Evidence, beliefs and decisions the rules rest on (default evidence/ beliefs/ decisions/ when they exist)",
}

var idsFlag = &cli.BoolFlag{
	Name:  "ids",
	Usage: "Print the compact id form, a stable format for scripts and agents, instead of sentences",
}

var withoutFlag = &cli.StringSliceFlag{
	Name:  "without",
	Usage: "Judge as if this evidence or belief were retracted or absent (repeatable); nothing is written",
}

func rulesCommand() *cli.Command {
	return &cli.Command{
		Name:  "rules",
		Usage: "Rules: what must or must not be done (ADR 0006)",
		Subcommands: []*cli.Command{
			{
				Name:      "check",
				Usage:     "Run the rules' checks and print a checklist; fails if a check fails",
				ArgsUsage: "[path...] [--beliefs <path>] [--ids]",
				Flags:     []cli.Flag{beliefsFlag, idsFlag},
				Action:    rulesCheck,
			},
			{
				Name:      "show",
				Usage:     "Print the rules as a markdown checklist without running checks",
				ArgsUsage: "[path...] [--beliefs <path>] [--without <id>] [--ids]",
				Flags:     []cli.Flag{beliefsFlag, withoutFlag, idsFlag},
				Action:    rulesShow,
			},
			{
				Name:      "judge",
				Usage:     "Ask a trusted agent whether the judged rules hold on a diff or a file; fails on a fail or an unclear answer",
				ArgsUsage: "--as <agent> [--on <path|git ref|->] [--model <model>] [--record] [path...] [--beliefs <path>]",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "as", Usage: "The judge: a harness id (claude_code, codex_cli, pi) trusted by the rules"},
					&cli.StringFlag{Name: "on", Usage: "What to judge: a file or directory, a git ref to diff against, or - for stdin (default: git diff origin/master...HEAD)"},
					&cli.StringFlag{Name: "model", Usage: "The judge's model (default: the first in the harness catalog)"},
					&cli.BoolFlag{Name: "record", Usage: "Append the verdicts to beliefs/judgments.yaml as opinions by the judge, never evidence"},
					beliefsFlag,
				},
				Action: rulesJudge,
			},
		},
	}
}

func beliefsCommand() *cli.Command {
	return &cli.Command{
		Name:  "beliefs",
		Usage: "Beliefs: what we hold true, and why (ADR 0006)",
		Subcommands: []*cli.Command{
			{
				Name:      "show",
				Usage:     "Print every belief as grounded, an opinion, contested or in doubt, computed from its evidence",
				ArgsUsage: "[path...] [--without <id>] [--ids]",
				Flags:     []cli.Flag{withoutFlag, idsFlag},
				Action:    beliefsShow,
			},
			{
				Name:      "why",
				Usage:     "Follow a belief down to the evidence it rests on",
				ArgsUsage: "<id> [path...] [--ids]",
				Flags:     []cli.Flag{idsFlag},
				Action:    beliefsWhy,
			},
			{
				Name:      "doubt",
				Usage:     "List the beliefs, decisions, rules and open actions that fall if a belief or a piece of evidence is wrong",
				ArgsUsage: "<id> [path...] [--ids]",
				Flags:     []cli.Flag{idsFlag},
				Action:    beliefsDoubt,
			},
			{
				Name:      "conflicts",
				Usage:     "List every contested contradiction with the evidence and opinions under each side",
				ArgsUsage: "[path...] [--ids]",
				Flags:     []cli.Flag{idsFlag},
				Action:    beliefsConflicts,
			},
		},
	}
}

func evidenceCommand() *cli.Command {
	return &cli.Command{
		Name:  "evidence",
		Usage: "Evidence: what was seen, by whom and when (ADR 0006)",
		Subcommands: []*cli.Command{
			{
				Name:      "show",
				Usage:     "Print the evidence and how many beliefs cite each piece",
				ArgsUsage: "[path...] [--ids]",
				Flags:     []cli.Flag{idsFlag},
				Action:    evidenceShow,
			},
		},
	}
}

func decisionsCommand() *cli.Command {
	return &cli.Command{
		Name:  "decisions",
		Usage: "Decisions: choices made once on beliefs (ADR 0006)",
		Subcommands: []*cli.Command{
			{
				Name:      "show",
				Usage:     "Print the decisions with their reasons",
				ArgsUsage: "[path...] [--without <id>] [--ids]",
				Flags:     []cli.Flag{withoutFlag, idsFlag},
				Action:    decisionsShow,
			},
		},
	}
}

func actionsCommand() *cli.Command {
	return &cli.Command{
		Name:  "actions",
		Usage: "Actions: what to do because of a decision (ADR 0006)",
		Subcommands: []*cli.Command{
			{
				Name:      "show",
				Usage:     "Print open actions, then done ones, each with the decision it serves",
				ArgsUsage: "[path...] [--without <id>] [--ids]",
				Flags:     []cli.Flag{withoutFlag, idsFlag},
				Action:    actionsShow,
			},
		},
	}
}

// knowledgeArgs are a command's paths and flags. urfave/cli stops reading
// flags at the first argument, so flags written after the paths are picked
// out here.
type knowledgeArgs struct {
	paths   []string
	ids     bool
	without []string
	beliefs []string

	// rules judge only.
	as     string
	on     string
	model  string
	record bool
}

func parseArgs(args []string, c *cli.Context) knowledgeArgs {
	parsed := knowledgeArgs{ids: c.Bool("ids"), without: c.StringSlice("without"), beliefs: c.StringSlice("beliefs")}
	parsed.as = c.String("as")
	parsed.on = c.String("on")
	parsed.model = c.String("model")
	parsed.record = c.Bool("record")
	values := map[string]*[]string{"--without": &parsed.without, "--beliefs": &parsed.beliefs}
	singles := map[string]*string{"--as": &parsed.as, "--on": &parsed.on, "--model": &parsed.model}

	for i := 0; i < len(args); i++ {
		name, value, hasValue := strings.Cut(args[i], "=")
		target, isValueFlag := values[name]
		single, isSingle := singles[name]
		switch {
		case args[i] == "--ids":
			parsed.ids = true
		case args[i] == "--record":
			parsed.record = true
		case isSingle && hasValue:
			*single = value
		case isSingle && i+1 < len(args):
			*single = args[i+1]
			i++
		case isValueFlag && hasValue:
			*target = append(*target, value)
		case isValueFlag && i+1 < len(args):
			*target = append(*target, args[i+1])
			i++
		default:
			parsed.paths = append(parsed.paths, args[i])
		}
	}

	return parsed
}

// existing returns the paths that exist.
func existing(paths ...string) []string {
	found := []string{}
	for _, path := range paths {
		_, err := os.Stat(path)
		if err == nil {
			found = append(found, path)
		}
	}

	return found
}

// loadDefault loads paths, or evidence/ beliefs/ decisions/ rules/
// actions/ when none are given, judged without the ids in without. A
// what-if says so on its first line, so it cannot be mistaken for the
// real state.
func loadDefault(paths []string, without []string) (*knowledge.Set, error) {
	if len(paths) == 0 {
		paths = existing("evidence", "beliefs", "decisions", "rules", "actions")
	}
	if len(paths) == 0 {
		return nil, ez.New(ez.EINVALID, "no evidence/, beliefs/, decisions/, rules/ or actions/ here: pass the files or directories", nil)
	}

	set, err := knowledge.Load(paths...)
	if err != nil {
		return nil, err
	}

	return suppose(set, without)
}

func suppose(set *knowledge.Set, without []string) (*knowledge.Set, error) {
	if len(without) == 0 {
		return set, nil
	}

	view, err := set.Without(without)
	if err != nil {
		return nil, err
	}
	fmt.Printf("Without %s:\n\n", strings.Join(without, ", "))

	return view, nil
}

// loadRules loads the rule paths (default rules/) and what they rest on.
func loadRules(parsed knowledgeArgs) (*knowledge.Set, error) {
	paths := parsed.paths
	if len(paths) == 0 {
		paths = existing("rules")
	}
	if len(paths) == 0 {
		return nil, ez.New(ez.EINVALID, "no rules/ here: pass the rule files or directories", nil)
	}

	beliefs := parsed.beliefs
	if len(beliefs) == 0 {
		beliefs = existing("evidence", "beliefs", "decisions")
	}

	set, err := knowledge.Load(append(paths, beliefs...)...)
	if err != nil {
		return nil, err
	}

	return suppose(set, parsed.without)
}

// dim greys out a trailer of ids when printing to a terminal.
func dim(text string) string {
	info, err := os.Stdout.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 || os.Getenv("NO_COLOR") != "" {
		return text
	}

	return "\x1b[2m" + text + "\x1b[0m"
}

// printLine prints a sentence indented by its depth, with its trailer
// dim on its own line under it.
func printLine(line knowledge.Line) {
	indent := strings.Repeat("  ", line.Depth)
	fmt.Println(indent + line.Text)
	if line.Trailer != "" {
		fmt.Println(indent + "  " + dim(line.Trailer))
	}
}

func rulesCheck(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadRules(parsed)
	if err != nil {
		return err
	}

	// Checks can take a while, so say which one is running.
	run := func(ctx context.Context, command string) (string, error) {
		fmt.Fprintf(os.Stderr, "  ⏵ %s…\n", command)
		return knowledge.Shell(ctx, command)
	}

	counts := map[string]int{}
	for _, ruleset := range set.Rulesets {
		results, err := set.Check(c.Context, ruleset.File, run)
		if err != nil {
			return err
		}

		printChecklist(set, ruleset, results, false, parsed.ids)
		for _, result := range results {
			counts[mark(result)]++
		}
	}
	failed := counts["✗"]

	fmt.Printf("%d passed, %d failed, %d manual\n", counts["✓"], failed, counts["☐"])
	if failed > 0 {
		return ez.New(ez.EINVALID, fmt.Sprintf("rule checks failed: %d", failed), nil)
	}

	return nil
}

func rulesShow(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadRules(parsed)
	if err != nil {
		return err
	}

	for _, ruleset := range set.Rulesets {
		results := []knowledge.Result{}
		for _, rule := range ruleset.Rules {
			results = append(results, knowledge.Result{Rule: rule})
		}
		printChecklist(set, ruleset, results, true, parsed.ids)
	}

	return nil
}

func mark(result knowledge.Result) string {
	if result.Passed == nil {
		return "☐"
	}
	if *result.Passed {
		return "✓"
	}

	return "✗"
}

// printChecklist prints a ruleset under its subject, one line per rule,
// with a failed check's output indented under it. A rule resting on an
// opinion or a doubted belief says so; its check status is unchanged. A
// rule with no check says who it needs: its trusted judges, or a human.
func printChecklist(set *knowledge.Set, ruleset knowledge.Ruleset, results []knowledge.Result, withChecks bool, ids bool) {
	fmt.Printf("## %s\n\n", oneLine(ruleset.Subject))
	for _, result := range results {
		rule := result.Rule
		needs := knowledge.JudgeNote(rule)
		if needs != "" {
			needs = " — " + needs
		}

		if ids {
			line := fmt.Sprintf("- %s %s: %s", mark(result), rule.ID, oneLine(rule.Rule))
			if withChecks && rule.Check != "" {
				line += " [check: `" + rule.Check + "`]"
			}
			fmt.Println(line + because(rule.Supports()) + restsOn(set.StatusOf(rule.Supports())) + needs)
		} else {
			line := set.RuleLine(rule, withChecks)
			fmt.Println("- " + mark(result) + " " + line.Text + needs + " " + dim(line.Trailer))
		}

		if result.Passed != nil && !*result.Passed {
			for _, outputLine := range strings.Split(result.Output, "\n") {
				fmt.Println("      " + outputLine)
			}
		}
	}
	fmt.Println()
}

// because lists the ids a rule, decision or action rests on, alternatives
// after "or".
func because(sets []knowledge.Support) string {
	parts := []string{}
	for _, set := range sets {
		if len(set.Because) > 0 {
			parts = append(parts, strings.Join(set.Because, ", "))
		}
	}
	if len(parts) == 0 {
		return ""
	}

	return " (because: " + strings.Join(parts, " or ") + ")"
}

// restsOn names the opinions and doubted beliefs under a rule, a decision
// or an action, if any.
func restsOn(status knowledge.Status) string {
	text := ""
	if len(status.Opinions) > 0 {
		text += " (rests on opinion: " + strings.Join(status.Opinions, ", ") + ")"
	}
	if len(status.Doubted) > 0 {
		text += " (rests on doubted belief: " + strings.Join(status.Doubted, ", ") + ")"
	}

	return text
}

// statusLabel prints a belief's status in brackets, with the ids behind
// each reason it is not plainly grounded.
func statusLabel(status knowledge.Status) string {
	if status.Label == knowledge.Grounded {
		return " [grounded]"
	}

	text := ""
	if len(status.Opinions) > 0 {
		text += " [opinion: missing evidence on " + strings.Join(status.Opinions, ", ") + "]"
	}
	if len(status.Retracted) > 0 {
		text += " [in doubt: retracted evidence " + strings.Join(status.Retracted, ", ") + " under " + strings.Join(status.Doubted, ", ") + "]"
	}
	if len(status.Without) > 0 {
		text += " [in doubt: without " + strings.Join(status.Without, ", ") + "]"
	}
	if len(status.ContestedBy) > 0 {
		text += " [contested by " + strings.Join(status.ContestedBy, ", ") + "]"
	}

	return text
}

// oneLine folds a YAML block of text onto one line.
func oneLine(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func beliefsWhy(c *cli.Context) error {
	id := c.Args().First()
	if id == "" {
		return ez.New(ez.EINVALID, "usage: agc beliefs why <id> [path...]", nil)
	}

	parsed := parseArgs(c.Args().Tail(), c)
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	reason, err := set.Why(id)
	if err != nil {
		return err
	}

	if parsed.ids {
		printReason(reason, 0)
		return nil
	}

	for _, line := range set.WhyLines(reason, 0) {
		printLine(line)
	}

	return nil
}

// printReason prints a belief with the evidence it cites and, indented
// under it, what it rests on, alternatives after "or:". A leaf citing
// nothing is an opinion.
func printReason(reason knowledge.Reason, depth int) {
	belief := reason.Belief
	line := fmt.Sprintf("%s- %s: %s", strings.Repeat("  ", depth), belief.ID, oneLine(belief.Belief))
	line += evidenceIDs(reason.Evidence)
	if len(reason.Evidence) == 0 && len(reason.Because) == 0 {
		line += " — opinion: no evidence"
	}
	fmt.Println(line)

	for _, child := range reason.Because {
		printReason(child, depth+1)
	}

	for _, alternative := range reason.Or {
		fmt.Printf("%s  - or:%s\n", strings.Repeat("  ", depth), evidenceIDs(alternative.Evidence))
		for _, child := range alternative.Because {
			printReason(child, depth+2)
		}
	}
}

func evidenceIDs(list []knowledge.Evidence) string {
	text := ""
	for _, evidence := range list {
		if evidence.Retracted != nil {
			text += fmt.Sprintf(" — evidence: %s (retracted %s: %s)", evidence.ID, evidence.Retracted.When, oneLine(evidence.Retracted.Why))
			continue
		}
		translated := ""
		if evidence.TranslatedBy != "" {
			translated = ", translated by " + evidence.TranslatedBy
		}
		text += fmt.Sprintf(" — evidence: %s (%s, %s%s)", evidence.ID, evidence.Who, evidence.When, translated)
	}

	return text
}

func beliefsDoubt(c *cli.Context) error {
	id := c.Args().First()
	if id == "" {
		return ez.New(ez.EINVALID, "usage: agc beliefs doubt <id> [path...]", nil)
	}

	parsed := parseArgs(c.Args().Tail(), c)
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	doubted, err := set.Doubt(id)
	if err != nil {
		return err
	}

	if parsed.ids {
		printDoubtIDs(set, id, doubted)
		return nil
	}

	fmt.Printf("%s\n\n", set.DoubtHeader(id))

	lines := []string{}
	for _, belief := range doubted.Beliefs {
		lines = append(lines, "- I believe "+set.Phrase(belief.ID)+". "+dim("("+belief.ID+" · "+belief.File+")"))
	}
	printSection("Beliefs to recheck", lines)

	lines = []string{}
	for _, decision := range doubted.Decisions {
		lines = append(lines, "- I decided that "+set.Phrase(decision.ID)+". "+dim("("+decision.ID+adrTrailer(decision)+")"))
	}
	printSection("Decisions to review", lines)

	lines = []string{}
	for _, rule := range doubted.Rules {
		lines = append(lines, "- "+oneLine(rule.Rule)+" "+dim("("+rule.ID+" · "+rule.File+")"))
	}
	printSection("Rules to recheck", lines)

	lines = []string{}
	for _, action := range doubted.Actions {
		line := set.ActionLine(action)
		lines = append(lines, "- "+line.Text+" "+dim(line.Trailer))
	}
	printSection("Actions to reconsider", lines)

	lines = []string{}
	for _, standing := range doubted.Standing {
		line := set.StandingLine(standing)
		lines = append(lines, "- "+line.Text+" "+dim(line.Trailer))
	}
	printSection("Still standing (other support)", lines)

	return nil
}

func printDoubtIDs(set *knowledge.Set, id string, doubted knowledge.Doubted) {
	_, isBelief := findBelief(set, id)
	if isBelief {
		fmt.Printf("If %s is wrong, recheck what rests on it.\n\n", id)
	} else {
		fmt.Printf("If %s no longer holds, recheck what rests on it.\n\n", id)
	}

	lines := []string{}
	for _, belief := range doubted.Beliefs {
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", belief.ID, oneLine(belief.Belief), belief.File))
	}
	printSection("Beliefs to recheck", lines)

	lines = []string{}
	for _, decision := range doubted.Decisions {
		lines = append(lines, fmt.Sprintf("- %s: %s%s (%s)", decision.ID, oneLine(decision.Decision), adr(decision), decision.File))
	}
	printSection("Decisions to review", lines)

	lines = []string{}
	for _, rule := range doubted.Rules {
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", rule.ID, oneLine(rule.Rule), rule.File))
	}
	printSection("Rules to recheck", lines)

	lines = []string{}
	for _, action := range doubted.Actions {
		lines = append(lines, fmt.Sprintf("- %s: %s (%s)", action.ID, oneLine(action.Action), action.File))
	}
	printSection("Actions to reconsider", lines)

	lines = []string{}
	for _, standing := range doubted.Standing {
		lines = append(lines, fmt.Sprintf("- %s %s: %s%s (%s)", standing.Kind, standing.ID, oneLine(standing.Text), because(standing.Other), standing.File))
	}
	printSection("Still standing (other support)", lines)
}

func printSection(title string, lines []string) {
	fmt.Printf("## %s\n\n", title)
	if len(lines) == 0 {
		lines = []string{"- none"}
	}
	for _, line := range lines {
		fmt.Println(line)
	}
	fmt.Println()
}

func adr(decision knowledge.Decision) string {
	if decision.ADR == "" {
		return ""
	}

	return " [adr: " + decision.ADR + "]"
}

func adrTrailer(decision knowledge.Decision) string {
	if decision.ADR == "" {
		return ""
	}

	return " · " + decision.ADR
}

func decisionsShow(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadDefault(parsed.paths, parsed.without)
	if err != nil {
		return err
	}

	if !parsed.ids {
		for _, decision := range set.Decisions {
			printLine(set.DecisionLine(decision))
		}
		return nil
	}

	fmt.Printf("## Decisions\n\n")
	for _, decision := range set.Decisions {
		decided := fmt.Sprintf(" — decided by %s on %s", decision.Decided.Who, decision.Decided.When)
		fmt.Println("- " + decision.ID + ": " + oneLine(decision.Decision) + because(decision.Supports()) + adr(decision) + decided + restsOn(set.StatusOf(decision.Supports())))
	}

	return nil
}

// actionsShow prints the open actions, then the done ones.
func actionsShow(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadDefault(parsed.paths, parsed.without)
	if err != nil {
		return err
	}

	open := []knowledge.Action{}
	done := []knowledge.Action{}
	for _, action := range set.Actions {
		if action.Done == nil {
			open = append(open, action)
			continue
		}
		done = append(done, action)
	}

	for _, group := range []struct {
		title   string
		actions []knowledge.Action
	}{{"To do", open}, {"Done", done}} {
		fmt.Printf("## %s\n\n", group.title)
		if len(group.actions) == 0 {
			fmt.Printf("- none\n")
		}
		for _, action := range group.actions {
			if parsed.ids {
				fmt.Println(actionIDs(set, action))
				continue
			}
			printLine(set.ActionLine(action))
		}
		fmt.Println()
	}

	return nil
}

func actionIDs(set *knowledge.Set, action knowledge.Action) string {
	line := "- " + action.ID + ": " + oneLine(action.Action) + because(action.Supports())
	line += fmt.Sprintf(" — opened by %s on %s", action.Opened.Who, action.Opened.When)
	if action.Done != nil {
		return line + fmt.Sprintf(" — done by %s on %s", action.Done.Who, action.Done.When)
	}

	return line + restsOn(set.StatusOf(action.Supports()))
}

func findBelief(set *knowledge.Set, id string) (knowledge.Belief, bool) {
	for _, belief := range set.Beliefs {
		if belief.ID == id {
			return belief, true
		}
	}

	return knowledge.Belief{}, false
}

// beliefsShow prints every belief with its status, computed now from the
// evidence under it.
func beliefsShow(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadDefault(parsed.paths, parsed.without)
	if err != nil {
		return err
	}

	if !parsed.ids {
		for _, belief := range set.Beliefs {
			line, err := set.BeliefLine(belief.ID)
			if err != nil {
				return err
			}

			if knowledge.IsAgent(belief.By) {
				line, err = agentBeliefLine(set, belief, line)
				if err != nil {
					return ez.Wrap(err)
				}
			}
			printLine(line)
		}
		return nil
	}

	file := ""
	for _, belief := range set.Beliefs {
		if belief.File != file {
			if file != "" {
				fmt.Println()
			}
			file = belief.File
			fmt.Printf("## %s\n\n", file)
		}

		status, err := set.Status(belief.ID)
		if err != nil {
			return err
		}

		by := ""
		if belief.By != "" {
			by = " [by " + belief.By + on(belief.When) + "]"
		}
		fmt.Println("- " + belief.ID + ": " + oneLine(belief.Belief) + by + statusLabel(status))
	}

	return nil
}

// agentBeliefLine says a belief held by an agent as the agent's thought,
// "<agent> thinks …", never as mine, with its label: an agent's verdict
// with no evidence under it is an opinion.
func agentBeliefLine(set *knowledge.Set, belief knowledge.Belief, line knowledge.Line) (knowledge.Line, error) {
	status, err := set.Status(belief.ID)
	if err != nil {
		return line, ez.Wrap(err)
	}

	line.Text = belief.By + " thinks " + set.Phrase(belief.ID) + on(belief.When) + " (" + string(status.Label) + ")."

	return line, nil
}

func on(when string) string {
	if when == "" {
		return ""
	}

	return " on " + when
}

// rulesJudge asks the agent named by --as about every rule that trusts it,
// prints each verdict, and with --record appends them to
// beliefs/judgments.yaml as the agent's opinions. It fails on any fail or
// unclear verdict.
func rulesJudge(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	if parsed.as == "" {
		return ez.New(ez.EINVALID, "usage: agc rules judge --as <agent> [--on <path|git ref|->] [--record] [path...]", nil)
	}

	set, err := loadRules(parsed)
	if err != nil {
		return ez.Wrap(err)
	}

	subject, err := knowledge.ReadSubject(c.Context, parsed.on, os.Stdin)
	if err != nil {
		return ez.Wrap(err)
	}

	ask, err := harnessAsker(c.Context, parsed.as, parsed.model)
	if err != nil {
		return ez.Wrap(err)
	}

	judgments, err := set.Judge(c.Context, parsed.as, subject, ask)
	if err != nil {
		return ez.Wrap(err)
	}

	fmt.Printf("## Judged by %s on %s\n\n", parsed.as, subject.Name)
	counts := map[knowledge.Verdict]int{}
	skipped := 0
	for _, judgment := range judgments {
		rule := strings.TrimSuffix(oneLine(judgment.Rule.Rule), ".")
		trailer := " " + dim("("+judgment.Rule.ID+")")
		if judgment.Skipped {
			skipped++
			fmt.Printf("- skipped: %s is not trusted for this rule: %s%s\n", parsed.as, rule, trailer)
			continue
		}

		counts[judgment.Verdict]++
		switch judgment.Verdict {
		case knowledge.Pass:
			fmt.Printf("- ✓ judged by %s: %s — %s%s\n", parsed.as, rule, judgment.Reason, trailer)
		case knowledge.Fail:
			fmt.Printf("- ✗ judged by %s: %s — %s%s\n", parsed.as, rule, judgment.Reason, trailer)
		default:
			fmt.Printf("- ? unclear from %s: %s%s\n", parsed.as, rule, trailer)
			printUnclear(judgment)
		}
	}
	fmt.Printf("\n%d passed, %d failed, %d unclear, %d skipped\n", counts[knowledge.Pass], counts[knowledge.Fail], counts[knowledge.Unclear], skipped)

	if parsed.record {
		err = recordJudgments(c.Context, set, judgments, subject)
		if err != nil {
			return ez.Wrap(err)
		}
	}

	bad := counts[knowledge.Fail] + counts[knowledge.Unclear]
	if bad > 0 {
		return ez.New(ez.EINVALID, fmt.Sprintf("judged rules failed or unclear: %d", bad), nil)
	}

	return nil
}

// printUnclear shows why a verdict is unclear: the judge's error, or the
// start of an answer with no verdict line agc could read.
func printUnclear(judgment knowledge.Judgment) {
	text := judgment.Reason
	if text == "" {
		text = judgment.Answer
	}
	if text == "" {
		text = "(empty answer)"
	}

	lines := strings.Split(text, "\n")
	if len(lines) > 5 {
		lines = append(lines[:5], "…")
	}
	for _, line := range lines {
		fmt.Println("      " + line)
	}
}

// harnessAsker runs the judge through the same harness code workflows use,
// on this machine, read only (the harnesses' default permissions). A
// person has no asker: knowledge.Judge says so.
func harnessAsker(ctx context.Context, as string, model string) (knowledge.Asker, error) {
	if !knowledge.IsHarness(as) {
		return nil, nil
	}

	kind := agent.Harness(as)
	for _, info := range harnesses.ListHarnessInfo(ctx) {
		if info.ID != kind {
			continue
		}
		if !info.Available {
			return nil, ez.New(ez.EINVALID, as+" cannot judge here: "+info.Binary+" is not on PATH", nil)
		}
		if model == "" && len(info.Models) > 0 {
			model = info.Models[0]
		}
	}
	if model == "" {
		return nil, ez.New(ez.EINVALID, "no model known for "+as+": pass --model", nil)
	}

	harness, err := harnesses.New(kind, host.New())
	if err != nil {
		return nil, ez.Wrap(err)
	}

	ask := func(ctx context.Context, rule knowledge.Rule, prompt string) (string, error) {
		fmt.Fprintf(os.Stderr, "  ⏵ %s (%s) judging %s…\n", as, model, rule.ID)
		conversation := &agent.Conversation{Harness: kind, Model: model, Instructions: prompt}
		result, err := harness.Run(ctx, conversation, prompt)
		if err != nil {
			if result != nil && strings.TrimSpace(result.HarnessError) != "" {
				return "", ez.New(ez.EINTERNAL, strings.TrimSpace(result.HarnessError), err)
			}
			return "", ez.Wrap(err)
		}

		return result.LastAssistantMessage, nil
	}

	return ask, nil
}

// recordJudgments appends the pass and fail verdicts to
// beliefs/judgments.yaml as opinions by the judge, ids ending in the short
// sha of HEAD, or today's date outside git.
func recordJudgments(ctx context.Context, set *knowledge.Set, judgments []knowledge.Judgment, subject knowledge.Subject) error {
	today := time.Now().Format("2006-01-02")
	suffix := knowledge.ShortHead(ctx)
	if suffix == "" {
		suffix = today
	}

	path := filepath.Join("beliefs", "judgments.yaml")
	written, err := knowledge.Record(path, judgments, subject, suffix, today, loadedIDs(set))
	if err != nil {
		return ez.Wrap(err)
	}

	fmt.Printf("Recorded %d opinions in %s, never evidence.\n", len(written), path)
	for _, belief := range written {
		fmt.Println("  " + dim(belief.ID))
	}

	return nil
}

// loadedIDs is every id loaded, so recorded ids do not collide.
func loadedIDs(set *knowledge.Set) map[string]bool {
	ids := map[string]bool{}
	for _, evidence := range set.Evidence {
		ids[evidence.ID] = true
	}
	for _, belief := range set.Beliefs {
		ids[belief.ID] = true
	}
	for _, decision := range set.Decisions {
		ids[decision.ID] = true
	}
	for _, action := range set.Actions {
		ids[action.ID] = true
	}
	for _, ruleset := range set.Rulesets {
		for _, rule := range ruleset.Rules {
			ids[rule.ID] = true
		}
	}

	return ids
}

// beliefsConflicts prints every contested contradiction with what each
// side comes down to. It never picks a side.
func beliefsConflicts(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	conflicts := set.Conflicts()
	if len(conflicts) == 0 {
		fmt.Println("Nothing I hold is contested.")
		return nil
	}

	for i, conflict := range conflicts {
		if parsed.ids {
			fmt.Printf("- %s ↔ %s: %s | %s\n", conflict.A.ID, conflict.B.ID, sideIDs(conflict.A), sideIDs(conflict.B))
			continue
		}

		if i > 0 {
			fmt.Println()
		}
		for _, line := range set.ConflictLines(conflict) {
			printLine(line)
		}
	}

	return nil
}

func sideIDs(side knowledge.Side) string {
	ids := []string{}
	for _, evidence := range side.Evidence {
		ids = append(ids, evidence.ID)
	}
	text := side.ID + " ← evidence: " + strings.Join(ids, ", ")
	if len(side.Opinions) > 0 {
		text += ", opinion: " + strings.Join(side.Opinions, ", ")
	}

	return text
}

// evidenceShow prints every piece of evidence with how many beliefs cite
// it, so unused evidence is visible.
func evidenceShow(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	if !parsed.ids {
		for _, evidence := range set.Evidence {
			printLine(set.EvidenceLine(evidence))
		}
		return nil
	}

	file := ""
	for _, evidence := range set.Evidence {
		if evidence.File != file {
			if file != "" {
				fmt.Println()
			}
			file = evidence.File
			fmt.Printf("## %s\n\n", file)
		}

		line := fmt.Sprintf("- %s: %s — %s, %s", evidence.ID, oneLine(evidence.Evidence), evidence.Who, evidence.When)
		if evidence.Source != "" {
			line += " [source: " + evidence.Source + "]"
		}
		if evidence.TranslatedBy != "" {
			line += " [translated by " + evidence.TranslatedBy + "]"
		}
		if evidence.Retracted != nil {
			retracted := evidence.Retracted
			line += fmt.Sprintf(" [retracted by %s on %s: %s]", retracted.Who, retracted.When, oneLine(retracted.Why))
		}

		cited := len(set.Citing(evidence.ID))
		noun := "beliefs"
		if cited == 1 {
			noun = "belief"
		}
		fmt.Printf("%s (cited by %d %s)\n", line, cited, noun)
	}

	return nil
}
