package subscription

import (
	"context"
	"time"

	"codebase/pkg/logger"
)

type renewer interface {
	RenewDue() (int, error)
}

// StartRenewalJob runs RenewDue now and then every interval until ctx is cancelled. The returned channel
// closes when the job has stopped.
// ponytail: in-process ticker; safe with several instances (conditional updates) but each one scans —
// move to a single scheduled worker if instances multiply.
func StartRenewalJob(ctx context.Context, r renewer, every time.Duration, log logger.Logger) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			n, err := r.RenewDue()
			if log != nil {
				if err != nil {
					log.Errorf("subscription renewal: renewed=%d err=%v", n, err)
				} else if n > 0 {
					log.Info("subscription renewal: renewed ", n)
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return done
}
