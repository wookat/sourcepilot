package selection

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/trademind-ai/trademind/backend/internal/modules/worker"
	"github.com/trademind-ai/trademind/backend/internal/pkg/tasktenant"
)

// StartWorker launches Redis queue consumers for selection tasks.
func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if queueName == "" {
		queueName = DefaultQueueName
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 8 {
		concurrency = 8
	}

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			var wid string
			if reg != nil {
				inst := reg.Register(ctx, "selection", fmt.Sprintf("selection-%d", slot), map[string]any{"queue": queueName})
				if inst != nil {
					defer inst.Stop(context.Background())
					wid = inst.WorkerID()
				}
			}
			if wid == "" {
				wid = worker.GenerateWorkerID("selection")
			}
			runSelectionWorker(ctx, log, svc, queueName, slot, wid)
		}(i + 1)
	}
}

func runSelectionWorker(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerID string) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		res, err := svc.Redis.BRPop(ctx, 5*time.Second, queueName).Result()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			continue
		}
		if len(res) < 2 {
			continue
		}

		var msg QueueMessage
		if err := json.Unmarshal([]byte(res[1]), &msg); err != nil {
			if log != nil {
				log.Warn("selection_worker_bad_message", "worker", slot, "error", err)
			}
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("selection_worker_bad_task_id", "worker", slot, "error", err)
			}
			continue
		}

		jobCtx := context.Background()
		if svc.DB != nil {
			var probe SelectionTask
			if err := svc.DB.WithContext(jobCtx).Select("tenant_id").First(&probe, "id = ?", tid).Error; err == nil {
				wctx, _, terr := tasktenant.BeginWorker(jobCtx, svc.DB, probe.TenantID, uuid.Nil, "selection")
				if terr != nil {
					if log != nil {
						log.Warn("selection_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", tasktenant.WrapError(terr))
					}
					continue
				}
				jobCtx = wctx
			}
		}
		svc.RunSelectionJob(jobCtx, tid, workerID)
	}
}
