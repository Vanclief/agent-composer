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
	if !channelRegex.MatchString(channel) {
		msg := fmt.Sprintf("channel %q is invalid: use # followed by 1-64 lowercase letters, digits, _ or -", channel)
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
