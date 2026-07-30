package selection

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// DefaultQueueName is the Redis list key for selection job payloads.
const DefaultQueueName = "selection:tasks"

// QueueMessage is one selection job payload.
type QueueMessage struct {
	TaskID string `json:"taskId"`
}

func (s *Service) enqueueTask(ctx context.Context, taskID uuid.UUID) error {
	if s == nil || s.Redis == nil || s.Redis.Client == nil {
		return ErrQueueUnavailable
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.Redis.Ping(ctx).Err(); err != nil {
		return ErrQueueUnavailable
	}
	payload, err := json.Marshal(QueueMessage{TaskID: taskID.String()})
	if err != nil {
		return fmt.Errorf("selection: marshal queue message: %w", err)
	}
	name := s.QueueName
	if name == "" {
		name = DefaultQueueName
	}
	if err := s.Redis.LPush(ctx, name, payload).Err(); err != nil {
		return ErrQueueUnavailable
	}
	return nil
}
