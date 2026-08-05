package customerchat

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// StartBuyerMsgScanner launches the periodic node-draft generator until ctx
// is cancelled. Each tick it scans every tenant that has enabled rules and
// creates missing pending drafts (never sends anything externally).
func StartBuyerMsgScanner(ctx context.Context, wg *sync.WaitGroup, svc *Service, log *slog.Logger, interval time.Duration) {
	if svc == nil || svc.DB == nil {
		return
	}
	if interval <= 0 {
		interval = 60 * time.Second
	}
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		tick := time.NewTicker(interval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				scanBuyerMsgOnce(ctx, svc, log)
			}
		}
	}()
}

func scanBuyerMsgOnce(ctx context.Context, svc *Service, log *slog.Logger) {
	tids, err := svc.BuyerMsgTenantIDs(ctx)
	if err != nil {
		if log != nil {
			log.Warn("buyer message scan: list tenants failed", "error", err)
		}
		return
	}
	for _, tid := range tids {
		created, err := svc.GenerateBuyerMsgDrafts(ctx, tid)
		if err != nil {
			if log != nil {
				log.Warn("buyer message scan: generate failed", "tenantId", tid, "error", err)
			}
			continue
		}
		if created > 0 && log != nil {
			log.Info("buyer message scan: drafts created", "tenantId", tid, "created", created)
		}
	}
}
