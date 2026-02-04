package identity

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
)

// Worker runs periodic identity resolution in the background
type Worker struct {
	resolver     *Resolver
	interval     time.Duration
	stopCh       chan struct{}
	mu           sync.Mutex
	running      bool
	lastRun      time.Time
	lastDuration time.Duration
	lastError    error
}

// NewWorker creates a new identity worker
func NewWorker(ch *clickhouse.Client, interval time.Duration) *Worker {
	return &Worker{
		resolver: NewResolver(ch),
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the periodic identity resolution
// This should be called as a goroutine
func (w *Worker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		log.Warn().Msg("Identity worker already running")
		return
	}
	w.running = true
	w.mu.Unlock()

	log.Info().
		Dur("interval", w.interval).
		Msg("Starting identity worker")

	// Run immediately on startup after a short delay to let other services initialize
	time.Sleep(30 * time.Second)
	w.runOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.runOnce(ctx)
		case <-w.stopCh:
			log.Info().Msg("Identity worker stopped")
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			return
		case <-ctx.Done():
			log.Info().Msg("Identity worker context cancelled")
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			return
		}
	}
}

// Stop stops the identity worker
func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		close(w.stopCh)
	}
}

// RunNow triggers an immediate identity computation
func (w *Worker) RunNow(ctx context.Context) error {
	return w.runOnce(ctx)
}

// runOnce executes a single identity computation
func (w *Worker) runOnce(ctx context.Context) error {
	startTime := time.Now()

	log.Info().Msg("Running identity computation")

	err := w.resolver.ComputeIdentities(ctx)

	w.mu.Lock()
	w.lastRun = startTime
	w.lastDuration = time.Since(startTime)
	w.lastError = err
	w.mu.Unlock()

	if err != nil {
		log.Error().
			Err(err).
			Dur("duration", w.lastDuration).
			Msg("Identity computation failed")
	} else {
		log.Info().
			Dur("duration", w.lastDuration).
			Msg("Identity computation completed")
	}

	return err
}

// Status returns the current status of the worker
func (w *Worker) Status() WorkerStatus {
	w.mu.Lock()
	defer w.mu.Unlock()

	return WorkerStatus{
		Running:      w.running,
		Interval:     w.interval,
		LastRun:      w.lastRun,
		LastDuration: w.lastDuration,
		LastError:    w.lastError,
	}
}

// WorkerStatus represents the current state of the identity worker
type WorkerStatus struct {
	Running      bool
	Interval     time.Duration
	LastRun      time.Time
	LastDuration time.Duration
	LastError    error
}
