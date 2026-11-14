package workers

import (
	"context"
	"log"
	"time"
	"task-manager/internal/repositories"
)

const cleanupTickTimeout = 10 * time.Second

type TokenCleanupWorker struct {
	tokenRepository repositories.TokenRepository
	interval        time.Duration
}

func NewTokenCleanupWorker(tokenRepository repositories.TokenRepository, interval time.Duration) *TokenCleanupWorker {
	return &TokenCleanupWorker{
		tokenRepository: tokenRepository,
		interval:        interval,
	}
}

// Start spawns the periodic cleanup loop in a background goroutine.
func (worker *TokenCleanupWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(worker.interval)
		defer ticker.Stop()
		log.Printf("[Cleanup Worker] Started periodic expired token cleanup (interval: %v)\n", worker.interval)

		for {
			select {
			case <-ticker.C:
				worker.runCleanupTick(ctx)
			case <-ctx.Done():
				log.Println("[Cleanup Worker] Stopped periodic token cleanup worker")
				return
			}
		}
	}()
}

// runCleanupTick executes a single cleanup pass with its own timeout so
// defer cancelTimeout runs on every tick rather than stacking in the loop.
func (worker *TokenCleanupWorker) runCleanupTick(parentCtx context.Context) {
	tickCtx, cancelTimeout := context.WithTimeout(parentCtx, cleanupTickTimeout)
	defer cancelTimeout()

	deletedCount, err := worker.tokenRepository.DeleteExpiredTokens(tickCtx)
	if err != nil {
		log.Printf("[Cleanup Worker] Error purging expired tokens: %v\n", err)
		return
	}

	if deletedCount > 0 {
		log.Printf("[Cleanup Worker] Purged %d expired revoked tokens\n", deletedCount)
	}
}
