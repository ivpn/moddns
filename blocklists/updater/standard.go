package updater

import (
	"context"

	"github.com/ivpn/dns/blocklists/model"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

type StandardUpdater struct {
	cron *cron.Cron
}

func NewStandardUpdater() *StandardUpdater {
	c := cron.New()
	return &StandardUpdater{
		cron: c,
	}
}

// Setup adds a new, single blocklist to the cron scheduler
func (u *StandardUpdater) Setup(source model.BlocklistMetadata, blocklistFunc func() (*model.BlocklistMetadata, error)) error {
	entryID, err := u.cron.AddFunc(source.Schedule, func() {
		log.Debug().Str("source", source.Name).Msg("Processing blocklist")
		_, err := blocklistFunc()
		if err != nil {
			log.Err(err).Str("blocklist_id", source.BlocklistID).Str("source", source.Name).Msg("Failed to process blocklist")
			return
		}
		log.Debug().Str("source", source.Name).Msg("Processed blocklist")
	})
	if err != nil {
		log.Err(err).Str("source", source.Name).Msg("Failed to add source to cron")
		return err
	}
	log.Info().Str("source", source.Name).Int("entry_id", int(entryID)).Msg("Added source to cron")
	return nil
}

// Start starts the cron scheduler
func (u *StandardUpdater) Start() {
	u.cron.Start()
}

// Erase removes all cron entries
func (u *StandardUpdater) Erase() {
	log.Info().Msg("Erasing standard updater cron entries")
	for _, entry := range u.cron.Entries() {
		u.cron.Remove(entry.ID)
	}
}

// Stop stops the cron scheduler
func (u *StandardUpdater) Stop() context.Context {
	log.Info().Msg("Stopping standard updater")
	return u.cron.Stop()
}
