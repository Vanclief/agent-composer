package schedules

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"text/template"
	"text/template/parse"
	"time"

	"github.com/vanclief/ez"

	"github.com/vanclief/agent-composer/core/resources/tasks"
	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
)

const (
	// nowField is the tick's time, available in every template.
	nowField = "Now"
	// sampleValue fills every item field when a template is checked.
	sampleValue = "sample"
	maxTaskName = 63
)

// render executes text as a template over data. A field data does not
// have is an error, so a typo in a template does not become empty text.
func render(field string, text string, data map[string]any) (string, error) {
	parsed, err := parseTemplate(field, text)
	if err != nil {
		return "", err
	}

	var out strings.Builder
	err = parsed.Execute(&out, data)
	if err != nil {
		return "", ez.New(ez.EINVALID, fmt.Sprintf("task.%s: %s", field, err.Error()), err)
	}

	return out.String(), nil
}

func parseTemplate(field string, text string) (*template.Template, error) {
	parsed, err := template.New(field).Option("missingkey=error").Parse(text)
	if err != nil {
		return nil, ez.New(ez.EINVALID, fmt.Sprintf("task.%s is not a valid template: %s", field, err.Error()), err)
	}

	return parsed, nil
}

// renderTemplate renders the title, name, brief and each accept entry of
// t over data; the other fields are copied.
func renderTemplate(t schedulemodels.Template, data map[string]any) (schedulemodels.Template, error) {
	rendered := t
	fields := []struct {
		name string
		text *string
	}{
		{"title", &rendered.Title},
		{"name", &rendered.Name},
		{"brief", &rendered.Brief},
	}

	for _, field := range fields {
		out, err := render(field.name, *field.text, data)
		if err != nil {
			return rendered, err
		}
		*field.text = out
	}

	rendered.Accept = make([]string, 0, len(t.Accept))
	for i, criterion := range t.Accept {
		out, err := render(fmt.Sprintf("accept[%d]", i), criterion, data)
		if err != nil {
			return rendered, err
		}
		rendered.Accept = append(rendered.Accept, out)
	}

	return rendered, nil
}

// templateFields lists the top-level fields t reads, such as url for
// {{.url}}.
func templateFields(t schedulemodels.Template) ([]string, error) {
	texts := map[string]string{"title": t.Title, "name": t.Name, "brief": t.Brief}
	for i, criterion := range t.Accept {
		texts[fmt.Sprintf("accept[%d]", i)] = criterion
	}

	seen := map[string]bool{}
	fields := []string{}
	for field, text := range texts {
		parsed, err := parseTemplate(field, text)
		if err != nil {
			return nil, err
		}

		for _, name := range nodeFields(parsed.Root) {
			if !seen[name] {
				seen[name] = true
				fields = append(fields, name)
			}
		}
	}

	return fields, nil
}

// nodeFields walks a template tree and collects the first identifier of
// every .field and $.field it reads.
func nodeFields(node parse.Node) []string {
	fields := []string{}

	switch n := node.(type) {
	case nil:
		return fields
	case *parse.ListNode:
		if n == nil {
			return fields
		}
		for _, child := range n.Nodes {
			fields = append(fields, nodeFields(child)...)
		}
	case *parse.ActionNode:
		fields = append(fields, nodeFields(n.Pipe)...)
	case *parse.PipeNode:
		if n == nil {
			return fields
		}
		for _, command := range n.Cmds {
			fields = append(fields, nodeFields(command)...)
		}
	case *parse.CommandNode:
		for _, arg := range n.Args {
			fields = append(fields, nodeFields(arg)...)
		}
	case *parse.FieldNode:
		fields = append(fields, n.Ident[0])
	case *parse.VariableNode:
		if len(n.Ident) > 1 && n.Ident[0] == "$" {
			fields = append(fields, n.Ident[1])
		}
	case *parse.ChainNode:
		fields = append(fields, nodeFields(n.Node)...)
	case *parse.IfNode:
		fields = append(fields, branchFields(&n.BranchNode)...)
	case *parse.RangeNode:
		fields = append(fields, branchFields(&n.BranchNode)...)
	case *parse.WithNode:
		fields = append(fields, branchFields(&n.BranchNode)...)
	case *parse.TemplateNode:
		fields = append(fields, nodeFields(n.Pipe)...)
	}

	return fields
}

func branchFields(n *parse.BranchNode) []string {
	fields := nodeFields(n.Pipe)
	fields = append(fields, nodeFields(n.List)...)
	if n.ElseList != nil {
		fields = append(fields, nodeFields(n.ElseList)...)
	}

	return fields
}

// itemData is what a template sees for one item: its fields and Now,
// unless the item has its own Now.
func itemData(fields map[string]string, now time.Time) map[string]any {
	data := map[string]any{nowField: now}
	for name, value := range fields {
		data[name] = value
	}

	return data
}

// defaultName is the name a task gets when the template has none:
// <schedule>-<suffix>, with the schedule part cut short to fit 63
// characters.
func defaultName(schedule string, suffix string) string {
	room := maxTaskName - len(suffix) - 1
	prefix := schedule
	if len(prefix) > room {
		prefix = strings.TrimRight(prefix[:room], "-")
	}

	return prefix + "-" + suffix
}

// timeSuffix is a no-poll task's name suffix, yyyymmdd-hhmm in UTC.
func timeSuffix(now time.Time) string {
	return now.UTC().Format("20060102-1504")
}

// keySuffix is a poll item's name suffix: the first 8 hex digits of the
// key's SHA-256, so a key always gives the same name.
func keySuffix(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])[:8]
}

// createRequest is the POST /api/tasks body for a rendered template, with
// fallbackName when the template has no name.
func createRequest(t schedulemodels.Template, fallbackName string) *tasks.CreateRequest {
	name := strings.TrimSpace(t.Name)
	if name == "" {
		name = fallbackName
	}

	return &tasks.CreateRequest{
		Title:    t.Title,
		Name:     name,
		Brief:    t.Brief,
		Accept:   t.Accept,
		Assignee: t.Assignee,
		Model:    t.Model,
		Manager:  t.Manager,
		Owner:    t.Owner,
		For:      t.For,
		Sandbox:  t.Sandbox,
	}
}

// checkTemplate renders the schedule's template with every item field it
// reads set to "sample" (with a poll), and checks the task it makes passes the same
// defaults and validation as POST /api/tasks.
func (api *API) checkTemplate(record *schedulemodels.Schedule) error {
	fields, err := templateFields(record.Task)
	if err != nil {
		return err
	}

	// Without a poll a template sees only Now.
	now := api.now()
	sample := map[string]string{}
	if record.Poll != nil {
		sample[keyField] = sampleValue
		for _, name := range fields {
			if name != nowField {
				sample[name] = sampleValue
			}
		}
	}

	rendered, err := renderTemplate(record.Task, itemData(sample, now))
	if err != nil {
		return err
	}

	fallback := defaultName(record.Name, timeSuffix(now))
	if record.Poll != nil {
		fallback = defaultName(record.Name, keySuffix(sampleValue))
	}

	request := createRequest(rendered, fallback)

	_, err = tasks.ApplyDefaults(request, api.tasks.Defaults())
	if err != nil {
		return ez.New(ez.EINVALID, "task: "+ez.ErrorMessage(err), err)
	}

	err = request.Validate()
	if err != nil {
		return ez.New(ez.EINVALID, "task: "+ez.ErrorMessage(err), err)
	}

	return nil
}
