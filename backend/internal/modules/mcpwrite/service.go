package mcpwrite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/mcpaudit"
	"gorm.io/gorm"
)

// Write quotas (D4). Counted from successful execute audit rows, so the
// quota and the audit trail can never disagree; a counting failure closes
// the pipeline.
const (
	// PerTokenHourlyLimit caps successful executes per token per hour.
	PerTokenHourlyLimit = 30
	// PerTenantDailyLimit caps successful executes per tenant per day.
	PerTenantDailyLimit = 200
)

// Modes of one write tool call.
const (
	ModeDryRun  = "dry_run"
	ModeExecute = "execute"
)

// Pipeline rejections.
var (
	ErrInvalidMode      = errors.New("mcp write: mode 必须为 dry_run 或 execute")
	ErrQuotaToken       = errors.New("mcp write: 该 token 本小时写执行次数已达上限（30 次/小时）")
	ErrQuotaTenant      = errors.New("mcp write: 该租户今日写执行次数已达上限（200 次/天）")
	ErrAuditUnavailable = errors.New("mcp write: audit log unavailable, tool call rejected")
)

// Caller identifies the authenticated MCP token making the write call.
type Caller struct {
	TenantID    int64
	TokenID     uuid.UUID
	TokenName   string
	TokenMasked string
}

// Request is one write tool invocation handed to the orchestrator. DryRun
// must be side-effect free (validation + impact preview); Execute performs
// the mutation on the supplied transaction handle so it commits atomically
// with its audit row.
type Request struct {
	Caller Caller
	Tool   string
	Mode   string
	// ConfirmationToken is required for execute.
	ConfirmationToken string
	// ParamsCanonical is a canonical serialization of the action parameters
	// (mode / confirmation excluded); its hash binds dry_run to execute.
	ParamsCanonical string
	// ParamsSummary is the allowlisted key=value audit summary of the params.
	ParamsSummary string
	// Amount is the money amount of an amount-bearing action
	// (procurement_mark_paid); recorded on the audit rows so the tenant
	// daily amount ceiling sums from the same fail-closed trail. Zero for
	// every other tool.
	Amount  float64
	DryRun  func(ctx context.Context, db *gorm.DB) (preview any, summary string, err error)
	Execute func(ctx context.Context, tx *gorm.DB) (result any, summary string, err error)
}

// Result is the structured outcome of one write tool call.
type Result struct {
	Mode    string `json:"mode"`
	Summary string `json:"summary"`
	// Dry-run fields.
	Preview               any    `json:"preview,omitempty"`
	ConfirmationToken     string `json:"confirmationToken,omitempty"`
	ConfirmationExpiresAt string `json:"confirmationExpiresAt,omitempty"`
	// Execute fields.
	Result          any  `json:"result,omitempty"`
	AlreadyExecuted bool `json:"alreadyExecuted,omitempty"`
	// Remaining quota after this call (informational).
	QuotaRemainingToken  int64 `json:"quotaRemainingToken"`
	QuotaRemainingTenant int64 `json:"quotaRemainingTenant"`
}

// Service orchestrates the governed write pipeline for every whitelisted
// write tool.
type Service struct {
	DB     *gorm.DB
	Gate   *Gate
	Audits *mcpaudit.Service
}

// paramsHash binds a confirmation to one tool + parameter set.
func paramsHash(tool, canonical string) string {
	sum := sha256.Sum256([]byte(tool + "\n" + canonical))
	return hex.EncodeToString(sum[:])
}

// auditReject records a rejected write call (best effort: the call is
// already refused, so a failed audit write cannot grant anything).
func (s *Service) auditReject(ctx context.Context, req Request, mode string, start time.Time, reason string) {
	if s == nil || s.Audits == nil {
		return
	}
	if err := s.Audits.Write(ctx, mcpaudit.WriteOpts{
		TenantID:      req.Caller.TenantID,
		TokenID:       req.Caller.TokenID,
		TokenName:     req.Caller.TokenName,
		TokenMasked:   req.Caller.TokenMasked,
		Tool:          req.Tool,
		Status:        mcpaudit.StatusError,
		Mode:          mode,
		ParamsSummary: req.ParamsSummary,
		ResultSummary: reason,
		Amount:        req.Amount,
		DurationMs:    time.Since(start).Milliseconds(),
	}); err != nil {
		slog.Error("mcp_write_audit_reject_failed", "tool", req.Tool, "error", err.Error())
	}
}

// quotaUsage returns current usage; any counting error fails closed.
func (s *Service) quotaUsage(db *gorm.DB, c Caller) (tokenUsed, tenantUsed int64, err error) {
	now := time.Now().UTC()
	tokenUsed, err = s.Audits.CountExecutesByToken(db, c.TokenID, now.Add(-time.Hour))
	if err != nil {
		return 0, 0, fmt.Errorf("mcp write: quota check failed (fail closed): %w", err)
	}
	tenantUsed, err = s.Audits.CountExecutesByTenant(db, c.TenantID, now.Add(-24*time.Hour))
	if err != nil {
		return 0, 0, fmt.Errorf("mcp write: quota check failed (fail closed): %w", err)
	}
	return tokenUsed, tenantUsed, nil
}

// Run executes one write tool call through the full pipeline: gates →
// (dry_run: validate → issue confirmation + audit in one tx) or (execute:
// consume confirmation → quota + mutation + audit in one tx).
func (s *Service) Run(ctx context.Context, req Request) (*Result, error) {
	start := time.Now()
	if s == nil || s.DB == nil || s.Audits == nil {
		return nil, ErrAuditUnavailable
	}
	mode := req.Mode
	if mode != ModeDryRun && mode != ModeExecute {
		s.auditReject(ctx, req, "", start, "invalid mode")
		return nil, ErrInvalidMode
	}
	if err := s.Gate.Check(ctx, req.Caller.TenantID); err != nil {
		s.auditReject(ctx, req, mode, start, "gate closed")
		return nil, err
	}
	ph := paramsHash(req.Tool, req.ParamsCanonical)
	if mode == ModeDryRun {
		return s.runDryRun(ctx, req, ph, start)
	}
	return s.runExecute(ctx, req, ph, start)
}

func (s *Service) runDryRun(ctx context.Context, req Request, ph string, start time.Time) (*Result, error) {
	db := s.DB.WithContext(ctx)
	tokenUsed, tenantUsed, err := s.quotaUsage(db, req.Caller)
	if err != nil {
		s.auditReject(ctx, req, ModeDryRun, start, "quota check failed")
		return nil, err
	}
	if tokenUsed >= PerTokenHourlyLimit {
		s.auditReject(ctx, req, ModeDryRun, start, "token quota exhausted")
		return nil, ErrQuotaToken
	}
	if tenantUsed >= PerTenantDailyLimit {
		s.auditReject(ctx, req, ModeDryRun, start, "tenant quota exhausted")
		return nil, ErrQuotaTenant
	}
	preview, summary, err := req.DryRun(ctx, db)
	if err != nil {
		s.auditReject(ctx, req, ModeDryRun, start, "validation failed: "+err.Error())
		return nil, err
	}
	var plain string
	var conf *Confirmation
	// Confirmation and its dry_run audit row commit atomically: no
	// confirmation can exist without an audit trail (fail closed).
	err = db.Transaction(func(tx *gorm.DB) error {
		var terr error
		plain, conf, terr = issueConfirmation(tx, req.Caller.TenantID, req.Caller.TokenID, req.Tool, ph)
		if terr != nil {
			return terr
		}
		return s.Audits.WriteTx(tx, mcpaudit.WriteOpts{
			TenantID:      req.Caller.TenantID,
			TokenID:       req.Caller.TokenID,
			TokenName:     req.Caller.TokenName,
			TokenMasked:   req.Caller.TokenMasked,
			Tool:          req.Tool,
			Status:        mcpaudit.StatusSuccess,
			Mode:          mcpaudit.ModeDryRun,
			ParamsSummary: req.ParamsSummary,
			ResultSummary: summary,
			ConfirmHash:   conf.ConfirmHash,
			Amount:        req.Amount,
			DurationMs:    time.Since(start).Milliseconds(),
		})
	})
	if err != nil {
		slog.Error("mcp_write_dryrun_audit_failed", "tool", req.Tool, "error", err.Error())
		return nil, ErrAuditUnavailable
	}
	return &Result{
		Mode:                  ModeDryRun,
		Summary:               summary,
		Preview:               preview,
		ConfirmationToken:     plain,
		ConfirmationExpiresAt: conf.ExpiresAt.UTC().Format(time.RFC3339),
		QuotaRemainingToken:   PerTokenHourlyLimit - tokenUsed,
		QuotaRemainingTenant:  PerTenantDailyLimit - tenantUsed,
	}, nil
}

func (s *Service) runExecute(ctx context.Context, req Request, ph string, start time.Time) (*Result, error) {
	if req.ConfirmationToken == "" {
		s.auditReject(ctx, req, ModeExecute, start, "confirmation missing")
		return nil, ErrConfirmationRequired
	}
	db := s.DB.WithContext(ctx)
	// The consume commits on its own first: any later failure burns the
	// confirmation (re-run dry_run) instead of leaving a replayable token.
	outcome, conf, err := consumeConfirmation(ctx, db, req.Caller.TenantID, req.Caller.TokenID, req.Tool, ph, req.ConfirmationToken)
	if err != nil {
		s.auditReject(ctx, req, ModeExecute, start, "confirmation rejected")
		return nil, err
	}
	if outcome == consumeAlreadyExecuted {
		s.auditReject(ctx, req, ModeExecute, start, "already_executed")
		return &Result{
			Mode:            ModeExecute,
			Summary:         "该确认已执行过，本次未重复执行",
			AlreadyExecuted: true,
		}, nil
	}
	var result any
	var summary string
	var remainingToken, remainingTenant int64
	// Quota check, business mutation, audit row and executed-marker commit
	// in one transaction: an audit failure rolls the mutation back.
	err = db.Transaction(func(tx *gorm.DB) error {
		tokenUsed, tenantUsed, qerr := s.quotaUsage(tx, req.Caller)
		if qerr != nil {
			return qerr
		}
		if tokenUsed >= PerTokenHourlyLimit {
			return ErrQuotaToken
		}
		if tenantUsed >= PerTenantDailyLimit {
			return ErrQuotaTenant
		}
		remainingToken = PerTokenHourlyLimit - tokenUsed - 1
		remainingTenant = PerTenantDailyLimit - tenantUsed - 1
		var xerr error
		result, summary, xerr = req.Execute(ctx, tx)
		if xerr != nil {
			return xerr
		}
		if aerr := s.Audits.WriteTx(tx, mcpaudit.WriteOpts{
			TenantID:      req.Caller.TenantID,
			TokenID:       req.Caller.TokenID,
			TokenName:     req.Caller.TokenName,
			TokenMasked:   req.Caller.TokenMasked,
			Tool:          req.Tool,
			Status:        mcpaudit.StatusSuccess,
			Mode:          mcpaudit.ModeExecute,
			ParamsSummary: req.ParamsSummary,
			ResultSummary: summary,
			ConfirmHash:   conf.ConfirmHash,
			Amount:        req.Amount,
			DurationMs:    time.Since(start).Milliseconds(),
		}); aerr != nil {
			return fmt.Errorf("%w: %v", ErrAuditUnavailable, aerr)
		}
		return tx.Model(&Confirmation{}).Where("id = ?", conf.ID).
			Update("executed_at", time.Now().UTC()).Error
	})
	if err != nil {
		if errors.Is(err, ErrAuditUnavailable) {
			slog.Error("mcp_write_execute_audit_failed", "tool", req.Tool, "error", err.Error())
			return nil, ErrAuditUnavailable
		}
		s.auditReject(ctx, req, ModeExecute, start, "execute failed: "+err.Error())
		return nil, err
	}
	return &Result{
		Mode:                 ModeExecute,
		Summary:              summary,
		Result:               result,
		QuotaRemainingToken:  remainingToken,
		QuotaRemainingTenant: remainingTenant,
	}, nil
}
