package knowledge

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Sentences: each belief, decision, action, rule and piece of evidence
// printed as a thought a person would have, built from the texts in the
// files, with the ids behind it in a trailer for editing.

// Line is one sentence, indented by Depth, and the ids behind it.
type Line struct {
	Depth   int
	Text    string
	Trailer string
}

// BeliefLine says what is held about the belief id now: believed,
// only thought, doubted or contested, with every reason.
func (s *Set) BeliefLine(id string) (Line, error) {
	status, err := s.Status(id)
	if err != nil {
		return Line{}, err
	}

	belief, _ := s.belief(id)
	text := s.embed(belief.Belief)
	sentence := ""
	switch {
	case slices.Contains(status.Without, id):
		sentence = "I am setting aside that " + text
	case status.Label == InDoubt:
		sentence = s.doubtClause(belief, text, status)
		if len(status.Opinions) > 0 {
			sentence += ", and " + holder(belief, "have") + " " + s.noEvidence(id, status.Opinions)
		}
	case len(status.Opinions) > 0:
		sentence = holder(belief, "think") + " " + text + ", but " + conjugate(belief, "have") + " " + s.noEvidence(id, status.Opinions)
	default:
		sentence = holder(belief, "believe") + " " + text
		if slices.Contains(status.Judgments, id) {
			sentence += fromExperience(belief)
		}
	}

	sentence += s.contested(status)

	return Line{Text: sentence + ".", Trailer: statusTrailer(id, status)}, nil
}

// holder says who holds a belief, followed by verb as that holder says
// it: "I believe" when By is empty, the repo owner, or "ana believes".
func holder(belief Belief, verb string) string {
	if belief.By == "" {
		return "I " + verb
	}

	return belief.By + " " + conjugate(belief, verb)
}

// conjugate says verb as the holder of a belief says it: as is for I,
// in the third person for anyone named in By.
func conjugate(belief Belief, verb string) string {
	if belief.By == "" {
		return verb
	}

	switch verb {
	case "have":
		return "has"
	case "believed":
		return verb
	}

	return verb + "s"
}

// fromExperience says whose experience a judgment comes from, and in what.
func fromExperience(belief Belief) string {
	if belief.Experience == nil {
		return ""
	}

	return ", from " + belief.Experience.Of + "'s experience in " + trimPeriod(fold(belief.Experience.In))
}

// contested says what contests a belief, if anything does.
func (s *Set) contested(status Status) string {
	if len(status.ContestedBy) == 0 {
		return ""
	}

	contesters := []string{}
	for _, id := range status.ContestedBy {
		evidence, isEvidence := s.evidenceByID[id]
		if isEvidence {
			contesters = append(contesters, s.seenWhat(evidence))
			continue
		}
		contesters = append(contesters, "my belief that "+s.Phrase(id))
	}

	joiner := ", though"
	if len(status.Opinions) > 0 {
		joiner = ", and"
	}

	return joiner + " it is contested by " + joinAnd(contesters)
}

// seenWhat says a piece of evidence as a thing: what someone saw.
func (s *Set) seenWhat(evidence Evidence) string {
	return "what " + evidence.Who + " saw on " + evidence.When + ", that " + s.embed(evidence.Evidence)
}

// doubtClause says why a belief is in doubt: the evidence retracted under
// it, and what --without set aside.
func (s *Set) doubtClause(belief Belief, text string, status Status) string {
	retracted := []string{}
	for _, evidenceID := range status.Retracted {
		evidence := s.evidenceByID[evidenceID]
		clause := fmt.Sprintf("%s was retracted on %s: %s", s.embed(evidence.Evidence), evidence.Retracted.When, trimPeriod(fold(evidence.Retracted.Why)))
		retracted = append(retracted, clause)
	}

	setAside := []string{}
	for _, id := range status.Without {
		evidence, isEvidence := s.evidenceByID[id]
		if isEvidence {
			setAside = append(setAside, "what "+evidence.Who+" saw on "+evidence.When)
			continue
		}
		setAside = append(setAside, "the belief that "+s.Phrase(id))
	}

	if len(retracted) == 0 {
		separator := " "
		if strings.Contains(text, ",") {
			separator = ", "
		}
		return "I would doubt that " + text + separator + "without " + joinAnd(setAside)
	}

	sentence := holder(belief, "believed") + " " + text + ", but " + joinAnd(retracted)
	if len(setAside) > 0 {
		sentence += ", and without " + joinAnd(setAside)
	}

	return sentence
}

// noEvidence says which opinions a belief rests on, after "have". A belief
// that is the opinion itself has no evidence yet.
func (s *Set) noEvidence(id string, opinions []string) string {
	others := []string{}
	for _, opinion := range opinions {
		if opinion != id {
			others = append(others, s.Phrase(opinion))
		}
	}

	if len(others) == 0 {
		return "no evidence yet"
	}

	return "no evidence yet that " + joinAnd(others)
}

// statusTrailer names the label and the ids behind it, leaving out the
// belief itself.
func statusTrailer(id string, status Status) string {
	parts := []string{}
	add := func(name string, ids []string) {
		others := []string{}
		for _, other := range ids {
			if other != id {
				others = append(others, other)
			}
		}
		if len(others) > 0 {
			parts = append(parts, name+" "+strings.Join(others, ", "))
		}
	}
	add("retracted", status.Retracted)
	add("without", status.Without)
	add("doubted", status.Doubted)
	add("opinion", status.Opinions)
	add("judgment", status.Judgments)
	add("contested by", status.ContestedBy)

	label := string(status.Label)
	if len(parts) > 0 {
		label += ": " + strings.Join(parts, " · ")
	}

	return "(" + id + ", " + label + ")"
}

// WhyLines follows a belief down to its evidence: one sentence for the
// belief, one per alternative, then the same for each belief it rests on,
// indented under it.
func (s *Set) WhyLines(reason Reason, depth int) []Line {
	belief := reason.Belief
	status, _ := s.Status(belief.ID)
	text := s.embed(belief.Belief)

	verb := "believe"
	switch {
	case status.Label == InDoubt:
		verb = "believed"
	case len(status.Opinions) > 0:
		verb = "think"
	}

	main := Alternative{Evidence: reason.Evidence, Because: reason.Because}
	leaf := len(main.Evidence) == 0 && len(main.Because) == 0
	sentence := holder(belief, verb) + " " + text + because(text) + s.reasons(main)
	leafLabel := "opinion"
	switch {
	case leaf && belief.Experience != nil:
		sentence = holder(belief, "believe") + " " + text + fromExperience(belief)
		leafLabel = "judgment"
	case leaf:
		sentence = holder(belief, "think") + " " + text + ", but " + conjugate(belief, "have") + " no evidence yet"
	}
	sentence += s.contested(status)

	lines := []Line{{Depth: depth, Text: sentence + "."}}
	for _, alternative := range reason.Or {
		lines = append(lines, Line{Depth: depth, Text: "Or because " + s.reasons(alternative) + "."})
	}

	sets := [][]string{whyIDs(main)}
	for _, alternative := range reason.Or {
		sets = append(sets, whyIDs(alternative))
	}
	trailer := "(" + belief.ID + ", " + leafLabel + ")"
	if len(sets[0]) > 0 {
		trailer = "(" + belief.ID + " ← " + joinSets(sets) + ")"
	}
	lines[len(lines)-1].Trailer = trailer

	for _, child := range main.Because {
		lines = append(lines, s.WhyLines(child, depth+1)...)
	}
	for _, alternative := range reason.Or {
		for _, child := range alternative.Because {
			lines = append(lines, s.WhyLines(child, depth+1)...)
		}
	}

	return lines
}

// reasons says one set of reasons: the beliefs it rests on and the
// evidence it cites. When every reason is a belief standing directly on
// live evidence, who saw it and when follows.
func (s *Set) reasons(alternative Alternative) string {
	parts := []string{}
	for _, child := range alternative.Because {
		parts = append(parts, s.embed(child.Belief.Belief))
	}
	for _, evidence := range alternative.Evidence {
		parts = append(parts, s.sawThat(evidence))
	}

	return joinAnd(parts) + s.seenBy(alternative)
}

// seenBy says who saw what grounds a set of beliefs that each cite live
// evidence directly, merging identical who and when.
func (s *Set) seenBy(alternative Alternative) string {
	if len(alternative.Because) == 0 || len(alternative.Evidence) > 0 {
		return ""
	}

	seen := []string{}
	for _, child := range alternative.Because {
		live := []string{}
		for _, evidence := range child.Evidence {
			if s.live(evidence.ID) {
				live = appendNew(live, evidence.Who+" on "+evidence.When)
			}
		}
		if len(child.Because) > 0 || len(child.Or) > 0 || len(live) == 0 {
			return ""
		}
		for _, observation := range live {
			seen = appendNew(seen, observation)
		}
	}

	qualifier := ""
	switch {
	case len(alternative.Because) == 2:
		qualifier = "both "
	case len(alternative.Because) > 2:
		qualifier = "all "
	}

	return ", " + qualifier + "seen by " + joinAnd(seen)
}

// sawThat says a piece of evidence as a reason, with its retraction.
func (s *Set) sawThat(evidence Evidence) string {
	clause := fmt.Sprintf("%s saw on %s that %s", evidence.Who, evidence.When, s.embed(evidence.Evidence)) + writtenUpBy(evidence)
	if evidence.Retracted != nil {
		clause += fmt.Sprintf(", retracted on %s: %s", evidence.Retracted.When, trimPeriod(fold(evidence.Retracted.Why)))
	}
	if s.without[evidence.ID] {
		clause += ", set aside"
	}

	return clause
}

// writtenUpBy names the model that translated a source into the record,
// if one did.
func writtenUpBy(evidence Evidence) string {
	if evidence.TranslatedBy == "" {
		return ""
	}

	return ", written up by " + evidence.TranslatedBy
}

func whyIDs(alternative Alternative) []string {
	ids := []string{}
	for _, child := range alternative.Because {
		ids = append(ids, child.Belief.ID)
	}
	for _, evidence := range alternative.Evidence {
		ids = append(ids, evidence.ID)
	}

	return ids
}

// joinSets lists sets of ids, alternatives after a bar.
func joinSets(sets [][]string) string {
	joined := []string{}
	for _, set := range sets {
		joined = append(joined, strings.Join(set, ", "))
	}

	return strings.Join(joined, " | ")
}

func supportIDs(sets []Support) [][]string {
	ids := [][]string{}
	for _, set := range sets {
		ids = append(ids, append(append([]string{}, set.Because...), set.Evidence...))
	}

	return ids
}

// DecisionLine says a decision, its reasons and when it was made, and
// what under it is only an opinion or now in doubt.
func (s *Set) DecisionLine(decision Decision) Line {
	sets := decision.Supports()
	text := s.embed(decision.Decision)
	sentence := "I decided that " + text + because(text) + s.supportPhrase(sets, s.setPhrase)
	sentence += ", on " + decision.Decided.When + s.restsOn(s.StatusOf(sets)) + "."

	trailer := "(" + decision.ID + " ← " + joinSets(supportIDs(sets)) + " · decided by " + decision.Decided.Who
	if decision.ADR != "" {
		trailer += " · " + decision.ADR
	}

	return Line{Text: sentence, Trailer: trailer + ")"}
}

// ActionLine says what will be done and the decision it serves, or what
// was done and when.
func (s *Set) ActionLine(action Action) Line {
	sets := action.Supports()
	trailer := "(" + action.ID + " ← " + joinSets(supportIDs(sets))
	if action.Done != nil {
		sentence := "I did " + s.embed(action.Action) + " on " + action.Done.When + "."
		return Line{Text: sentence, Trailer: trailer + " · done by " + action.Done.Who + ")"}
	}

	sentence := "I will " + s.embed(action.Action) + ", because " + s.supportPhrase(sets, s.servesPhrase)
	sentence += s.restsOn(s.StatusOf(sets)) + "."
	trailer += " · opened by " + action.Opened.Who + " on " + action.Opened.When

	return Line{Text: sentence, Trailer: trailer + ")"}
}

// servesPhrase says the decisions and beliefs an action rests on as what
// was decided and what is believed.
func (s *Set) servesPhrase(set Support) string {
	decided := []string{}
	believed := []string{}
	for _, id := range set.Because {
		if s.nodes[id].kind == "decision" {
			decided = append(decided, s.Phrase(id))
			continue
		}
		believed = append(believed, s.Phrase(id))
	}

	parts := []string{}
	if len(decided) > 0 {
		parts = append(parts, "I decided that "+strings.Join(decided, ", and that "))
	}
	if len(believed) > 0 {
		parts = append(parts, "I believe that "+strings.Join(believed, ", and that "))
	}

	return strings.Join(parts, ", and ")
}

// RuleLine says a rule as written, with the reasons it rests on and what
// under them is only an opinion or now in doubt. The check mark is the
// caller's.
func (s *Set) RuleLine(rule Rule, withCheck bool) Line {
	sets := rule.Supports()
	sentence := fold(rule.Rule)
	if len(rule.Because) > 0 {
		sentence = trimPeriod(sentence)
		sentence += because(sentence) + s.supportPhrase(sets, s.setPhrase) + s.restsOn(s.StatusOf(sets)) + "."
	}

	trailer := "(" + rule.ID
	if withCheck && rule.Check != "" {
		trailer += " · check: " + rule.Check
	}

	return Line{Text: sentence, Trailer: trailer + ")"}
}

// EvidenceLine says who saw what and when, and its retraction.
func (s *Set) EvidenceLine(evidence Evidence) Line {
	sentence := fmt.Sprintf("%s saw that %s on %s", evidence.Who, s.embed(evidence.Evidence), evidence.When) + writtenUpBy(evidence)
	if evidence.Retracted != nil {
		sentence += fmt.Sprintf(", retracted on %s: %s", evidence.Retracted.When, trimPeriod(fold(evidence.Retracted.Why)))
	}

	trailer := "(" + evidence.ID
	if evidence.Source != "" {
		trailer += " · source: " + evidence.Source
	}
	cited := len(s.Citing(evidence.ID))
	noun := "beliefs"
	if cited == 1 {
		noun = "belief"
	}
	trailer += " · cited by " + strconv.Itoa(cited) + " " + noun + ")"

	return Line{Text: sentence + ".", Trailer: trailer}
}

// DoubtHeader says what a doubt listing is about.
func (s *Set) DoubtHeader(id string) string {
	evidence, isEvidence := s.evidenceByID[id]
	if isEvidence {
		return fmt.Sprintf("If what %s saw on %s is wrong, that %s, recheck what rests on it.", evidence.Who, evidence.When, s.embed(evidence.Evidence))
	}

	return "If it is wrong that " + s.Phrase(id) + ", recheck what rests on it."
}

// ConflictLines says both sides of a contradiction and what each comes
// down to, so a human can see what to retract. It picks no side.
func (s *Set) ConflictLines(conflict Conflict) []Line {
	lines := []Line{{Text: s.claim(conflict.A.ID) + ", but " + s.claim(conflict.B.ID) + "."}}
	for i, side := range []Side{conflict.A, conflict.B} {
		if side.Kind == "evidence" {
			// The first line already says what was seen.
			continue
		}

		leaves := []string{}
		for _, evidence := range side.Evidence {
			leaves = append(leaves, s.seenWhat(evidence))
		}
		for _, opinion := range side.Opinions {
			leaves = append(leaves, "my opinion that "+s.Phrase(opinion))
		}
		if len(leaves) == 0 {
			leaves = []string{"nothing live"}
		}

		which := "The first"
		if i == 1 {
			which = "The second"
		}
		lines = append(lines, Line{Depth: 1, Text: which + " rests on " + joinAnd(leaves) + "."})
	}
	lines[len(lines)-1].Trailer = "(" + conflict.A.ID + " ↔ " + conflict.B.ID + ")"

	return lines
}

// StandingLine says what rests on a doubted id in some sets of its reasons
// but still stands on another.
func (s *Set) StandingLine(standing Standing) Line {
	subject := trimPeriod(fold(standing.Text))
	switch standing.Kind {
	case "belief":
		subject = "I believe " + s.embed(standing.Text)
	case "decision":
		subject = "I decided that " + s.embed(standing.Text)
	case "action":
		subject = "I will " + s.embed(standing.Text)
	}

	sentence := subject + ", which still stands because " + s.supportPhrase(standing.Other, s.setPhrase) + "."

	return Line{Text: sentence, Trailer: "(" + standing.ID + " ← " + joinSets(supportIDs(standing.Other)) + ")"}
}

// claim says a belief as held now, or a piece of evidence as seen.
func (s *Set) claim(id string) string {
	_, isEvidence := s.evidenceByID[id]
	if isEvidence {
		return s.Phrase(id)
	}

	belief, _ := s.belief(id)
	status := s.judge(id)
	switch {
	case status.Label == InDoubt:
		return holder(belief, "believed") + " " + s.Phrase(id)
	case len(status.Opinions) > 0:
		return holder(belief, "think") + " " + s.Phrase(id)
	}

	return holder(belief, "believe") + " " + s.Phrase(id)
}

// supportPhrase says every set of reasons, alternatives after "or
// because".
func (s *Set) supportPhrase(sets []Support, phrase func(Support) string) string {
	parts := []string{}
	for _, set := range sets {
		parts = append(parts, phrase(set))
	}

	return strings.Join(parts, ", or because ")
}

// setPhrase says one set of reasons.
func (s *Set) setPhrase(set Support) string {
	return joinAnd(s.phrases(append(append([]string{}, set.Because...), set.Evidence...)))
}

// restsOn says which opinions and doubted beliefs something rests on.
func (s *Set) restsOn(status Status) string {
	clauses := []string{}
	if len(status.Opinions) > 0 {
		clauses = append(clauses, "I only think that "+joinAnd(s.phrases(status.Opinions)))
	}
	if len(status.Doubted) > 0 {
		clauses = append(clauses, "I now doubt that "+joinAnd(s.phrases(status.Doubted)))
	}
	if len(clauses) == 0 {
		return ""
	}

	return " — though " + strings.Join(clauses, ", and ")
}

// Phrase says a belief or decision as a clause inside a sentence, or a
// piece of evidence as who saw it, when, and what.
func (s *Set) Phrase(id string) string {
	evidence, isEvidence := s.evidenceByID[id]
	if isEvidence {
		return evidence.Who + " saw on " + evidence.When + " that " + s.embed(evidence.Evidence)
	}

	return s.embed(s.text(id))
}

func (s *Set) phrases(ids []string) []string {
	phrases := []string{}
	for _, id := range ids {
		phrases = append(phrases, s.Phrase(id))
	}

	return phrases
}

// text is the text of a belief or a decision.
func (s *Set) text(id string) string {
	for _, belief := range s.Beliefs {
		if belief.ID == id {
			return belief.Belief
		}
	}
	for _, decision := range s.Decisions {
		if decision.ID == id {
			return decision.Decision
		}
	}

	return id
}

// embed makes a text a clause: one line, no closing period, and its first
// letter lower case unless the first word is a name: a word written
// capitalized mid-sentence somewhere, one with an inner capital, or a
// single letter other than the article A.
func (s *Set) embed(text string) string {
	text = trimPeriod(fold(text))
	if text == "" {
		return text
	}

	first, _, _ := strings.Cut(text, " ")
	key := properKey(first)
	letter := utf8.RuneCountInString(key) == 1 && key != "A"
	if s.proper[key] || hasInnerUpper(first) || letter {
		return text
	}

	r, size := utf8.DecodeRuneInString(text)

	return string(unicode.ToLower(r)) + text[size:]
}

// properWords lists the words written capitalized in the middle of a
// sentence somewhere in texts, such as Go or Task: names, which keep their
// capital when a text is embedded.
func properWords(texts []string) map[string]bool {
	proper := map[string]bool{}
	for _, text := range texts {
		words := strings.Fields(text)
		for i, word := range words {
			if i == 0 || strings.ContainsAny(words[i-1][len(words[i-1])-1:], ".!?") {
				continue
			}
			r, _ := utf8.DecodeRuneInString(word)
			if unicode.IsUpper(r) {
				proper[properKey(word)] = true
			}
		}
	}

	return proper
}

// properKey is a word without punctuation around it or a possessive 's.
func properKey(word string) string {
	word = strings.TrimFunc(word, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	word = strings.TrimSuffix(word, "'s")

	return strings.TrimSuffix(word, "’s")
}

// hasInnerUpper says whether a word has a capital after its first letter,
// as in SQLite or SSH.
func hasInnerUpper(word string) bool {
	_, size := utf8.DecodeRuneInString(word)

	return strings.ContainsFunc(word[size:], unicode.IsUpper)
}

// fold puts a YAML block of text on one line.
func fold(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func trimPeriod(text string) string {
	return strings.TrimSuffix(text, ".")
}

// joinAnd joins items as a person would: "a", "a and b", "a, b and c".
// When an item has a comma of its own, the last one gets a comma too, so
// "a, x and b" cannot be read as three items.
func joinAnd(items []string) string {
	if len(items) < 2 {
		return strings.Join(items, "")
	}

	last := " and "
	if strings.Contains(strings.Join(items, ""), ",") {
		last = ", and "
	}

	return strings.Join(items[:len(items)-1], ", ") + last + items[len(items)-1]
}

// because starts the reasons after a text, behind a comma when the text
// has one, so the reasons do not read as part of its last clause.
func because(text string) string {
	if strings.Contains(text, ",") {
		return ", because "
	}

	return " because "
}

// proposed says a proposal as a thing: the rule that …, the action to ….
// Action and code texts are imperative, so they take "to".
func (s *Set) proposed(proposal Proposal) string {
	text := s.embed(proposal.Proposal)
	switch proposal.Proposes {
	case "action":
		return "the action to " + text
	case "code":
		return "the code change to " + text
	case "gate":
		return "the gate change to " + text
	case "task":
		return "the task act to " + text
	}

	return "the " + proposal.Proposes + " that " + text
}

// ProposalLine says what someone proposes, what it rests on and what it
// needs before it lands, or what I did with it and why.
func (s *Set) ProposalLine(review Review) Line {
	proposal := review.Proposal
	what := s.proposed(proposal)
	trailer := proposalTrailer(review)

	if proposal.Accepted != nil {
		sentence := "I accepted " + what + ", proposed by " + proposal.By + ", on " + proposal.Accepted.When + "."
		return Line{Text: sentence, Trailer: trailer}
	}
	if proposal.Rejected != nil {
		sentence := "I rejected " + what + ", proposed by " + proposal.By + ", on " + proposal.Rejected.When + ": " + trimPeriod(fold(proposal.Rejected.Why)) + "."
		return Line{Text: sentence, Trailer: trailer}
	}

	sentence := proposal.By + " proposes " + what
	support := proposal.Support()
	if len(support.Because) > 0 || len(support.Evidence) > 0 {
		sentence += because(what) + s.setPhrase(support)
	}

	// The proposal itself is the opinion when it rests on nothing; what
	// restsOn says is about the beliefs under it.
	status := review.Status
	status.Opinions = slices.DeleteFunc(slices.Clone(status.Opinions), func(id string) bool { return id == proposal.ID })
	if len(status.Opinions) < len(review.Status.Opinions) {
		sentence += ", with no evidence under it"
	}
	sentence += s.restsOn(status)

	switch {
	case review.Illegible:
		sentence += ", but it rests on nothing I hold, so there is no why to read, and it needs my decision"
	case len(review.TightenedBy) > 0:
		sentence += ", and it needs my decision, since I rejected " + count(len(review.TightenedBy), "one like it", "like it")
	case review.Audited:
		sentence += ", and it is audited, so it needs my decision"
	case review.Tier == Decide:
		sentence += ", and it needs my decision"
	case review.Tier == Observe:
		sentence += ", and it lands for me to see"
	default:
		sentence += ", and it lands"
	}

	return Line{Text: sentence + ".", Trailer: trailer}
}

// proposalTrailer names the ids behind a proposal: what it rests on, the
// gate and tier, who proposed it and what became of it.
func proposalTrailer(review Review) string {
	proposal := review.Proposal
	trailer := "(" + proposal.ID
	ids := append(append([]string{}, proposal.Because...), proposal.Evidence...)
	if len(ids) > 0 {
		trailer += " ← " + strings.Join(ids, ", ")
	}

	tier := string(review.Tier)
	switch {
	case review.Gate == "":
		tier += " by default"
	case review.Tier != review.Written:
		tier += " via " + review.Gate + ", written " + string(review.Written)
	default:
		tier += " via " + review.Gate
	}
	if len(review.TightenedBy) > 0 {
		tier += " · tightened by " + strings.Join(review.TightenedBy, ", ")
	}
	trailer += " · " + tier + " · proposed by " + proposal.By + " on " + proposal.When

	switch {
	case proposal.Accepted != nil:
		trailer += " · accepted by " + proposal.Accepted.Who
	case proposal.Rejected != nil:
		trailer += " · rejected by " + proposal.Rejected.Who
	}
	if proposal.Became != "" {
		trailer += " · now " + proposal.Became
	}
	if proposal.Change != "" {
		trailer += " · change: " + proposal.Change
	}

	return trailer + ")"
}

// count says n things: "one like it", "3 like it".
func count(n int, one string, many string) string {
	if n == 1 {
		return one
	}

	return strconv.Itoa(n) + " " + many
}

// plural says a proposable kind as a plural noun.
func plural(kind string) string {
	switch kind {
	case "evidence":
		return "evidence"
	case "code":
		return "code changes"
	case "task":
		return "task acts"
	}

	return kind + "s"
}

// GateLine says who a gate covers, what their proposals need, and how
// the proposals under it have gone: the counts are the health signal, and
// a decide gate that has only ever accepted says so.
func (s *Set) GateLine(report GateReport) Line {
	gate := report.Gate
	tally := " So far " + strconv.Itoa(report.Accepted) + " accepted, " + strconv.Itoa(report.Rejected) + " rejected and " + strconv.Itoa(report.Open) + " open"
	if report.Audited > 0 {
		tally += ", " + strconv.Itoa(report.Audited) + " audited"
	}
	tally += "."

	if gate.ID == "" {
		return Line{Text: "Anything no gate covers needs my decision." + tally, Trailer: "(default · decide)"}
	}

	who := "anyone"
	if len(gate.By) > 0 {
		who = joinOr(gate.By)
	}
	sentence := capitalize(plural(gate.Proposes)) + " proposed by " + who
	switch gate.Tier {
	case Decide:
		sentence += " need my decision before they land"
	case Observe:
		sentence += " land for me to see"
	default:
		sentence += " land without me"
	}
	if gate.Since != "" {
		sentence += " since " + gate.Since
	}
	if gate.Tier == Observe && gate.Sample > 0 {
		sentence += ", about one in " + strconv.Itoa(gate.Sample) + " audited"
	}
	if len(report.TightenedBy) > 0 {
		sentence += ", but need my decision now, since I rejected " + count(len(report.TightenedBy), "one of them", "of them")
	}
	if len(gate.Because) > 0 {
		sentence += because(sentence) + s.setPhrase(Support{Because: gate.Because})
	}
	sentence += s.restsOn(report.Status) + "." + tally
	if report.AllAccepted {
		sentence += " I have accepted every one: either they are perfect or I have stopped deciding, so move this gate to observe honestly, or audit it."
	}

	trailer := "(" + gate.ID + " · " + gate.Proposes + " by " + strings.Join(append([]string{}, gate.By...), ", ")
	if len(gate.By) == 0 {
		trailer = "(" + gate.ID + " · " + gate.Proposes + " by anyone"
	}
	trailer += " · " + string(gate.Tier)
	if gate.Sample > 0 {
		trailer += " · sample 1/" + strconv.Itoa(gate.Sample)
	}
	if gate.Since != "" {
		trailer += " · since " + gate.Since
	}
	if len(report.TightenedBy) > 0 {
		trailer += " · tightened by " + strings.Join(report.TightenedBy, ", ")
	}
	if len(gate.Because) > 0 {
		trailer += " ← " + strings.Join(gate.Because, ", ")
	}

	return Line{Text: sentence, Trailer: trailer + ")"}
}

// capitalize upper-cases the first letter of a text.
func capitalize(text string) string {
	r, size := utf8.DecodeRuneInString(text)
	if size == 0 {
		return text
	}

	return string(unicode.ToUpper(r)) + text[size:]
}
