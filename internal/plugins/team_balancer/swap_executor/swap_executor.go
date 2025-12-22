package swap_executor

import (
	"fmt"
	"sync"
	"time"

	"go.codycody31.dev/squad-aegis/internal/plugin_manager"
)

// MoveStatus tracks the status of a single player move
type MoveStatus struct {
	SteamID     string
	Name        string
	TargetTeam  int
	Attempts    int
	MaxAttempts int
	LastError   error
	Completed   bool
	Failed      bool
	StartedAt   time.Time
	CompletedAt time.Time
}

// ExecutorConfig holds configuration for the swap executor
type ExecutorConfig struct {
	RetryInterval          time.Duration // Time between retry attempts
	MaxCompletionTime      time.Duration // Maximum time to complete all swaps
	WarnOnSwap             bool          // Send warning to players when swapped
	MaxAttemptsPerPlayer   int           // Maximum retry attempts per player
}

// SwapExecutor manages the execution of player team swaps with retry logic
type SwapExecutor struct {
	moves  map[string]*MoveStatus
	config ExecutorConfig
	rcon   plugin_manager.RconAPI
	log    plugin_manager.LogAPI
	mu     sync.Mutex
	wg     sync.WaitGroup
	ctx    chan struct{} // Cancellation channel
}

// New creates a new SwapExecutor
func New(config ExecutorConfig, rcon plugin_manager.RconAPI, log plugin_manager.LogAPI) *SwapExecutor {
	if config.MaxAttemptsPerPlayer == 0 {
		config.MaxAttemptsPerPlayer = 5
	}
	if config.RetryInterval == 0 {
		config.RetryInterval = 200 * time.Millisecond
	}
	if config.MaxCompletionTime == 0 {
		config.MaxCompletionTime = 15 * time.Second
	}

	return &SwapExecutor{
		moves:  make(map[string]*MoveStatus),
		config: config,
		rcon:   rcon,
		log:    log,
		ctx:    make(chan struct{}),
	}
}

// QueueMove adds a player move to the execution queue
func (e *SwapExecutor) QueueMove(steamID, name string, targetTeam int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.moves[steamID] = &MoveStatus{
		SteamID:     steamID,
		Name:        name,
		TargetTeam:  targetTeam,
		Attempts:    0,
		MaxAttempts: e.config.MaxAttemptsPerPlayer,
		Completed:   false,
		Failed:      false,
		StartedAt:   time.Now(),
	}
}

// ExecuteAll executes all queued moves with retry logic
func (e *SwapExecutor) ExecuteAll() error {
	e.mu.Lock()
	if len(e.moves) == 0 {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	e.log.Info("Starting swap execution", map[string]interface{}{
		"total_moves": len(e.moves),
	})

	// Start execution goroutines for each move
	e.mu.Lock()
	for steamID := range e.moves {
		e.wg.Add(1)
		go e.executeMove(steamID)
	}
	e.mu.Unlock()

	// Wait for completion or timeout
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		e.log.Info("All swap moves completed", nil)
		return nil
	case <-time.After(e.config.MaxCompletionTime):
		e.log.Warn("Swap execution timed out", map[string]interface{}{
			"timeout": e.config.MaxCompletionTime.String(),
		})
		e.Cancel()
		return fmt.Errorf("swap execution timed out after %v", e.config.MaxCompletionTime)
	}
}

// executeMove executes a single player move with retry logic
func (e *SwapExecutor) executeMove(steamID string) {
	defer e.wg.Done()

	for {
		// Check if cancelled
		select {
		case <-e.ctx:
			e.mu.Lock()
			if move, exists := e.moves[steamID]; exists {
				move.Failed = true
			}
			e.mu.Unlock()
			return
		default:
		}

		e.mu.Lock()
		move, exists := e.moves[steamID]
		if !exists {
			e.mu.Unlock()
			return
		}

		// Check if already completed or failed
		if move.Completed || move.Failed {
			e.mu.Unlock()
			return
		}

		// Check if max attempts reached
		if move.Attempts >= move.MaxAttempts {
			move.Failed = true
			e.log.Error("Player move failed after max attempts", fmt.Errorf("max attempts reached"), map[string]interface{}{
				"steamID":     steamID,
				"name":        move.Name,
				"attempts":    move.Attempts,
				"target_team": move.TargetTeam,
			})
			e.mu.Unlock()
			return
		}

		move.Attempts++
		currentAttempt := move.Attempts
		e.mu.Unlock()

		// Execute the team change command
		command := fmt.Sprintf("AdminForceTeamChange %s", steamID)
		_, err := e.rcon.SendCommand(command)

		e.mu.Lock()
		if err != nil {
			move.LastError = err
			e.log.Debug("Player move attempt failed, will retry", map[string]interface{}{
				"steamID":     steamID,
				"name":        move.Name,
				"attempt":     currentAttempt,
				"max_attempts": move.MaxAttempts,
				"error":       err.Error(),
			})
			e.mu.Unlock()

			// Wait before retry
			time.Sleep(e.config.RetryInterval)
			continue
		}

		// Success
		move.Completed = true
		move.CompletedAt = time.Now()
		e.log.Debug("Player moved successfully", map[string]interface{}{
			"steamID":     steamID,
			"name":        move.Name,
			"target_team": move.TargetTeam,
			"attempts":    currentAttempt,
		})

		// Send warning to player if configured
		if e.config.WarnOnSwap {
			warnMsg := fmt.Sprintf("You have been moved to balance teams. Thank you for your cooperation!")
			if warnErr := e.rcon.SendWarningToPlayer(steamID, warnMsg); warnErr != nil {
				e.log.Debug("Failed to send warning to swapped player", map[string]interface{}{
					"steamID": steamID,
					"error":   warnErr.Error(),
				})
			}
		}

		e.mu.Unlock()
		return
	}
}

// WaitForCompletion waits for all moves to complete or timeout
func (e *SwapExecutor) WaitForCompletion(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("wait timed out after %v", timeout)
	}
}

// GetStatus returns the current status of all moves
func (e *SwapExecutor) GetStatus() map[string]*MoveStatus {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Return a copy to avoid race conditions
	status := make(map[string]*MoveStatus)
	for k, v := range e.moves {
		moveCopy := *v
		status[k] = &moveCopy
	}
	return status
}

// GetSummary returns a summary of move execution
func (e *SwapExecutor) GetSummary() (total, completed, failed, pending int) {
	e.mu.Lock()
	defer e.mu.Unlock()

	total = len(e.moves)
	for _, move := range e.moves {
		if move.Completed {
			completed++
		} else if move.Failed {
			failed++
		} else {
			pending++
		}
	}
	return
}

// Cancel cancels all pending moves
func (e *SwapExecutor) Cancel() {
	close(e.ctx)
}

// Cleanup resets the executor state
func (e *SwapExecutor) Cleanup() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.moves = make(map[string]*MoveStatus)
	e.ctx = make(chan struct{})
}

// IsPending returns true if there are any pending moves
func (e *SwapExecutor) IsPending() bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, move := range e.moves {
		if !move.Completed && !move.Failed {
			return true
		}
	}
	return false
}

