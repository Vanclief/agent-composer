package controller

import (
	"os"
	"time"
)

// BuiltinArchiveDone is how long after its task is done a #<task> channel
// stays listed when neither the config file nor the environment sets
// archive_done (ADR 0005 "Archived channels").
const BuiltinArchiveDone = 7 * 24 * time.Hour

// MessagingSettings is the messaging: block of the config file.
type MessagingSettings struct {
	// ArchiveDone is how long after its task is done agc rest archives
	// #<task>, a Go duration such as 168h; 0 turns archiving off.
	ArchiveDone string `mapstructure:"archive_done" json:"archive_done"`
}

// ArchiveDone returns the channels' grace period: AGC_MESSAGING_ARCHIVE_DONE
// overrides the config file, which overrides the built-in 7 days. 0 means
// archiving is off.
func (controller *Controller) ArchiveDone() (time.Duration, error) {
	return ResolveArchiveDone(controller.Config.Messaging)
}

// ResolveArchiveDone applies the environment and the built-in fallback to
// configured.
func ResolveArchiveDone(configured MessagingSettings) (time.Duration, error) {
	value := firstSet(os.Getenv("AGC_MESSAGING_ARCHIVE_DONE"), configured.ArchiveDone)
	if value == "" {
		return BuiltinArchiveDone, nil
	}

	return ParseKeep(value)
}
