package knowledge

import (
	"fmt"
	"strings"

	"github.com/vanclief/ez"

	kb "github.com/vanclief/agent-composer/knowledge"
)

// RuleSentence is a rule an agent follows (ADR 0016 item 2): its id, its
// text as written, and the sentence agc rules show prints for it.
type RuleSentence struct {
	ID       string
	Rule     string
	Sentence string
}

// RuleSentences reads the knowledge directory now and returns the rules
// named by ids, in that order. An id that is not a rule fails, naming it
// and the known ids.
func (api *API) RuleSentences(ids []string) ([]RuleSentence, error) {
	if len(ids) == 0 {
		return []RuleSentence{}, nil
	}

	set, err := api.load()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	byID := map[string]kb.Rule{}
	known := []string{}
	for _, ruleset := range set.Rulesets {
		for _, rule := range ruleset.Rules {
			byID[rule.ID] = rule
			known = append(known, rule.ID)
		}
	}

	sentences := []RuleSentence{}
	for _, id := range ids {
		rule, found := byID[id]
		if !found {
			msg := fmt.Sprintf("rule %q is not in the knowledge directory %s; known rules: %s", id, api.dir, knownList(known))
			return nil, ez.New(ez.EINVALID, msg, nil)
		}

		line := set.RuleLine(rule, false)
		sentences = append(sentences, RuleSentence{ID: rule.ID, Rule: rule.Rule, Sentence: line.Text})
	}

	return sentences, nil
}

func knownList(ids []string) string {
	if len(ids) == 0 {
		return "none"
	}

	return strings.Join(ids, ", ")
}
