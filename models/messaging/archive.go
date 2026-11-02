package messaging

import (
	"context"
	"time"

	"github.com/uptrace/bun"
	"github.com/vanclief/ez"
)

// ChannelArchive marks a channel archived (ADR 0005 "Archived channels"):
// hidden from the channel list by default, still readable, closed to posts
// from anyone but agc. Channels have no rows of their own, so this is the
// one place the state lives.
type ChannelArchive struct {
	bun.BaseModel `bun:"table:channel_archives"`

	ID         int64     `bun:"id,pk,autoincrement" json:"id"`
	Channel    string    `bun:"channel,notnull,unique" json:"channel"`
	ArchivedAt time.Time `bun:"archived_at,notnull" json:"archived_at"`
}

// ArchiveChannel archives channel at at. It answers false when the channel
// was already archived, which leaves it unchanged.
func ArchiveChannel(ctx context.Context, db bun.IDB, channel string, at time.Time) (bool, error) {
	err := ValidateChannel(channel)
	if err != nil {
		return false, ez.Wrap(err)
	}

	record := &ChannelArchive{Channel: channel, ArchivedAt: at.UTC()}
	res, err := db.NewInsert().Model(record).On("CONFLICT (channel) DO NOTHING").Exec(ctx)
	if err != nil {
		return false, ez.Wrap(err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return false, ez.Wrap(err)
	}

	return rows > 0, nil
}

// ArchivedChannels maps every archived channel to when it was archived.
func ArchivedChannels(ctx context.Context, db bun.IDB) (map[string]time.Time, error) {
	var records []ChannelArchive

	err := db.NewSelect().Model(&records).Scan(ctx)
	if err != nil {
		return nil, ez.Wrap(err)
	}

	archived := map[string]time.Time{}
	for _, record := range records {
		archived[record.Channel] = record.ArchivedAt
	}

	return archived, nil
}

// ChannelArchived reports whether channel is archived.
func ChannelArchived(ctx context.Context, db bun.IDB, channel string) (bool, error) {
	exists, err := db.NewSelect().Model((*ChannelArchive)(nil)).Where("channel = ?", channel).Exists(ctx)
	if err != nil {
		return false, ez.Wrap(err)
	}

	return exists, nil
}
