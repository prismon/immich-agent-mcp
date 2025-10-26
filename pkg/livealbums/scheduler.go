package livealbums

import (
	"context"
	"sync"

	"github.com/yourusername/mcp-immich/pkg/config"
	"github.com/yourusername/mcp-immich/pkg/immich"
	"github.com/robfig/cron/v3"
	"github.com/rs/zerolog/log"
)

// Scheduler manages periodic live album updates
type Scheduler struct {
	cfg      *config.Config
	client   *immich.Client
	updater  *Updater
	cron     *cron.Cron
	mu       sync.Mutex
	running  bool
}

// NewScheduler creates a new live album scheduler
func NewScheduler(cfg *config.Config, client *immich.Client) *Scheduler {
	return &Scheduler{
		cfg:     cfg,
		client:  client,
		updater: NewUpdater(client),
		cron:    cron.New(cron.WithSeconds()),
		running: false,
	}
}

// Start starts the scheduler
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		log.Warn().Msg("Live album scheduler already running")
		return nil
	}

	if !s.cfg.EnableLiveAlbums {
		log.Info().Msg("Live albums disabled in configuration, scheduler not started")
		return nil
	}

	log.Info().
		Str("cron_expression", s.cfg.LiveAlbumUpdateCron).
		Str("sync_strategy", s.cfg.LiveAlbumSyncStrategy).
		Int("max_results", s.cfg.LiveAlbumMaxResults).
		Msg("Starting live album scheduler")

	// Add cron job
	_, err := s.cron.AddFunc(s.cfg.LiveAlbumUpdateCron, func() {
		s.runUpdate()
	})
	if err != nil {
		return err
	}

	s.cron.Start()
	s.running = true

	log.Info().Msg("Live album scheduler started successfully")

	return nil
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	log.Info().Msg("Stopping live album scheduler")

	ctx := s.cron.Stop()
	<-ctx.Done()

	s.running = false

	log.Info().Msg("Live album scheduler stopped")
}

// IsRunning returns whether the scheduler is running
func (s *Scheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// RunNow triggers an immediate update of all live albums
func (s *Scheduler) RunNow(ctx context.Context) ([]UpdateResult, error) {
	log.Info().Msg("Running live album update on demand")
	return s.updater.UpdateAllLiveAlbums(ctx)
}

// runUpdate is called by the cron scheduler
func (s *Scheduler) runUpdate() {
	ctx := context.Background()

	log.Info().Msg("Starting scheduled live album update")

	results, err := s.updater.UpdateAllLiveAlbums(ctx)
	if err != nil {
		log.Error().Err(err).Msg("Failed to update live albums")
		return
	}

	// Log summary
	successCount := 0
	errorCount := 0
	totalAdded := 0
	totalRemoved := 0

	for _, result := range results {
		if result.Error != nil {
			errorCount++
			log.Error().
				Err(result.Error).
				Str("album_id", result.AlbumID).
				Str("album_name", result.AlbumName).
				Msg("Failed to update live album")
		} else {
			successCount++
			totalAdded += result.AssetsAdded
			totalRemoved += result.AssetsRemoved
		}
	}

	log.Info().
		Int("success", successCount).
		Int("errors", errorCount).
		Int("total_added", totalAdded).
		Int("total_removed", totalRemoved).
		Msg("Scheduled live album update completed")
}
