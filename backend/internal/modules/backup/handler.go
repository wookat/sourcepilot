package backup

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/operationlog"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
	"gorm.io/gorm"
)

type Handler struct {
	Svc *Service
}

func Register(r gin.IRouter, h *Handler) {
	if r == nil || h == nil {
		return
	}
	g := r.Group("/ops/backups")
	g.GET("", h.List)
	g.POST("", h.Create)
	g.GET("/:id", h.Get)
	g.GET("/:id/download", h.Download)
	g.POST("/:id/verify", h.Verify)
	g.POST("/:id/upload", h.Upload)
	g.POST("/:id/hold", h.Hold)
	g.DELETE("/:id", h.Delete)
}

func (h *Handler) List(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupRead) {
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	items, total, err := h.Svc.List(c.Request.Context(), page, pageSize)
	if err != nil {
		response.HandleError(c, err)
		return
	}
	response.OK(c, gin.H{"items": items, "total": total, "page": page, "pageSize": pageSize})
}

func (h *Handler) Create(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupCreate) {
		return
	}
	var req CreateRequest
	_ = c.ShouldBindJSON(&req)
	row, err := h.Svc.CreateDatabaseBackup(c.Request.Context(), req, currentAdminID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Get(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupRead) {
		return
	}
	row, err := h.Svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusNotFound, response.CodeNotFound, "backup not found")
		return
	}
	response.OK(c, row)
}

func (h *Handler) Verify(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupVerify) {
		return
	}
	row, err := h.Svc.Verify(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Download(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupDownload) {
		return
	}
	backupID := c.Param("id")
	row, artifact, err := h.Svc.Download(c.Request.Context(), backupID)
	if err != nil {
		h.logDownload(c, backupID, "failed", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) || !validID(backupID) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "backup not found")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	h.logDownload(c, row.BackupID, "success", "backup artifact downloaded")
	c.FileAttachment(artifact.LocalPath, artifact.Name)
}

func (h *Handler) logDownload(c *gin.Context, backupID, status, msg string) {
	if h.Svc == nil || h.Svc.OpLog == nil {
		return
	}
	_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
		Action:     "backup.download",
		Resource:   "backup",
		ResourceID: backupID,
		Permission: adminperm.PermBackupDownload,
		Status:     status,
		Message:    msg,
	})
}

// Upload retries pushing a completed backup artifact to object storage.
func (h *Handler) Upload(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupCreate) {
		return
	}
	backupID := c.Param("id")
	row, err := h.Svc.RetryUpload(c.Request.Context(), backupID)
	if err != nil {
		h.logUpload(c, backupID, "failed", err.Error())
		if errors.Is(err, gorm.ErrRecordNotFound) || !validID(backupID) {
			response.Fail(c, http.StatusNotFound, response.CodeNotFound, "backup not found")
			return
		}
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	h.logUpload(c, row.BackupID, "success", "backup artifact uploaded to object storage")
	response.OK(c, row)
}

func (h *Handler) logUpload(c *gin.Context, backupID, status, msg string) {
	if h.Svc == nil || h.Svc.OpLog == nil {
		return
	}
	_ = h.Svc.OpLog.Write(c, operationlog.WriteOpts{
		Action:     "backup.upload",
		Resource:   "backup",
		ResourceID: backupID,
		Permission: adminperm.PermBackupCreate,
		Status:     status,
		Message:    msg,
	})
}

func (h *Handler) Hold(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupHold) {
		return
	}
	var req HoldRequest
	_ = c.ShouldBindJSON(&req)
	row, err := h.Svc.Hold(c.Request.Context(), c.Param("id"), req, currentAdminID(c))
	if err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, row)
}

func (h *Handler) Delete(c *gin.Context) {
	if !adminperm.RequirePermission(c, h.Svc.DB, adminperm.PermBackupDelete) {
		return
	}
	if err := h.Svc.Delete(c.Request.Context(), c.Param("id")); err != nil {
		response.Fail(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}

func currentAdminID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if id, err := uuid.Parse(s); err == nil {
				return &id
			}
		}
	}
	return nil
}
