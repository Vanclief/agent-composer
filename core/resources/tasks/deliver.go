package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/messaging"
	msgmodels "github.com/vanclief/agent-composer/models/messaging"
	taskmodels "github.com/vanclief/agent-composer/models/task"
)

// deliveredPrefix starts the message a delivery posts in #<task>.
const deliveredPrefix = "Delivered: "

// reportCut ends a Delivered message cut to fit a message body.
const reportCut = "\n\n(cut: the full report is on the task)"

type DeliverRequest struct {
	ID      uuid.UUID `json:"id"`
	Name    string    `json:"name"`
	Summary string    `json:"summary"`
	// Report is optional: what the delivery says against each accept
	// criterion, the changes, the checks run, what was left out and what
	// the reviewer must decide (ADR 0010, "Delivery report").
	Report *taskmodels.Report `json:"report"`
	// Files holds the PNG of each of the report's shots under its label,
	// base64 in JSON.
	Files map[string][]byte `json:"files,omitempty"`
}

func (r DeliverRequest) Validate() error {
	err := validateRef(r.ID, r.Name)
	if err != nil {
		return ez.Wrap(err)
	}

	summary := strings.TrimSpace(r.Summary)
	if summary == "" {
		return ez.New(ez.EINVALID, "summary is required: say what was done and where the work is", nil)
	}

	limit := msgmodels.MaxBodyBytes - len(deliveredPrefix)
	if len(summary) > limit {
		return ez.New(ez.EINVALID, fmt.Sprintf("summary is longer than %d bytes", limit), nil)
	}

	if r.Report != nil {
		err = r.Report.Validate()
		if err != nil {
			return ez.Wrap(err)
		}
	}

	if len(r.Files) > taskmodels.MaxShots {
		return ez.New(ez.EINVALID, fmt.Sprintf("files has %d entries, at most %d", len(r.Files), taskmodels.MaxShots), nil)
	}

	return nil
}

// Deliver sets delivered_at, summary and report and posts "Delivered:
// <summary>" in #<task> as agc, followed by the report rendered as
// markdown when there is one (ADR 0010 item 2, "Delivery report"). The
// sandboxes are left as they are. A task can be delivered again; the last
// summary and report win. A report must carry one criterion per accept
// line. A done task cannot be delivered.
func (api *API) Deliver(ctx context.Context, requester interface{}, request *DeliverRequest) (*TaskView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	record, err := api.find(ctx, request.ID, request.Name)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	if record.DoneAt != nil {
		return nil, ez.New(ez.ECONFLICT, "task "+record.Name+" is done: it was accepted by "+orUnknown(record.AcceptedBy), nil)
	}

	summary := strings.TrimSpace(request.Summary)

	if request.Report != nil {
		err = request.Report.ValidateFor(record.Accept)
		if err != nil {
			return nil, ez.Wrap(err)
		}
	}

	report, err := api.storeShots(record.Name, request.Report, request.Files)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = record.MarkDelivered(ctx, api.db, summary, report)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	api.post(ctx, record, DeliveredMessage(record))

	return api.view(ctx, record)
}

// deliverFromSandbox is agc task deliver on a sandbox's socket, which
// speaks for its own task only. report is the request's report as sent,
// files the PNGs of its shots under their labels.
func (api *API) deliverFromSandbox(ctx context.Context, taskName string, summary string, report json.RawMessage, files map[string][]byte) (any, error) {
	decoded, err := DecodeReport(report)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.Deliver(ctx, nil, &DeliverRequest{Name: taskName, Summary: summary, Report: decoded, Files: files})
}

// DecodeReport reads a delivery report from JSON; absent or null is nil.
func DecodeReport(raw json.RawMessage) (*taskmodels.Report, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return nil, nil
	}

	report := &taskmodels.Report{}
	err := json.Unmarshal([]byte(trimmed), report)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "report is not a valid report object: "+err.Error(), err)
	}

	return report, nil
}

// DeliveredMessage is what a delivery posts in #<task>: "Delivered:
// <summary>", then the report, when there is one, as markdown: a checklist
// line per criterion, the changes, the checks, what was left out and what
// the reviewer is asked to decide, with a "<Label>: <url>" line per shot
// after the checks. Empty sections are left out. A message
// too long for a body is cut; the task keeps the whole report.
func DeliveredMessage(record *taskmodels.Task) string {
	body := deliveredPrefix + record.Summary
	if record.Report == nil {
		return body
	}

	rendered := renderReport(record.Report.Normalized(), record.Accept)
	if rendered != "" {
		body += "\n\n" + rendered
	}

	if len(body) > msgmodels.MaxBodyBytes {
		body = strings.ToValidUTF8(body[:msgmodels.MaxBodyBytes-len(reportCut)], "") + reportCut
	}

	return body
}

func renderReport(report taskmodels.Report, accept []string) string {
	sections := []string{}

	if len(report.Criteria) > 0 {
		lines := []string{}
		for i, criterion := range report.Criteria {
			box := "[ ]"
			if criterion.Met == taskmodels.MetYes {
				box = "[x]"
			}
			if criterion.Met == taskmodels.MetPartly {
				box = "[~]"
			}

			text := fmt.Sprintf("criterion %d", i+1)
			if i < len(accept) {
				text = accept[i]
			}

			lines = append(lines, "- "+box+" "+withNote(text, criterion.Note))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if report.Changes != nil {
		line := "Changes: " + report.Changes.Branch + " @ " + report.Changes.SHA
		if report.Changes.Files != nil {
			line += fmt.Sprintf(" (%d files)", *report.Changes.Files)
		}
		sections = append(sections, line)
	}

	if len(report.Checks) > 0 {
		lines := []string{"Checks:"}
		for _, check := range report.Checks {
			mark := "✗"
			if check.Passed {
				mark = "✓"
			}
			lines = append(lines, "- "+mark+" "+withNote(check.Name, check.Note))
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(report.Shots) > 0 {
		lines := []string{}
		for _, shot := range report.Shots {
			lines = append(lines, shotLabel(shot.Label)+": "+shot.URL)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(report.LeftOut) > 0 {
		lines := []string{"Left out:"}
		for _, left := range report.LeftOut {
			lines = append(lines, "- "+left.What+": "+left.Why)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	if len(report.Needs) > 0 {
		lines := []string{"Needs you:"}
		for _, need := range report.Needs {
			lines = append(lines, "- "+need)
		}
		sections = append(sections, strings.Join(lines, "\n"))
	}

	return strings.Join(sections, "\n\n")
}

// shotLabel is a shot's label as a line starts it: before is "Before".
func shotLabel(label string) string {
	if label == "" {
		return label
	}

	return strings.ToUpper(label[:1]) + label[1:]
}

func withNote(text string, note string) string {
	if note == "" {
		return text
	}

	return text + " — " + note
}

// post posts body in #<task> as agc. The change it reports is already
// stored, so a failure is logged, not returned.
func (api *API) post(ctx context.Context, record *taskmodels.Task, body string) {
	_, err := api.msg.PostAsSystem(ctx, messaging.TaskChannel(record.Name), body)
	if err != nil {
		log.Warn().Err(err).Str("task", record.Name).Msg("cannot post in the task's channel")
	}
}

func orUnknown(name string) string {
	if name == "" {
		return "someone unrecorded"
	}

	return name
}
