package cli

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	cli "github.com/urfave/cli/v2"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/knowledge"
)

// The proposals and gates commands
// (docs/decisions/0007-proposals-and-gates.md). Everything an agent
// produces is a proposal; gates, written by a human, say which land
// without one. show groups proposals by what they need now, why follows
// one down to the evidence, and accept and reject write the verdict on
// the proposal, nothing else. gates show prints each gate with how the
// proposals under it have gone.

var byFlag = &cli.StringFlag{
	Name:  "by",
	Usage: "Who decides (default: the first name in git config user.name)",
}

func proposalsCommand() *cli.Command {
	return &cli.Command{
		Name:  "proposals",
		Usage: "Proposals: what agents put forward, and what each needs before it lands (ADR 0007)",
		Subcommands: []*cli.Command{
			{
				Name:      "show",
				Usage:     "Print the proposals grouped by what they need now: my decision, my eyes, or nothing",
				ArgsUsage: "[path...] [--ids]",
				Flags:     []cli.Flag{idsFlag},
				Action:    proposalsShow,
			},
			{
				Name:      "why",
				Usage:     "Follow a proposal down to the beliefs and evidence it rests on",
				ArgsUsage: "<id> [path...] [--ids]",
				Flags:     []cli.Flag{idsFlag},
				Action:    proposalsWhy,
			},
			{
				Name:      "accept",
				Usage:     "Write accepted: { who, when } on an open proposal",
				ArgsUsage: "<id> [path...] [--by <who>]",
				Flags:     []cli.Flag{byFlag},
				Action:    proposalsAccept,
			},
			{
				Name:      "reject",
				Usage:     "Write rejected: { who, when, why } on an open proposal; why is required",
				ArgsUsage: "<id> --why <text> [path...] [--by <who>]",
				Flags: []cli.Flag{
					byFlag,
					&cli.StringFlag{Name: "why", Usage: "What you saw that the proposer did not"},
				},
				Action: proposalsReject,
			},
		},
	}
}

func gatesCommand() *cli.Command {
	return &cli.Command{
		Name:  "gates",
		Usage: "Gates: which proposals land without a human, and how each gate has gone (ADR 0007)",
		Subcommands: []*cli.Command{
			{
				Name:      "show",
				Usage:     "Print every gate with its tier now and its accepted, rejected and open counts",
				ArgsUsage: "[path...] [--ids]",
				Flags:     []cli.Flag{idsFlag},
				Action:    gatesShow,
			},
		},
	}
}

// proposalsShow prints the proposals in groups: what needs my decision
// first, then what landed for me to see, what landed, and what I already
// accepted or rejected.
func proposalsShow(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	groups := []struct {
		title string
		keep  func(knowledge.Review) bool
	}{
		{"Needs my decision", func(r knowledge.Review) bool { return r.Proposal.Open() && r.Tier == knowledge.Decide }},
		{"Landed, for me to see", func(r knowledge.Review) bool { return r.Proposal.Open() && r.Tier == knowledge.Observe }},
		{"Landed", func(r knowledge.Review) bool { return r.Proposal.Open() && r.Tier == knowledge.Delegated }},
		{"Accepted", func(r knowledge.Review) bool { return r.Proposal.Accepted != nil }},
		{"Rejected", func(r knowledge.Review) bool { return r.Proposal.Rejected != nil }},
	}

	reviews := set.Reviews()
	for _, group := range groups {
		lines := []string{}
		for _, review := range reviews {
			if !group.keep(review) {
				continue
			}
			if parsed.ids {
				lines = append(lines, proposalIDs(review))
				continue
			}
			line := set.ProposalLine(review)
			lines = append(lines, "- "+line.Text+" "+dim(line.Trailer))
		}
		printSection(group.title, lines)
	}

	return nil
}

// proposalIDs is the compact form of a proposal: its id, kind, proposer,
// text, reasons, tier and verdict.
func proposalIDs(review knowledge.Review) string {
	proposal := review.Proposal
	line := fmt.Sprintf("- %s [%s by %s on %s]: %s", proposal.ID, proposal.Proposes, proposal.By, proposal.When, oneLine(proposal.Proposal))
	line += because([]knowledge.Support{proposal.Support()})
	if len(proposal.Evidence) > 0 {
		line += " (evidence: " + strings.Join(proposal.Evidence, ", ") + ")"
	}
	line += restsOn(review.Status)

	tier := " — " + string(review.Tier)
	switch {
	case review.Gate == "":
		tier += " by default"
	default:
		tier += " via " + review.Gate
	}
	if review.Illegible {
		tier += " (illegible: rests on nothing)"
	}
	if review.Audited {
		tier += " (audited)"
	}
	if len(review.TightenedBy) > 0 {
		tier += " (tightened by " + strings.Join(review.TightenedBy, ", ") + ")"
	}
	line += tier

	switch {
	case proposal.Accepted != nil:
		line += fmt.Sprintf(" — accepted by %s on %s", proposal.Accepted.Who, proposal.Accepted.When)
	case proposal.Rejected != nil:
		line += fmt.Sprintf(" — rejected by %s on %s: %s", proposal.Rejected.Who, proposal.Rejected.When, oneLine(proposal.Rejected.Why))
	}
	if proposal.Became != "" {
		line += " [now: " + proposal.Became + "]"
	}

	return line
}

// proposalsWhy prints a proposal and, under it, each belief it rests on
// down to its evidence, each decision it rests on, and the evidence it
// cites itself: the chain to read before accepting.
func proposalsWhy(c *cli.Context) error {
	id := c.Args().First()
	if id == "" {
		return ez.New(ez.EINVALID, "usage: agc proposals why <id> [path...]", nil)
	}

	parsed := parseArgs(c.Args().Tail(), c)
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	review, err := set.Review(id)
	if err != nil {
		return err
	}

	if parsed.ids {
		fmt.Println(proposalIDs(review))
	} else {
		printLine(set.ProposalLine(review))
	}

	for _, next := range review.Proposal.Because {
		reason, err := set.Why(next)
		if err == nil {
			if parsed.ids {
				printReason(reason, 1)
				continue
			}
			for _, line := range set.WhyLines(reason, 1) {
				printLine(line)
			}
			continue
		}

		decision, found := findDecision(set, next)
		if !found {
			return ez.New(ez.ENOTFOUND, "because names "+next+", which is not a loaded belief or decision", nil)
		}
		if parsed.ids {
			fmt.Println("  - " + decision.ID + ": " + oneLine(decision.Decision) + because(decision.Supports()))
			continue
		}
		line := set.DecisionLine(decision)
		line.Depth = 1
		printLine(line)
	}

	for _, evidenceID := range review.Proposal.Evidence {
		evidence, found := findEvidence(set, evidenceID)
		if !found {
			continue
		}
		if parsed.ids {
			fmt.Println("  -" + evidenceIDs([]knowledge.Evidence{evidence}))
			continue
		}
		line := set.EvidenceLine(evidence)
		line.Depth = 1
		printLine(line)
	}

	return nil
}

func findDecision(set *knowledge.Set, id string) (knowledge.Decision, bool) {
	for _, decision := range set.Decisions {
		if decision.ID == id {
			return decision, true
		}
	}

	return knowledge.Decision{}, false
}

func findEvidence(set *knowledge.Set, id string) (knowledge.Evidence, bool) {
	for _, evidence := range set.Evidence {
		if evidence.ID == id {
			return evidence, true
		}
	}

	return knowledge.Evidence{}, false
}

func proposalsAccept(c *cli.Context) error {
	id := c.Args().First()
	if id == "" {
		return ez.New(ez.EINVALID, "usage: agc proposals accept <id> [path...] [--by <who>]", nil)
	}

	parsed := parseArgs(c.Args().Tail(), c)
	set, review, who, err := loadDecision(c.Context, parsed, id)
	if err != nil {
		return ez.Wrap(err)
	}

	err = knowledge.Accept(review.Proposal.File, id, who, time.Now().Format("2006-01-02"))
	if err != nil {
		return ez.Wrap(err)
	}

	return reprint(set, parsed, id, "Accepted")
}

func proposalsReject(c *cli.Context) error {
	id := c.Args().First()
	if id == "" {
		return ez.New(ez.EINVALID, "usage: agc proposals reject <id> --why <text> [path...] [--by <who>]", nil)
	}

	parsed := parseArgs(c.Args().Tail(), c)
	set, review, who, err := loadDecision(c.Context, parsed, id)
	if err != nil {
		return ez.Wrap(err)
	}

	err = knowledge.Reject(review.Proposal.File, id, who, time.Now().Format("2006-01-02"), parsed.why)
	if err != nil {
		return ez.Wrap(err)
	}

	return reprint(set, parsed, id, "Rejected")
}

// loadDecision loads the set, finds the proposal and settles who decides.
func loadDecision(ctx context.Context, parsed knowledgeArgs, id string) (*knowledge.Set, knowledge.Review, string, error) {
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return nil, knowledge.Review{}, "", err
	}

	review, err := set.Review(id)
	if err != nil {
		return nil, knowledge.Review{}, "", err
	}

	who := parsed.by
	if who == "" {
		who = gitFirstName(ctx)
	}
	if who == "" {
		return nil, knowledge.Review{}, "", ez.New(ez.EINVALID, "who decides? pass --by <who>; git config user.name is not set", nil)
	}

	return set, review, who, nil
}

// reprint reloads the files the set came from and prints the proposal as
// it reads now.
func reprint(set *knowledge.Set, parsed knowledgeArgs, id string, verb string) error {
	again, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	review, err := again.Review(id)
	if err != nil {
		return err
	}

	fmt.Printf("%s %s in %s.\n", verb, id, review.Proposal.File)
	printLine(again.ProposalLine(review))

	return nil
}

// gitFirstName is the first name in git config user.name, lower case, as
// the repo's who fields are written, or empty outside git.
func gitFirstName(ctx context.Context) string {
	output, err := exec.CommandContext(ctx, "git", "config", "user.name").Output()
	if err != nil {
		return ""
	}

	fields := strings.Fields(string(output))
	if len(fields) == 0 {
		return ""
	}

	return strings.ToLower(fields[0])
}

// gatesShow prints every gate with its tier now and its counts, then the
// default for proposals no gate covers.
func gatesShow(c *cli.Context) error {
	parsed := parseArgs(c.Args().Slice(), c)
	set, err := loadDefault(parsed.paths, nil)
	if err != nil {
		return err
	}

	for _, report := range set.GateReports() {
		if parsed.ids {
			fmt.Println(gateIDs(report))
			continue
		}
		printLine(set.GateLine(report))
	}

	return nil
}

// gateIDs is the compact form of a gate report.
func gateIDs(report knowledge.GateReport) string {
	gate := report.Gate
	tally := fmt.Sprintf(" — %d accepted, %d rejected, %d open", report.Accepted, report.Rejected, report.Open)
	if report.Audited > 0 {
		tally += fmt.Sprintf(", %d audited", report.Audited)
	}
	if gate.ID == "" {
		return "- default: decide" + tally
	}

	who := "anyone"
	if len(gate.By) > 0 {
		who = strings.Join(gate.By, ", ")
	}
	line := fmt.Sprintf("- %s: %s by %s → %s", gate.ID, gate.Proposes, who, gate.Tier)
	if gate.Sample > 0 {
		line += fmt.Sprintf(" (sample 1/%d)", gate.Sample)
	}
	if gate.Since != "" {
		line += " (since " + gate.Since + ")"
	}
	if len(report.TightenedBy) > 0 {
		line += " (now decide: tightened by " + strings.Join(report.TightenedBy, ", ") + ")"
	}
	line += because(gate.Supports()) + restsOn(report.Status) + tally
	if report.AllAccepted {
		line += " (all accepted: move to observe or audit)"
	}

	return line
}
