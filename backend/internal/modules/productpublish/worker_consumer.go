package productpublish

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

func StartWorker(ctx context.Context, wg *sync.WaitGroup, log *slog.Logger, svc *Service, queueName string, concurrency int, reg *worker.Registry) {
	if svc == nil || svc.Redis == nil || svc.Redis.Client == nil {
		return
	}
	if queueName == "" {
		queueName = "product:publish:tasks"
	}
	if concurrency < 1 {
		concurrency = 1
	}
	SetProductPublishWorkersRunning(true)
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			var wid string
			if reg != nil {
				inst := reg.Register(ctx, worker.TypeProductPublish, fmt.Sprintf("product-publish-%d", slot), map[string]any{"queue": queueName})
				if inst != nil {
					defer inst.Stop(context.Background())
					wid = inst.WorkerID()
				}
			}
			if wid == "" {
				wid = worker.GenerateWorkerID(worker.TypeProductPublish)
			}
			runPublishWorkerLoop(ctx, log, svc, queueName, slot, wid)
		}(i + 1)
	}
}

func runPublishWorkerLoop(ctx context.Context, log *slog.Logger, svc *Service, queueName string, slot int, workerLeaseID string) {
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
		payload := res[1]
		var msg productPublishQueueMsg
		if err := json.Unmarshal([]byte(payload), &msg); err != nil {
			if log != nil {
				log.Warn("product_publish_worker_bad_message", "worker", slot, "error", err)
			}
			continue
		}
		tid, err := uuid.Parse(strings.TrimSpace(msg.TaskID))
		if err != nil {
			if log != nil {
				log.Warn("product_publish_worker_bad_task_id", "worker", slot, "error", err)
			}
			continue
		}
		jobCtx := context.Background()
		if svc.DB != nil {
			var probe ProductPublishTask
			if err := svc.DB.WithContext(jobCtx).Select("shop_id, tenant_id").First(&probe, "id = ?", tid).Error; err == nil {
				wctx, _, terr := tasktenant.BeginWorker(jobCtx, svc.DB, probe.TenantID, probe.ShopID, "product_publish")
				if terr != nil {
					if probe.TenantID == 0 && svc.AllowTenantZeroTasks {
						// Platform-tenant demo seed task: process under the demo
						// gate with an explicit tenant-0 worker context. Publish
						// capability limits (e.g. local_draft_only) still apply.
						jobCtx = tasktenant.BuildWorkerContext(tasktenant.TaskScope{TenantID: 0, ShopID: probe.ShopID}, uuid.Nil, "product_publish")
					} else {
						if log != nil {
							log.Warn("product_publish_worker_tenant_missing", "worker", slot, "taskId", tid.String(), "error", tasktenant.WrapError(terr))
						}
						svc.failTaskTenantGate(jobCtx, tid, tasktenant.WrapError(terr))
						continue
					}
				} else {
					jobCtx = wctx
				}
			}
		}
		if err := svc.ProcessQueuedTask(jobCtx, tid, workerLeaseID); err != nil && log != nil {
			log.Warn("product_publish_worker_task_error", "worker", slot, "taskId", tid.String(), "error", err)
		}
	}
}

// failTaskTenantGate marks a task rejected by the worker tenant gate as
// failed with a user-visible reason, instead of dropping it silently and
// leaving it pending forever.
func (svc *Service) failTaskTenantGate(ctx context.Context, taskID uuid.UUID, reason string) {
	if svc == nil || svc.DB == nil {
		return
	}
	fin := time.Now().UTC()
	_ = svc.DB.WithContext(ctx).Model(&ProductPublishTask{}).
		Where("id = ? AND status IN ?", taskID, []string{TaskPending, TaskRunning}).
		Updates(map[string]any{
			"status":         TaskFailed,
			"publish_status": StatusPubFailed,
			"error_code":     "task_tenant_missing",
			"error_message":  reason,
			"finished_at":    &fin,
			"locked_by":      nil,
			"locked_until":   nil,
			"updated_at":     fin,
		}).Error
	var task ProductPublishTask
	if err := svc.DB.WithContext(ctx).First(&task, "id = ?", taskID).Error; err == nil {
		if rid, ok := snapshotPublicationFromTask(&task); ok {
			_ = svc.DB.WithContext(ctx).Model(&ProductPublication{}).Where("id = ?", rid).
				Updates(map[string]any{
					"status":         StatusPubFailed,
					"publish_status": StatusPubFailed,
					"updated_at":     fin,
				}).Error
		}
	}
}
