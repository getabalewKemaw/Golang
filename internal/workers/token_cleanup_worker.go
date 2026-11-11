package workers

import (
	"context"
	"log"
	"time"

	"task-manager/internal/repositories"
)

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
				deletedCount, err := worker.tokenRepository.DeleteExpiredTokens(ctx)
				if err != nil {
					log.Printf("[Cleanup Worker] Error purging expired tokens: %v\n", err)
				} else if deletedCount > 0 {
					log.Printf("[Cleanup Worker] Purged %d expired revoked tokens\n", deletedCount)
				}
			case <-ctx.Done():
				log.Println("[Cleanup Worker] Stopped periodic token cleanup worker")
				return
			}
		}
	}()
}
