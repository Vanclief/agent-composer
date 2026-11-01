package knowledge

import (
	"bytes"
	"hash/fnv"
	"os"
	"slices"
	"strings"

	"github.com/vanclief/ez"
	"gopkg.in/yaml.v3"
)

// Gates: everything an agent produces is a proposal, and gates, written by
// a human, say which proposals land without one. The tier a proposal gets
// is computed each time from the gates, the rejections under them and a
// sample, never stored. The gates themselves are the one thing no gate
// covers: a gate change always needs a human
// (docs/decisions/0007-proposals-and-gates.md).

// Tier says how much of a human a proposal needs before it lands.
type Tier string

const (
	// Decide means a human accepts the proposal before it lands. It is
	// the default, and what every gate falls back to.
	Decide Tier = "decide"
	// Observe means the proposal lands and a human sees it; the gate may
	// audit a sample of them as if it were decide.
	Observe Tier = "observe"
	// Delegated means the proposal lands without a human seeing it.
	Delegated Tier = "delegated"
)

// tiers lists the tiers from the strictest down, the order a gate's
// strictness is compared in.
var tiers = []Tier{Decide, Observe, Delegated}

// Proposable lists what a proposal may propose. A goal is not on it on
// purpose: what we want is never proposed, only how to get there.
var Proposable = []string{"rule", "belief", "evidence", "decision", "action", "gate", "code"}

// acts lists the kinds of proposal that do something when they land, so
// one of them resting on nothing cannot be read: there is no why to
// follow before accepting it. A belief or a piece of evidence resting on
// nothing is an opinion, which is a readable thing to be.
var acts = []string{"rule", "decision", "action", "gate", "code"}

// stricter says whether a is at least as strict as b.
func stricter(a Tier, b Tier) bool {
	return slices.Index(tiers, a) <= slices.Index(tiers, b)
}

// Review is how a proposal stands at the gate: which gate covers it, the
// tier that gate is written with, the tier it gets now and why they
// differ. Status is how what the proposal rests on is grounded, and
// Illegible says it acts and rests on nothing, so there is no why to
// read: the reader rejects it rather than trusts it.
type Review struct {
	Proposal Proposal `json:"proposal"`
	// Gate is the id of the gate that covers the proposal, empty when
	// none does and the default applies.
	Gate    string `json:"gate,omitempty"`
	Written Tier   `json:"written"`
	Tier    Tier   `json:"tier"`
	// Audited says an observe gate sampled this proposal, so it needs a
	// decision although the gate lands the rest.
	Audited bool `json:"audited,omitempty"`
	// TightenedBy lists the rejected proposals under the same gate that
	// pulled it to decide.
	TightenedBy []string `json:"tightened_by,omitempty"`
	Status      Status   `json:"status"`
	Illegible   bool     `json:"illegible,omitempty"`
}

// Review computes how the proposal id stands at the gate now.
func (s *Set) Review(id string) (Review, error) {
	proposal, found := s.proposal(id)
	if !found {
		return Review{}, ez.New(ez.ENOTFOUND, "proposal "+id+" not found", nil)
	}

	return s.review(proposal), nil
}

// Reviews computes every proposal's review, in load order.
func (s *Set) Reviews() []Review {
	reviews := []Review{}
	for _, proposal := range s.Proposals {
		reviews = append(reviews, s.review(proposal))
	}

	return reviews
}

func (s *Set) review(proposal Proposal) Review {
	support := proposal.Support()
	// A proposed belief or piece of evidence resting on nothing is an
	// opinion, as a belief would be; anything else resting on nothing has
	// no leaves and is grounded, and Illegible says what that means.
	opines := proposal.Proposes == "belief" || proposal.Proposes == "evidence"
	review := Review{
		Proposal:  proposal,
		Written:   Decide,
		Tier:      Decide,
		Status:    s.judgeSets(proposal.ID, []Support{support}, opines),
		Illegible: slices.Contains(acts, proposal.Proposes) && len(support.Because) == 0 && len(support.Evidence) == 0,
	}

	// A gate change is the fixed point: no gate covers it.
	if proposal.Proposes == "gate" {
		return review
	}

	gate, covered := s.gateFor(proposal)
	if !covered {
		return review
	}
	review.Gate = gate.ID
	review.Written = gate.Tier
	review.Tier = gate.Tier

	// Legibility is the gate before any gate: what cannot be read is not
	// landed, whatever its gate says.
	if review.Illegible {
		review.Tier = Decide
		return review
	}

	review.TightenedBy = s.tightenedBy(gate)
	if len(review.TightenedBy) > 0 {
		review.Tier = Decide
		return review
	}

	if gate.Tier == Observe && gate.Sample > 0 && sampled(proposal.ID, gate.Sample) {
		review.Audited = true
		review.Tier = Decide
	}

	return review
}

// gateFor finds the gate covering a proposal: one on its kind that names
// its proposer or nobody in particular. When several cover it the
// strictest wins, and among equals the first loaded.
func (s *Set) gateFor(proposal Proposal) (Gate, bool) {
	var chosen Gate
	covered := false
	for _, gate := range s.Gates {
		if !gate.covers(proposal) {
			continue
		}
		if !covered || (stricter(gate.Tier, chosen.Tier) && gate.Tier != chosen.Tier) {
			chosen = gate
			covered = true
		}
	}

	return chosen, covered
}

func (gate Gate) covers(proposal Proposal) bool {
	if gate.Proposes != proposal.Proposes {
		return false
	}

	return len(gate.By) == 0 || slices.Contains(gate.By, proposal.By)
}

// tightenedBy lists the proposals under the gate rejected on or after its
// since, which pull it to decide. A decide gate has nowhere to tighten
// to, so its rejections are only counted.
func (s *Set) tightenedBy(gate Gate) []string {
	if gate.Tier == Decide {
		return nil
	}

	ids := []string{}
	for _, proposal := range s.Proposals {
		if !gate.covers(proposal) || proposal.Rejected == nil {
			continue
		}
		if proposal.Rejected.When >= gate.Since {
			ids = append(ids, proposal.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}

	return ids
}

// sampled picks about one proposal in n by hashing its id, so the same
// proposal is audited whatever order the files load in.
func sampled(id string, n int) bool {
	h := fnv.New32a()
	h.Write([]byte(id))

	return h.Sum32()%uint32(n) == 0
}

// rubberStamp is how many accepted proposals with no rejection make a
// decide gate look like a ritual: either the proposer is perfect or the
// human has stopped deciding, and the report says so.
const rubberStamp = 5

// GateReport is a gate with the tier it gives now and how the proposals
// under it have gone. The default, for proposals no gate covers, is a
// report whose gate has an empty id and the decide tier.
type GateReport struct {
	Gate        Gate     `json:"gate"`
	Tier        Tier     `json:"tier"`
	TightenedBy []string `json:"tightened_by,omitempty"`
	Open        int      `json:"open"`
	Accepted    int      `json:"accepted"`
	Rejected    int      `json:"rejected"`
	Audited     int      `json:"audited"`
	// AllAccepted says a decide gate has accepted rubberStamp or more
	// proposals and rejected none: time to move it to observe honestly,
	// or to audit it.
	AllAccepted bool   `json:"all_accepted"`
	Status      Status `json:"status"`
}

// Gates reports every gate in load order, then the default for the
// proposals no gate covers.
func (s *Set) GateReports() []GateReport {
	reports := []GateReport{}
	for _, gate := range s.Gates {
		report := GateReport{Gate: gate, Tier: gate.Tier, TightenedBy: s.tightenedBy(gate), Status: s.StatusOf(gate.Supports())}
		if len(report.TightenedBy) > 0 {
			report.Tier = Decide
		}
		reports = append(reports, report)
	}
	reports = append(reports, GateReport{Gate: Gate{Tier: Decide}, Tier: Decide, Status: Status{Label: Grounded}})

	for _, review := range s.Reviews() {
		index := len(reports) - 1
		if review.Gate != "" {
			index = slices.IndexFunc(reports, func(report GateReport) bool { return report.Gate.ID == review.Gate })
		}
		report := &reports[index]
		switch {
		case review.Proposal.Accepted != nil:
			report.Accepted++
		case review.Proposal.Rejected != nil:
			report.Rejected++
		default:
			report.Open++
		}
		if review.Audited {
			report.Audited++
		}
	}

	for i := range reports {
		report := &reports[i]
		report.AllAccepted = report.Gate.Tier == Decide && report.Rejected == 0 && report.Accepted >= rubberStamp
	}

	return reports
}

func (s *Set) proposal(id string) (Proposal, bool) {
	for _, proposal := range s.Proposals {
		if proposal.ID == id {
			return proposal, true
		}
	}

	return Proposal{}, false
}

// Accept writes accepted: { who, when } on the open proposal id in the
// file at path, keeping everything else in the file as it is.
func Accept(path string, id string, who string, when string) error {
	return decide(path, id, "accepted", Observation{Who: who, When: when})
}

// Reject writes rejected: { who, when, why } on the open proposal id in
// the file at path. Why is required: an override is a record of what the
// human saw that the proposer did not.
func Reject(path string, id string, who string, when string, why string) error {
	if strings.TrimSpace(why) == "" {
		return ez.New(ez.EINVALID, "rejecting "+id+" needs why: what you saw that the proposer did not", nil)
	}

	return decide(path, id, "rejected", Rejection{Who: who, When: when, Why: why})
}

func decide(path string, id string, key string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return ez.New(ez.EINVALID, "cannot read "+path, err)
	}

	var doc yaml.Node
	err = yaml.Unmarshal(data, &doc)
	if err != nil {
		return ez.New(ez.EINVALID, path+": "+err.Error(), err)
	}

	item := proposalNode(&doc, id)
	if item == nil {
		return ez.New(ez.ENOTFOUND, "proposal "+id+" not found in "+path, nil)
	}

	for i := 0; i+1 < len(item.Content); i += 2 {
		field := item.Content[i].Value
		if field == "accepted" || field == "rejected" {
			return ez.New(ez.EINVALID, "proposal "+id+" is already "+field, nil)
		}
	}

	var mapping yaml.Node
	err = mapping.Encode(value)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot encode "+key, err)
	}
	mapping.Style = yaml.FlowStyle
	item.Content = append(item.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}, &mapping)

	var out bytes.Buffer
	encoder := yaml.NewEncoder(&out)
	encoder.SetIndent(2)
	err = encoder.Encode(&doc)
	if err != nil {
		return ez.New(ez.EINTERNAL, "cannot encode "+path, err)
	}

	err = os.WriteFile(path, out.Bytes(), 0o644)
	if err != nil {
		return ez.New(ez.EINVALID, "cannot write "+path, err)
	}

	return nil
}

// proposalNode finds the mapping of the proposal id under proposals: in a
// parsed file, or nil.
func proposalNode(doc *yaml.Node, id string) *yaml.Node {
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil
	}

	root := doc.Content[0]
	for i := 0; i+1 < len(root.Content); i += 2 {
		if root.Content[i].Value != "proposals" || root.Content[i+1].Kind != yaml.SequenceNode {
			continue
		}
		for _, item := range root.Content[i+1].Content {
			if item.Kind != yaml.MappingNode {
				continue
			}
			for j := 0; j+1 < len(item.Content); j += 2 {
				if item.Content[j].Value == "id" && item.Content[j+1].Value == id {
					return item
				}
			}
		}
	}

	return nil
}
