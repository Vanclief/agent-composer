package messaging

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// MaxBodyBytes caps a message body at 64 KiB.
const MaxBodyBytes = 64 * 1024

var channelRegex = regexp.MustCompile(`^#[a-z0-9][a-z0-9_-]{0,63}$`)

// dmRegex is a direct message channel, @<human>/<task> (ADR 0011).
var dmRegex = regexp.MustCompile(`^@([a-z0-9][a-z0-9_-]{0,63})/([a-z0-9][a-z0-9_-]{0,63})$`)

// bareDMRegex is @<human> alone, which a task's calls expand to its own DM.
var bareDMRegex = regexp.MustCompile(`^@([a-z0-9][a-z0-9_-]{0,63})$`)

// ParseDM splits a direct message channel @<human>/<task> into its two
// names. ok is false for any other channel.
func ParseDM(channel string) (human, task string, ok bool) {
	match := dmRegex.FindStringSubmatch(channel)
	if match == nil {
		return "", "", false
	}

	return match[1], match[2], true
}

// BareDMHuman returns the human of a bare @<human> channel, the short form
// a task's calls expand to @<human>/<its task>.
func BareDMHuman(channel string) (human string, ok bool) {
	match := bareDMRegex.FindStringSubmatch(channel)
	if match == nil {
		return "", false
	}

	return match[1], true
}

// DMChannel is the direct message channel between human and task.
func DMChannel(human, task string) string {
	return "@" + human + "/" + task
}

// Message is one entry in the append-only log. The autoincrement id is
// the cursor and gives total order per channel.
type Message struct {
	bun.BaseModel `bun:"table:messaging_messages"`

	ID            int64  `bun:"id,pk,autoincrement" json:"id"`
	Channel       string `bun:"channel,notnull" json:"channel"`
	From          string `bun:"from,notnull" json:"from"`
	ParticipantID string `bun:"participant_id,notnull" json:"participant_id"`
	Task          string `bun:"task,notnull" json:"task"`
	// Human is copied from the participant: false for agc and agents.
	Human     bool      `bun:"human,notnull,default:false" json:"human"`
	ReplyTo   *int64    `bun:"reply_to,nullzero" json:"reply_to,omitempty"`
	Body      string    `bun:"body,notnull" json:"body"`
	CreatedAt time.Time `bun:"created_at,notnull" json:"created_at"`
}

var _ bun.AfterCreateTableHook = (*Message)(nil)

// AfterCreateTable adds the index every read and wait uses.
func (*Message) AfterCreateTable(ctx context.Context, query *bun.CreateTableQuery) error {
	_, err := query.DB().NewCreateIndex().
		Model((*Message)(nil)).
		Index("messaging_messages_channel_id_idx").
		Column("channel", "id").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

func ValidateChannel(channel string) error {
	_, _, isDM := ParseDM(channel)
	if !channelRegex.MatchString(channel) && !isDM {
		msg := fmt.Sprintf("channel %q is invalid: use # followed by 1-64 lowercase letters, digits, _ or -, or @human/task for a direct message", channel)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

func (m *Message) Validate() error {
	if m == nil {
		return ez.New(ez.EINVALID, "message is nil", nil)
	}

	err := ValidateChannel(m.Channel)
	if err != nil {
		return ez.Wrap(err)
	}

	if m.ParticipantID == "" || m.From == "" {
		return ez.New(ez.EINVALID, "message needs a participant", nil)
	}

	if strings.TrimSpace(m.Body) == "" {
		return ez.New(ez.EINVALID, "body is required", nil)
	}

	if len(m.Body) > MaxBodyBytes {
		return ez.New(ez.EINVALID, fmt.Sprintf("body is %d bytes, max is %d", len(m.Body), MaxBodyBytes), nil)
	}

	return nil
}

func (m *Message) Insert(ctx context.Context, db bun.IDB) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}

	err := m.Validate()
	if err != nil {
		return ez.Wrap(err)
	}

	_, err = db.NewInsert().Model(m).Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}
