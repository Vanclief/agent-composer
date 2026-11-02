package agents

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	"github.com/vanclief/agent-composer/models/agent"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
)

// Every is the reads pattern for every channel.
const Every = "*"

// validatePattern checks a reads entry: *, or a channel name that may hold
// glob characters (path.Match syntax).
func validatePattern(pattern string) error {
	if pattern == Every {
		return nil
	}

	if !strings.HasPrefix(pattern, "#") && !strings.HasPrefix(pattern, "@") {
		return ez.New(ez.EINVALID, fmt.Sprintf("reads entry %q must be *, or a channel or glob starting with # or @", pattern), nil)
	}

	_, err := path.Match(pattern, "")
	if err != nil {
		return ez.New(ez.EINVALID, fmt.Sprintf("reads entry %q is not a valid glob", pattern), err)
	}

	return nil
}

// isGlob reports whether pattern matches more than one channel.
func isGlob(pattern string) bool {
	return strings.ContainsAny(pattern, "*?[")
}

// MatchChannel reports whether channel matches pattern: * matches every
// channel, otherwise a glob on the whole name (* does not cross the / of a
// DM channel).
func MatchChannel(pattern string, channel string) bool {
	if pattern == Every {
		return true
	}

	if !isGlob(pattern) {
		return pattern == channel
	}

	matched, err := path.Match(pattern, channel)
	return err == nil && matched
}

// validChannel is messaging's channel name check, swapped in tests.
var validChannel = msgmodels.ValidateChannel

// Reads is every channel pattern the agent reads (ADR 0016 item 6): its
// DM channel, #<task> for each task it manages, then the template's
// reads. It is the one list behind GET /agent, the wake hook and the
// home's messaging access.
//
// A channel messaging rejects is left out: messaging only knows # names
// until direct-message channels land (ADR 0011), and one name it rejects
// fails the home loop's whole wait on every channel. The DM channel
// @<owner>/<name> joins the list once messaging accepts it.
func (api *API) Reads(ctx context.Context, record *agent.Agent, tmpl *Template) ([]string, error) {
	patterns := []string{}

	dm := DMChannel(record)
	if dm != "" {
		patterns = append(patterns, dm)
	}

	managed, err := api.managed(ctx, record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for i := range managed {
		patterns = append(patterns, messaging.TaskChannel(managed[i].Name))
	}

	if tmpl != nil {
		patterns = append(patterns, tmpl.Reads...)
	}

	return dedupe(knownChannels(patterns)), nil
}

// knownChannels drops every pattern that names one channel messaging
// rejects; globs and * stay, they only ever match channels that exist.
func knownChannels(patterns []string) []string {
	out := []string{}
	for _, pattern := range patterns {
		if pattern != Every && !isGlob(pattern) && validChannel(pattern) != nil {
			continue
		}

		out = append(out, pattern)
	}

	return out
}

// matchesAny reports whether channel matches one of patterns.
func matchesAny(patterns []string, channel string) bool {
	for _, pattern := range patterns {
		if MatchChannel(pattern, channel) {
			return true
		}
	}

	return false
}

// Channels is the agent's current channel list: Reads with every glob
// replaced by the channels that have messages and match it. A new channel
// a glob matches shows up on the next call.
func (api *API) Channels(ctx context.Context, record *agent.Agent, tmpl *Template) ([]string, error) {
	patterns, err := api.Reads(ctx, record, tmpl)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	channels := []string{}
	globs := []string{}
	for _, pattern := range patterns {
		if isGlob(pattern) {
			globs = append(globs, pattern)
			continue
		}

		channels = append(channels, pattern)
	}

	if len(globs) == 0 {
		return channels, nil
	}

	existing := []string{}

	err = api.db.NewSelect().
		Model((*msgmodels.Message)(nil)).
		ColumnExpr("DISTINCT channel").
		Order("channel ASC").
		Scan(ctx, &existing)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	for _, channel := range existing {
		if matchesAny(globs, channel) {
			channels = append(channels, channel)
		}
	}

	return dedupe(channels), nil
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		if seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}

	return out
}
