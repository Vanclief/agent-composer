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

// Message kinds. A question is a message its asker needs answered; a
// reply to it from anyone else answers it.
const (
	KindMessage  = "message"
	KindQuestion = "question"
)

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
	// Agent is copied from the participant: the home sandbox of the
	// persistent agent that posted (ADR 0016), else empty.
	Agent string `bun:"agent,notnull,default:''" json:"agent"`
	// Human is copied from the participant: false for agc and agents.
	Human   bool   `bun:"human,notnull,default:false" json:"human"`
	ReplyTo *int64 `bun:"reply_to,nullzero" json:"reply_to,omitempty"`
	Body    string `bun:"body,notnull" json:"body"`
	// Kind is message or question.
	Kind string `bun:"kind,notnull,default:'message'" json:"kind"`
	// Images are the images the message carries (ADR 0005 "Images"),
	// kept under the agc home; each URL is filled in from the id.
	Images    []Image   `bun:"images,type:jsonb,nullzero" json:"images,omitempty"`
	CreatedAt time.Time `bun:"created_at,notnull" json:"created_at"`
}

var _ bun.AfterScanRowHook = (*Message)(nil)

// AfterScanRow fills in each image's URL.
func (m *Message) AfterScanRow(ctx context.Context) error {
	m.FillImageURLs()

	return nil
}

// FillImageURLs sets each image's URL from the message id.
func (m *Message) FillImageURLs() {
	for i := range m.Images {
		m.Images[i].URL = ImagePath(m.ID, m.Images[i].Name)
	}
}

var _ bun.AfterCreateTableHook = (*Message)(nil)

// AfterCreateTable adds the index every read and wait uses, and the one
// that finds the replies answering a question.
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

	_, err = query.DB().NewCreateIndex().
		Model((*Message)(nil)).
		Index(ReplyToIndex).
		Column("reply_to").
		IfNotExists().
		Exec(ctx)
	if err != nil {
		return ez.Wrap(err)
	}

	return nil
}

// ReplyToIndex finds the replies to a message.
const ReplyToIndex = "messaging_messages_reply_to_idx"

func ValidateChannel(channel string) error {
	if !channelRegex.MatchString(channel) {
		msg := fmt.Sprintf("channel %q is invalid: use # followed by 1-64 lowercase letters, digits, _ or -", channel)
		return ez.New(ez.EINVALID, msg, nil)
	}

	return nil
}

// ValidateKind accepts message and question.
func ValidateKind(kind string) error {
	if kind != KindMessage && kind != KindQuestion {
		msg := fmt.Sprintf("kind %q is invalid: use %s or %s", kind, KindMessage, KindQuestion)
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

	err = ValidateKind(m.Kind)
	if err != nil {
		return ez.Wrap(err)
	}

	if strings.TrimSpace(m.Body) == "" && len(m.Images) == 0 {
		return ez.New(ez.EINVALID, "body is required", nil)
	}

	if len(m.Images) > MaxImages {
		msg := fmt.Sprintf("a message carries at most %d images, this one has %d", MaxImages, len(m.Images))
		return ez.New(ez.EINVALID, msg, nil)
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

	if m.Kind == "" {
		m.Kind = KindMessage
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
