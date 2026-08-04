package auth

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"github.com/trademind-ai/trademind/backend/internal/providers/email"
	"github.com/trademind-ai/trademind/backend/internal/providers/email/smtp"
	"gorm.io/gorm"
)

// ipHourlyEmailCodeLimit caps registration codes requested from one client
// address per hour, on top of the per-address hourly limit.
const ipHourlyEmailCodeLimit = 20

// errEmailSettingsIncomplete marks a send attempt with no usable SMTP
// configuration, so the handler can return actionable guidance instead of a
// generic send failure.
var errEmailSettingsIncomplete = errors.New("email settings incomplete")

const (
	msgEmailVerifyDisabled     = "当前部署已关闭注册邮箱验证，无需获取验证码，直接提交注册即可。"
	msgEmailSettingsIncomplete = "邮件服务未配置，无法发送注册验证码。请管理员在「设置 → 邮件设置」完成 SMTP 配置；本地/自托管部署可通过环境变量 AUTH_REGISTER_SKIP_EMAIL_VERIFY=true 显式关闭注册邮箱验证（仅限非生产环境）。"
)

type sendEmailCodeBody struct {
	Email string `json:"email" binding:"required,email"`
	Scene string `json:"scene" binding:"required,oneof=register"`
}

func (h *Handler) SendEmailCode(c *gin.Context) {
	if h.Redis == nil {
		response.Fail(c, 503, response.CodeInternalError, "redis unavailable")
		return
	}
	if h.Cfg != nil && h.Cfg.RegisterEmailVerifyDisabled() {
		response.Fail(c, 400, response.CodeBadRequest, msgEmailVerifyDisabled)
		return
	}
	var body sendEmailCodeBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	emailAddr := strings.ToLower(strings.TrimSpace(body.Email))

	// Missing SMTP configuration is reported before the address is looked at,
	// so the guidance response never distinguishes registered addresses.
	smtpCfg, err := h.smtpConfig(c.Request.Context())
	if err != nil {
		if errors.Is(err, errEmailSettingsIncomplete) {
			response.Fail(c, 503, response.CodeServiceUnavailable, msgEmailSettingsIncomplete)
			return
		}
		response.Fail(c, 500, response.CodeInternalError, "email settings error")
		return
	}

	// Per-IP budget blunts bulk registration: one client cannot walk a list of
	// addresses even though each address has its own hourly budget below.
	ipKey := fmt.Sprintf("email_code_ip_hourly:%s:%s", body.Scene, c.ClientIP())
	ipCount, _ := h.Redis.Get(c.Request.Context(), ipKey).Int()
	if ipCount >= ipHourlyEmailCodeLimit {
		response.Fail(c, 429, response.CodeBadRequest, "hourly limit reached")
		return
	}

	// Check rate limit: 60s cooldown
	cooldownKey := fmt.Sprintf("email_code_cooldown:%s:%s", body.Scene, emailAddr)
	exists, err := h.Redis.Exists(c.Request.Context(), cooldownKey).Result()
	if err != nil {
		response.Fail(c, 500, response.CodeInternalError, "redis error")
		return
	}
	if exists > 0 {
		response.Fail(c, 429, response.CodeBadRequest, "please wait before sending again")
		return
	}

	// Check rate limit: hourly limit
	hourlyKey := fmt.Sprintf("email_code_hourly:%s:%s", body.Scene, emailAddr)
	count, _ := h.Redis.Get(c.Request.Context(), hourlyKey).Int()
	if count >= 5 {
		response.Fail(c, 429, response.CodeBadRequest, "hourly limit reached")
		return
	}

	// Registered addresses must not be distinguishable from new ones: the
	// budget is consumed and the response is byte-identical to a real send,
	// only no code is generated or stored (registration would reject it
	// anyway). The skipped send stays visible in the operation log.
	if body.Scene == "register" {
		_, err := h.Admins.ByEmail(c.Request.Context(), emailAddr)
		if err != nil && err != gorm.ErrRecordNotFound {
			response.Fail(c, 500, response.CodeInternalError, "database error")
			return
		}
		if err == nil {
			h.consumeEmailCodeBudget(c, cooldownKey, hourlyKey, ipKey, count, ipCount)
			if h.OpLog != nil {
				_ = h.OpLog.Write(c, operationlog.WriteOpts{
					Username: emailAddr,
					Action:   "email_code.send",
					Resource: "auth",
					Status:   "skipped",
					Message:  "email already registered",
				})
			}
			response.OK(c, gin.H{"ok": true})
			return
		}
	}

	// Generate code
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	code := fmt.Sprintf("%06d", n.Int64())

	// Send email. Transient SMTP failures still answer with the same 200 body
	// as a successful send: an error here only happens for unregistered
	// addresses, so any distinct status would re-open account enumeration.
	// The failure stays visible in the operation log and the code is not
	// stored, so the address simply retries after the cooldown.
	if err := h.sendCodeEmail(c.Request.Context(), smtpCfg, emailAddr, code); err != nil {
		if h.OpLog != nil {
			_ = h.OpLog.Write(c, operationlog.WriteOpts{
				Username: emailAddr,
				Action:   "email_code.send",
				Resource: "auth",
				Status:   "failed",
				Message:  "failed to send email",
			})
		}
		h.consumeEmailCodeBudget(c, cooldownKey, hourlyKey, ipKey, count, ipCount)
		response.OK(c, gin.H{"ok": true})
		return
	}

	// Save to Redis
	codeKey := fmt.Sprintf("email_code:%s:%s", body.Scene, emailAddr)
	h.Redis.Set(c.Request.Context(), codeKey, code, 10*time.Minute)
	h.consumeEmailCodeBudget(c, cooldownKey, hourlyKey, ipKey, count, ipCount)

	if h.OpLog != nil {
		_ = h.OpLog.Write(c, operationlog.WriteOpts{
			Username: emailAddr,
			Action:   "email_code.send",
			Resource: "auth",
			Status:   "success",
		})
	}

	response.OK(c, gin.H{"ok": true})
}

// consumeEmailCodeBudget arms the per-address cooldown and increments the
// per-address and per-IP hourly counters.
func (h *Handler) consumeEmailCodeBudget(c *gin.Context, cooldownKey, hourlyKey, ipKey string, count, ipCount int) {
	ctx := c.Request.Context()
	h.Redis.Set(ctx, cooldownKey, "1", 60*time.Second)
	if count == 0 {
		h.Redis.Set(ctx, hourlyKey, 1, time.Hour)
	} else {
		h.Redis.Incr(ctx, hourlyKey)
	}
	if ipCount == 0 {
		h.Redis.Set(ctx, ipKey, 1, time.Hour)
	} else {
		h.Redis.Incr(ctx, ipKey)
	}
}

// smtpConfig resolves the deployment SMTP configuration and reports
// errEmailSettingsIncomplete when it is unusable.
func (h *Handler) smtpConfig(ctx context.Context) (smtp.Config, error) {
	m, err := h.Settings.PlainMailSettings(ctx)
	if err != nil {
		return smtp.Config{}, err
	}
	providerStr := strings.TrimSpace(m["provider"])
	if providerStr != "" && providerStr != "smtp" {
		return smtp.Config{}, fmt.Errorf("unsupported email provider %q", providerStr)
	}
	port, _ := strconv.Atoi(m["smtp_port"])
	cfg := smtp.Config{
		Host:     m["smtp_host"],
		Port:     port,
		Username: m["smtp_username"],
		Password: m["smtp_password"],
		FromName: m["smtp_from_name"],
		From:     m["smtp_from"],
		UseTLS:   m["smtp_use_tls"] == "true",
		UseSSL:   m["smtp_use_ssl"] == "true",
	}
	if cfg.Host == "" || cfg.From == "" {
		return smtp.Config{}, errEmailSettingsIncomplete
	}
	return cfg, nil
}

func (h *Handler) sendCodeEmail(ctx context.Context, cfg smtp.Config, to, code string) error {
	p := smtp.NewProvider(cfg)
	return p.Send(ctx, email.SendEmailRequest{
		To:      to,
		Subject: "Your Verification Code - TradeMind",
		Content: fmt.Sprintf("Your verification code is: %s. It will expire in 10 minutes.", code),
	})
}
