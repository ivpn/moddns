package service

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ivpn/dns/blocklists/internal/extractor"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	jsonExt                 = ".json"
	processingTimeout       = 1 * time.Minute
	downloadTimeout         = 30 * time.Second
	maxBlocklistSize  int64 = 100 * 1024 * 1024 // 100MB limit
)

func (s *Service) ReadSources() ([]model.BlocklistMetadata, error) {
	err := filepath.Walk(s.Cfg.Updater.SourcesDir, s.visit)
	if err != nil {
		log.Err(err).Str("sources_dir", s.Cfg.Updater.SourcesDir).Msg("Error walking the sources directory")
		return nil, err
	}
	return s.Blocklists, nil
}

func (s *Service) visit(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}

	if info.IsDir() {
		return nil
	}

	if filepath.Ext(path) == jsonExt {
		blocklistSources, err := NewSources(path)
		if err != nil {
			log.Err(err).Str("path", path).Msg("Error reading blocklist source file")
			return err
		}
		s.Blocklists = append(s.Blocklists, blocklistSources...)
	}

	return nil
}

func (s *Service) Setup(sources []model.BlocklistMetadata) error {
	for _, src := range sources {
		// Create a closure that captures the current value of source
		blocklistFunc := func() (*model.BlocklistMetadata, error) {
			return s.ProcessBlocklist(src)
		}
		if err := s.Updater.Setup(src, blocklistFunc); err != nil {
			log.Err(err).Str("source", src.Name).Msg("Failed to setup updater")
			return err
		}
	}
	return nil
}

// Trigger is called to launch the processing of all blocklists
func (s *Service) Trigger(sources []model.BlocklistMetadata) {
	for _, src := range sources {
		_, err := s.ProcessBlocklist(src)
		if err != nil {
			log.Err(err).Str("source", src.Name).Msg("Failed to process blocklist")
			continue
		}
	}
}

func (s *Service) ProcessBlocklist(metadata model.BlocklistMetadata) (*model.BlocklistMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), processingTimeout)
	defer cancel()

	if metadata.Name == "" {
		metadata.Name = "My First Blocklist"
	}

	// Download and process data first to know the total size
	blocklistBytes, err := s.download(ctx, metadata.SourceUrl)
	if err != nil {
		log.Err(err).Str("source_url", metadata.SourceUrl).Msg("Failed to download blocklist")
		return nil, err
	}

	extractor, err := extractor.NewExtractor(metadata.BlocklistID)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to create extractor")
		return nil, err
	}

	lastModified, version, numEntries, err := extractor.ExtractMetadata(blocklistBytes)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to extract metadata")
		return nil, err
	}

	domainsBytes, err := extractor.Convert(blocklistBytes)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to convert blocklist")
		return nil, err
	}

	// Process domains line by line and create chunks
	const maxDomainsPerDoc = 100000
	var currentChunk []string
	chunkIndex := 1

	reader := bytes.NewReader(domainsBytes)
	scanner := bufio.NewScanner(reader)

	fltr := map[string]any{"blocklist_id": metadata.BlocklistID}
	existingBlocklists, err := s.Store.GetContent(ctx, fltr)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to get blocklist content")
		return nil, err
	}
	var removeOldContents bool
	if len(existingBlocklists) > 0 {
		removeOldContents = true
	}

	existingMetadata, err := s.Store.GetMetadata(ctx, fltr)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to get blocklist metadata")
		return nil, err
	}
	switch len(existingMetadata) {
	case 0:
		metadata.ID = primitive.NewObjectID()
	case 1:
		metadata.ID = existingMetadata[0].ID
	default:
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("number of blocklists found is not proper")
		return nil, fmt.Errorf("number of blocklists found is not proper")
	}

	for scanner.Scan() {
		line := scanner.Text()
		// Skip empty lines
		if len(strings.TrimSpace(line)) == 0 {
			continue
		}

		domain, err := extractor.ProcessLine(line)
		if err != nil {
			log.Warn().Err(err).Str("line", line).Msg("Failed to process line")
			continue
		}

		if domain != "" {
			currentChunk = append(currentChunk, domain)
			// allDomains = append(allDomains, domain)
		}

		// When chunk reaches max size, save it to MongoDB
		if len(currentChunk) >= maxDomainsPerDoc {
			_, err := s.saveChunk(ctx, metadata.BlocklistID, chunkIndex, currentChunk)
			if err != nil {
				log.Err(err).
					Str("blocklist_id", metadata.BlocklistID).
					Int("chunk", chunkIndex).
					Msg("Failed to save chunk")
				return nil, err
			}

			chunkIndex++
			currentChunk = make([]string, 0, maxDomainsPerDoc)
		}
	}

	// Save the last chunk if it contains any domains
	if len(currentChunk) > 0 {
		_, err := s.saveChunk(ctx, metadata.BlocklistID, chunkIndex, currentChunk)
		if err != nil {
			log.Err(err).
				Str("blocklist_id", metadata.BlocklistID).
				Int("chunk", chunkIndex).
				Msg("Failed to save last chunk")
			return nil, err
		}
	}

	// Update cache with complete domain list
	// update all domains at once
	if err := s.Cache.CreateOrUpdateBlocklist(ctx, metadata.BlocklistID, domainsBytes); err != nil {
		return nil, err
	}

	metadata.LastModified = lastModified
	metadata.Version = version
	metadata.Entries = numEntries
	metadata.Type = model.BlocklistTypePublic

	// Update metadata first
	if err := s.Store.UpsertMetadata(ctx, metadata); err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to upsert blocklist metadata")
		return nil, err
	}
	// remove old blocklist contents
	if removeOldContents {
		existingIDs := make([]primitive.ObjectID, 0)
		for _, existingBlocklist := range existingBlocklists {
			existingIDs = append(existingIDs, existingBlocklist.ID)
		}
		fltr := map[string]any{"_id": existingIDs}
		if err := s.Store.Delete(ctx, fltr); err != nil {
			log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to delete old blocklist contents")
		}
	}

	return &metadata, nil
}

// saveChunk saves a chunk of domains to MongoDB
func (s *Service) saveChunk(ctx context.Context, blocklistID string, chunkIndex int, domains []string) (primitive.ObjectID, error) {
	partialBlocklistContent, err := model.NewBlocklistContent(blocklistID, chunkIndex, domains)
	if err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to create blocklist content: %w", err)
	}

	if err := s.Store.UpsertContent(ctx, *partialBlocklistContent); err != nil {
		return primitive.NilObjectID, fmt.Errorf("failed to upsert blocklist content: %w", err)
	}

	log.Info().
		Str("blocklist_id", blocklistID).
		Int("chunk", chunkIndex).
		Int("domains", len(domains)).
		Msg("Saved blocklist chunk")

	return partialBlocklistContent.ID, nil
}

// PurgeStale removes metadata and content for blocklists that are no longer
// present in the current sources. This ensures that removed blocklists don't
// linger in the database and get served by the API.
func (s *Service) PurgeStale(sources []model.BlocklistMetadata) {
	ctx, cancel := context.WithTimeout(context.Background(), processingTimeout)
	defer cancel()

	sourceIDs := make([]string, 0, len(sources))
	for _, src := range sources {
		sourceIDs = append(sourceIDs, src.BlocklistID)
	}

	// Get all metadata currently in the database
	allMetadata, err := s.Store.GetMetadata(ctx, map[string]any{})
	if err != nil {
		log.Err(err).Msg("Failed to get all blocklist metadata for stale check")
		return
	}

	staleIDs := make([]string, 0)
	sourceSet := make(map[string]struct{}, len(sourceIDs))
	for _, id := range sourceIDs {
		sourceSet[id] = struct{}{}
	}
	for _, meta := range allMetadata {
		if _, exists := sourceSet[meta.BlocklistID]; !exists {
			staleIDs = append(staleIDs, meta.BlocklistID)
		}
	}

	if len(staleIDs) == 0 {
		log.Debug().Msg("No stale blocklists to purge")
		return
	}

	log.Info().Strs("blocklist_ids", staleIDs).Msg("Purging stale blocklists")

	for _, id := range staleIDs {
		// Delete metadata
		if err := s.Store.DeleteMetadata(ctx, map[string]any{"blocklist_id": id}); err != nil {
			log.Err(err).Str("blocklist_id", id).Msg("Failed to delete stale metadata")
		}
		// Delete content
		if err := s.Store.Delete(ctx, map[string]any{"blocklist_id": id}); err != nil {
			log.Err(err).Str("blocklist_id", id).Msg("Failed to delete stale content")
		}
		// Delete from cache
		if err := s.Cache.DeleteBlocklist(ctx, id); err != nil {
			log.Err(err).Str("blocklist_id", id).Msg("Failed to delete stale blocklist from cache")
		}
	}

	log.Info().Int("count", len(staleIDs)).Msg("Purged stale blocklists")
}

// download fetches blocklist data from the given link
func (s *Service) download(ctx context.Context, link string) ([]byte, error) {
	client := &http.Client{
		Timeout: downloadTimeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		},
	}
	req, err := http.NewRequestWithContext(ctx, "GET", link, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bad status: %s", resp.Status)
	}

	var buffer bytes.Buffer
	_, err = io.CopyBuffer(&buffer, io.LimitReader(resp.Body, maxBlocklistSize), make([]byte, 32*1024))
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
