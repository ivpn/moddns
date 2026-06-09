package service

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ivpn/dns/blocklists/internal/downloader"
	"github.com/ivpn/dns/blocklists/internal/extractor"
	"github.com/ivpn/dns/blocklists/internal/metrics"
	"github.com/ivpn/dns/blocklists/model"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	jsonExt           = ".json"
	processingTimeout = 1 * time.Minute
)

// errDownloadTooLarge signals that a source's response body exceeded the
// configured size limit — a truncated/abusive source rather than a successfully
// fetched list. It aliases downloader.ErrTooLarge so processBlocklist can map an
// over-size download to the "truncated" validation-rejected metric. The size
// limit itself is configured via downloader.Config.MaxBodySize.
var errDownloadTooLarge = downloader.ErrTooLarge

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

// Trigger is called to launch the processing of all blocklists. It emits a
// single summary log at the end (succeeded/failed/total) so a startup refresh
// can be assessed at a glance, naming any sources that failed.
func (s *Service) Trigger(sources []model.BlocklistMetadata) {
	var failedSources []string
	for _, src := range sources {
		_, err := s.ProcessBlocklist(src)
		if err != nil {
			failedSources = append(failedSources, src.Name)
			log.Err(err).Str("source", src.Name).Msg("Failed to process blocklist")
			continue
		}
	}

	event := log.Info()
	if len(failedSources) > 0 {
		event = log.Warn().Strs("failed_sources", failedSources)
	}
	event.
		Int("succeeded", len(sources)-len(failedSources)).
		Int("failed", len(failedSources)).
		Int("total", len(sources)).
		Msg("Blocklist startup refresh complete")
}

// ProcessBlocklist downloads, validates and publishes a single blocklist,
// recording update metrics (duration, success/failure, last-success timestamp).
func (s *Service) ProcessBlocklist(metadata model.BlocklistMetadata) (*model.BlocklistMetadata, error) {
	source := metadata.BlocklistID
	start := time.Now()

	result, err := s.processBlocklist(metadata)

	s.Metrics.RecordDuration(source, time.Since(start))
	if err != nil {
		s.Metrics.RecordUpdate(source, metrics.StatusFailure)
		return nil, err
	}
	s.Metrics.RecordUpdate(source, metrics.StatusSuccess)
	s.Metrics.SetLastSuccess(source, time.Now())
	return result, nil
}

func (s *Service) processBlocklist(metadata model.BlocklistMetadata) (*model.BlocklistMetadata, error) {
	ctx, cancel := context.WithTimeout(context.Background(), processingTimeout)
	defer cancel()

	if metadata.Name == "" {
		metadata.Name = "My First Blocklist"
	}

	// Download and process data first to know the total size. The shared
	// downloader (per-host throttling, global concurrency cap, retry/backoff) is
	// always set by service.New; the blocklist ID is the retry-metric label.
	blocklistBytes, err := s.Downloader.Fetch(ctx, metadata.BlocklistID, metadata.SourceUrl)
	if err != nil {
		if errors.Is(err, errDownloadTooLarge) {
			s.Metrics.RecordValidationRejected(metadata.BlocklistID, metrics.ReasonTruncated)
		}
		log.Err(err).Str("source_url", metadata.SourceUrl).Msg("Failed to download blocklist")
		return nil, err
	}
	s.Metrics.RecordDownloadBytes(metadata.BlocklistID, int64(len(blocklistBytes)))

	// Strip a leading UTF-8 BOM so it does not corrupt the first header line
	// (metadata extraction) or the first domain.
	blocklistBytes = bytes.TrimPrefix(blocklistBytes, []byte("\uFEFF"))

	extr, err := extractor.NewExtractor(metadata.BlocklistID)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to create extractor")
		return nil, err
	}

	lastModified, version, numEntries, err := extr.ExtractMetadata(blocklistBytes)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to extract metadata")
		return nil, err
	}

	domainsBytes, err := extr.Convert(blocklistBytes)
	if err != nil {
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Failed to convert blocklist")
		return nil, err
	}

	const maxDomainsPerDoc = 100000

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
		log.Error().Str("blocklist_id", metadata.BlocklistID).Msg("number of blocklists found is not proper")
		return nil, fmt.Errorf("number of blocklists found is not proper")
	}

	// Scan all lines into a single validated slice BEFORE persisting anything.
	// A corrupt/truncated download fails the validation gate below, leaving the
	// previously published Redis/Mongo data untouched.
	validated, err := scanValidatedDomains(bytes.NewReader(domainsBytes))
	if err != nil {
		s.Metrics.RecordValidationRejected(metadata.BlocklistID, metrics.ReasonScanError)
		log.Err(err).Str("blocklist_id", metadata.BlocklistID).Msg("Scan error while processing blocklist; aborting swap")
		return nil, fmt.Errorf("scan blocklist %s: %w", metadata.BlocklistID, err)
	}

	totalDomains := len(validated)

	// Validation gate: refuse to publish an empty or sharply-shrunken list.
	if err := s.checkValidationGate(metadata.BlocklistID, existingMetadata, totalDomains, numEntries); err != nil {
		return nil, err
	}

	s.Metrics.SetDomainsExtracted(metadata.BlocklistID, totalDomains)
	if numEntries > 0 {
		// Source's own count (header or self-counted) — a divergence signal
		// against the published count above.
		s.Metrics.SetDeclaredEntries(metadata.BlocklistID, numEntries)
	}

	// Persist Mongo content chunks from the validated domains.
	for i := 0; i < len(validated); i += maxDomainsPerDoc {
		end := i + maxDomainsPerDoc
		if end > len(validated) {
			end = len(validated)
		}
		chunkIndex := i/maxDomainsPerDoc + 1
		if _, err := s.saveChunk(ctx, metadata.BlocklistID, chunkIndex, validated[i:end]); err != nil {
			log.Err(err).
				Str("blocklist_id", metadata.BlocklistID).
				Int("chunk", chunkIndex).
				Msg("Failed to save chunk")
			return nil, err
		}
	}

	// Publish the SAME validated domains to the Redis set the proxy reads.
	data := []byte(strings.Join(validated, "\n"))
	if err := s.Cache.CreateOrUpdateBlocklist(ctx, metadata.BlocklistID, data); err != nil {
		return nil, err
	}

	metadata.LastModified = lastModified
	metadata.Version = version
	metadata.Entries = totalDomains
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

// scanValidatedDomains reads the extractor's Convert output (one candidate
// domain per line) and returns the normalized, validated domains. Convert is
// the canonical, format-specific extractor for every source; this shared gate
// then applies consistent normalization (BOM/CR/whitespace/trailing-dot strip,
// lowercase) and validation, so the proxy-visible Redis set and the Mongo
// content never contain comments, mixed case, CRLF artefacts or injected
// non-domain junk. Invalid lines (incl. comment lines) are silently skipped.
//
// It relies on bufio.Scanner's default 64KB line cap (no legitimate domain
// approaches it) and returns the scanner error so the caller can ABORT rather
// than publish a truncated list: a single oversized line yields
// bufio.ErrTooLong instead of silently dropping the rest of the source.
func scanValidatedDomains(r io.Reader) ([]string, error) {
	scanner := bufio.NewScanner(r)
	validated := make([]string, 0)
	for scanner.Scan() {
		domain := extractor.NormalizeDomain(scanner.Text())
		if domain == "" || !extractor.ValidDomain(domain) {
			continue
		}
		validated = append(validated, domain)
	}
	return validated, scanner.Err()
}

// checkValidationGate decides whether a freshly-extracted blocklist may replace
// the currently-published one. It rejects the swap (returning an error and
// recording a metric) when the new list is empty, or — relative to the previous
// run — shrinks by more than UpdaterConfig.ShrinkThreshold. The upstream header
// count is used only as a warn-only sanity signal, since those counts are often
// approximate or stale.
func (s *Service) checkValidationGate(blocklistID string, existingMetadata []model.BlocklistMetadata, newCount, headerCount int) error {
	if newCount == 0 {
		s.Metrics.RecordValidationRejected(blocklistID, metrics.ReasonEmpty)
		return fmt.Errorf("validation gate: blocklist %s produced 0 domains, aborting swap", blocklistID)
	}

	prev := 0
	if len(existingMetadata) == 1 {
		prev = existingMetadata[0].Entries
	}
	if prev > 0 {
		minAllowed := int(float64(prev) * (1 - s.Cfg.Updater.ShrinkThreshold))
		if newCount < minAllowed {
			s.Metrics.RecordValidationRejected(blocklistID, metrics.ReasonShrink)
			return fmt.Errorf("validation gate: blocklist %s shrank to %d domains (min allowed %d, previous %d), aborting swap",
				blocklistID, newCount, minAllowed, prev)
		}
	}

	// Warn-only: large divergence from the upstream header count may indicate a
	// partial download even when the shrink gate passes.
	if headerCount > 0 && newCount < headerCount/2 {
		log.Warn().
			Str("blocklist_id", blocklistID).
			Int("extracted", newCount).
			Int("header_count", headerCount).
			Msg("Extracted domain count is far below the upstream header count")
	}

	return nil
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

	log.Debug().
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
