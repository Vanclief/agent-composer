package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/knowledge"
)

// The rules, beliefs and decisions commands
// (docs/decisions/0006-rules-and-beliefs.md). Output is plain markdown so
// it can be pasted into an agent's context. Nothing is ever written.

var beliefsFlag = &cli.StringSliceFlag{
	Name:  "beliefs",
	Usage: "Beliefs and decisions the rules rest on (default beliefs/ decisions/ when they exist)",
}

func rulesCommand() *cli.Command {
	return &cli.Command{
		Name:  "rules",
		Usage: "Rules: what must or must not be done (ADR 0006)",
		Subcommands: []*cli.Command{
			{
				Name:      "check",
				Usage:     "Run the rules' checks and print a checklist; fails if a check fails",
				ArgsUsage: "[path...] [--beliefs <path>]",
				Flags:     []cli.Flag{beliefsFlag},
				Action:    rulesCheck,
			},
			{
				Name:      "show",
				Usage:     "Print the rules as a markdown checklist without running checks",
				ArgsUsage: "[path...] [--beliefs <path>]",
				Flags:     []cli.Flag{beliefsFlag},
				Action:    rulesShow,
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
				Name:      "why",
				Usage:     "Follow a belief down to the observations it rests on",
				ArgsUsage: "<id> [path...]",
				Action:    beliefsWhy,
			},
			{
				Name:      "doubt",
				Usage:     "List the beliefs, decisions and rules that fall if a belief is wrong",
				ArgsUsage: "<id> [path...]",
				Action:    beliefsDoubt,
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
				Usage:     "Print the decisions as markdown",
				ArgsUsage: "[path...]",
				Action:    decisionsShow,
			},
		},
	}
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

// loadDefault loads paths, or beliefs/ decisions/ rules/ when none are
// given.
func loadDefault(paths []string) (*knowledge.Set, error) {
	if len(paths) == 0 {
		paths = existing("beliefs", "decisions", "rules")
	}
	if len(paths) == 0 {
		return nil, ez.New(ez.EINVALID, "no beliefs/, decisions/ or rules/ here: pass the files or directories", nil)
	}

	return knowledge.Load(paths...)
}

// loadRules loads the rule paths (default rules/) and what they rest on.
// urfave/cli stops reading flags at the first argument, so a --beliefs
// written after the paths is picked out here.
func loadRules(c *cli.Context) (*knowledge.Set, error) {
	paths := []string{}
	beliefs := c.StringSlice("beliefs")
	args := c.Args().Slice()
	for i := 0; i < len(args); i++ {
		value, isFlag := strings.CutPrefix(args[i], "--beliefs=")
		switch {
		case isFlag:
			beliefs = append(beliefs, value)
		case args[i] == "--beliefs" && i+1 < len(args):
			beliefs = append(beliefs, args[i+1])
			i++
		default:
			paths = append(paths, args[i])
		}
	}

	if len(paths) == 0 {
		paths = existing("rules")
	}
	if len(paths) == 0 {
		return nil, ez.New(ez.EINVALID, "no rules/ here: pass the rule files or directories", nil)
	}
	if len(beliefs) == 0 {
		beliefs = existing("beliefs", "decisions")
	}

	return knowledge.Load(append(paths, beliefs...)...)
}

func rulesCheck(c *cli.Context) error {
	set, err := loadRules(c)
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

		printChecklist(ruleset, results, false)
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
	set, err := loadRules(c)
	if err != nil {
		return err
	}

	for _, ruleset := range set.Rulesets {
		results := []knowledge.Result{}
		for _, rule := range ruleset.Rules {
			results = append(results, knowledge.Result{Rule: rule})
		}
		printChecklist(ruleset, results, true)
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
// with a failed check's output indented under it.
func printChecklist(ruleset knowledge.Ruleset, results []knowledge.Result, withChecks bool) {
	fmt.Printf("## %s\n\n", oneLine(ruleset.Subject))
	for _, result := range results {
		rule := result.Rule
		line := fmt.Sprintf("- %s %s: %s", mark(result), rule.ID, oneLine(rule.Rule))
		if withChecks && rule.Check != "" {
			line += " [check: `" + rule.Check + "`]"
		}
		fmt.Println(line + because(rule.Because))

		if result.Passed != nil && !*result.Passed {
			for _, outputLine := range strings.Split(result.Output, "\n") {
				fmt.Println("      " + outputLine)
			}
		}
	}
	fmt.Println()
}

func because(ids []string) string {
	if len(ids) == 0 {
		return ""
	}

	return " (because: " + strings.Join(ids, ", ") + ")"
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

	set, err := loadDefault(c.Args().Tail())
	if err != nil {
		return err
	}

	reason, err := set.Why(id)
	if err != nil {
		return err
	}

	printReason(reason, 0)

	return nil
}

// printReason prints a belief and, indented under it, what it rests on.
func printReason(reason knowledge.Reason, depth int) {
	belief := reason.Belief
	line := fmt.Sprintf("%s- %s: %s", strings.Repeat("  ", depth), belief.ID, oneLine(belief.Belief))
	if belief.Observed != nil {
		line += fmt.Sprintf(" — observed by %s on %s", belief.Observed.Who, belief.Observed.When)
	}
	fmt.Println(line)

	for _, child := range reason.Because {
		printReason(child, depth+1)
	}
}

func beliefsDoubt(c *cli.Context) error {
	id := c.Args().First()
	if id == "" {
		return ez.New(ez.EINVALID, "usage: agc beliefs doubt <id> [path...]", nil)
	}

	set, err := loadDefault(c.Args().Tail())
	if err != nil {
		return err
	}

	doubted, err := set.Doubt(id)
	if err != nil {
		return err
	}

	fmt.Printf("If %s is wrong, recheck what rests on it.\n\n", id)

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

	return nil
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

func decisionsShow(c *cli.Context) error {
	set, err := loadDefault(c.Args().Slice())
	if err != nil {
		return err
	}

	fmt.Printf("## Decisions\n\n")
	for _, decision := range set.Decisions {
		decided := fmt.Sprintf(" — decided by %s on %s", decision.Decided.Who, decision.Decided.When)
		fmt.Println("- " + decision.ID + ": " + oneLine(decision.Decision) + because(decision.Because) + adr(decision) + decided)
	}

	return nil
}
