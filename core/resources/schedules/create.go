package schedules

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/vanclief/ez"
	"gopkg.in/yaml.v3"

	schedulemodels "github.com/vanclief/agent-composer/models/schedule"
	"github.com/vanclief/agent-composer/secrets"
)

// CreateRequest is a schedule, as POST /api/schedules takes it and as a
// schedule.yaml is written. Enabled defaults to true.
type CreateRequest struct {
	Name    string                  `yaml:"name" json:"name"`
	Every   string                  `yaml:"every" json:"every"`
	Enabled *bool                   `yaml:"enabled" json:"enabled"`
	Task    schedulemodels.Template `yaml:"task" json:"task"`
	Poll    *schedulemodels.Poll    `yaml:"poll" json:"poll"`
}

func (r CreateRequest) Validate() error {
	err := schedulemodels.ValidateName(strings.TrimSpace(r.Name))
	if err != nil {
		return ez.Wrap(err)
	}

	return validateBody(r.Every, r.Poll)
}

// validateBody checks what create and update share apart from the
// template, which needs the configured defaults.
func validateBody(every string, poll *schedulemodels.Poll) error {
	if strings.TrimSpace(every) == "" {
		return ez.New(ez.EINVALID, "every is required: a duration such as 2m or 1h", nil)
	}

	_, err := schedulemodels.ParseEvery(strings.TrimSpace(every))
	if err != nil {
		return ez.Wrap(err)
	}

	if poll == nil {
		return nil
	}

	if strings.TrimSpace(poll.Command) == "" {
		return ez.New(ez.EINVALID, "poll.command is required when poll is set", nil)
	}

	for _, name := range poll.Secrets {
		request, err := secrets.ParseRequest(name)
		if err != nil {
			return ez.New(ez.EINVALID, "poll.secrets: "+ez.ErrorMessage(err), err)
		}

		if request.Env != request.Name {
			return ez.New(ez.EINVALID, fmt.Sprintf("poll.secrets: %q renames the secret, but a poll command reads it by its own name: write %s", name, request.Name), nil)
		}

		if len(request.Hosts) > 0 {
			return ez.New(ez.EINVALID, fmt.Sprintf("poll.secrets: %s names hosts, but a poll command runs on the host and gets the value itself: write %s", name, request.Name), nil)
		}
	}

	return nil
}

// LoadFile reads a schedule.yaml.
func LoadFile(filePath string) (*CreateRequest, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "cannot read schedule file "+filePath, err)
	}

	parsed, err := ParseFile(data)
	if err != nil {
		return nil, ez.New(ez.EINVALID, "schedule file "+filePath+": "+ez.ErrorMessage(err), err)
	}

	return parsed, nil
}

// ParseFile parses a schedule.yaml. Unknown keys are rejected so a typo
// does not silently fall back to a default.
func ParseFile(data []byte) (*CreateRequest, error) {
	parsed := &CreateRequest{}

	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	err := decoder.Decode(parsed)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, ez.New(ez.EINVALID, err.Error(), err)
	}

	return parsed, nil
}

// Create stores the schedule. It does not run it: the scheduler of
// agc rest runs it on its next check, as it is due at once.
func (api *API) Create(ctx context.Context, requester interface{}, request *CreateRequest) (*ScheduleView, error) {
	err := request.Validate()
	if err != nil {
		return nil, ez.Wrap(err)
	}

	name := strings.TrimSpace(request.Name)

	_, err = schedulemodels.GetByName(ctx, api.db, name)
	if err == nil {
		return nil, ez.New(ez.ECONFLICT, "schedule "+name+" already exists", nil)
	}
	if ez.ErrorCode(err) != ez.ENOTFOUND {
		return nil, ez.Wrap(err)
	}

	// Stamped with the scheduler's clock, not the model's: a schedule that
	// never ran is due at created_at, and RunDue compares against api.now.
	record := &schedulemodels.Schedule{
		Name:      name,
		Every:     strings.TrimSpace(request.Every),
		Enabled:   request.Enabled == nil || *request.Enabled,
		Task:      request.Task,
		Poll:      cleanPoll(request.Poll),
		CreatedAt: api.now().UTC().Truncate(time.Microsecond),
	}

	err = api.checkTemplate(record)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	err = record.Insert(ctx, api.db)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	return api.view(record), nil
}

// cleanPoll trims the command and secret names and keeps secrets an
// array.
func cleanPoll(poll *schedulemodels.Poll) *schedulemodels.Poll {
	if poll == nil {
		return nil
	}

	names := make([]string, 0, len(poll.Secrets))
	for _, name := range poll.Secrets {
		names = append(names, strings.TrimSpace(name))
	}

	return &schedulemodels.Poll{Command: strings.TrimSpace(poll.Command), Secrets: names}
}
