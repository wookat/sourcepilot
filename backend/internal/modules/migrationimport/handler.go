package migrationimport

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/ctxkey"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// Handler exposes migration import HTTP routes.
type Handler struct {
	Svc *Service
}

func (h *Handler) requireWrite() gin.HandlerFunc {
	return func(c *gin.Context) {
		if h == nil || h.Svc == nil || h.Svc.DB == nil {
			response.Fail(c, 500, response.CodeInternalError, "import unavailable")
			c.Abort()
			return
		}
		p, _ := adminperm.LoadPrincipal(c, h.Svc.DB)
		if p == nil || p.IsReadonly() {
			response.Fail(c, 403, response.CodeReadonlyForbidden, "当前账号为只读权限，无法执行此操作")
			c.Abort()
			return
		}
		c.Next()
	}
}

func adminUUID(c *gin.Context) *uuid.UUID {
	if v, ok := c.Get(ctxkey.AdminID); ok {
		if s, ok := v.(string); ok {
			if u, err := uuid.Parse(strings.TrimSpace(s)); err == nil {
				return &u
			}
		}
	}
	return nil
}

func (h *Handler) failFrom(c *gin.Context, err error) {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		response.Fail(c, 404, response.CodeNotFound, "店铺不存在或不可见")
	case errors.Is(err, errShopNotOperable):
		response.Fail(c, 403, response.CodeStorePermissionDenied, err.Error())
	default:
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
	}
}

// Parse POST /imports/parse — multipart file upload -> columns / rows / guessed mapping.
func (h *Handler) Parse(c *gin.Context) {
	kind, err := normalizeKind(c.PostForm("kind"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	fh, err := c.FormFile("file")
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "file is required")
		return
	}
	if fh.Size > MaxImportFileSize {
		response.Fail(c, 400, response.CodeBadRequest, "文件超过 10MB 限制")
		return
	}
	f, err := fh.Open()
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "文件读取失败")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, MaxImportFileSize+1))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "文件读取失败")
		return
	}
	parsed, err := ParseImportFile(fh.Filename, data)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, &ParseResult{
		Kind:         kind,
		FileName:     fh.Filename,
		FileHash:     parsed.FileHash,
		SourceFormat: DetectSourceFormat(parsed.Columns),
		Columns:      parsed.Columns,
		Rows:         parsed.Rows,
		TotalRows:    len(parsed.Rows),
		Mapping:      GuessMapping(kind, parsed.Columns),
		Fields:       FieldsForKind(kind),
	})
}

// Validate POST /imports/validate.
func (h *Handler) Validate(c *gin.Context) {
	var body WizardBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.Validate(c, body)
	if err != nil {
		h.failFrom(c, err)
		return
	}
	response.OK(c, out)
}

// Commit POST /imports/commit.
func (h *Handler) Commit(c *gin.Context) {
	var body WizardBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.Commit(c, body, adminUUID(c))
	if err != nil {
		h.failFrom(c, err)
		return
	}
	response.OK(c, out)
}

// List GET /imports.
func (h *Handler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	jobs, total, err := h.Svc.ListJobs(c, c.Query("kind"), page, pageSize)
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"list": jobs, "total": total, "page": page, "pageSize": pageSize})
}

// Get GET /imports/:id.
func (h *Handler) Get(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	job, rows, err := h.Svc.GetJob(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "导入任务不存在")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if rows == nil {
		rows = []ImportJobRow{}
	}
	response.OK(c, gin.H{"job": job, "errorRows": rows})
}

// ErrorsCSV GET /imports/:id/errors.csv — downloadable error row report.
func (h *Handler) ErrorsCSV(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	job, rows, err := h.Svc.GetJob(c, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "导入任务不存在")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=import-%s-errors.csv", job.ID.String()[:8]))
	c.Status(http.StatusOK)
	_, _ = c.Writer.Write([]byte{0xEF, 0xBB, 0xBF}) // BOM for Excel
	w := csv.NewWriter(c.Writer)
	_ = w.Write([]string{"行号", "状态", "字段", "错误信息", "原始数据"})
	for _, r := range rows {
		_ = w.Write([]string{
			strconv.Itoa(r.RowNumber), r.Status, r.Field, r.Message, rawValuesText(r),
		})
	}
	w.Flush()
}

func rawValuesText(r ImportJobRow) string {
	if len(r.RawValues) == 0 {
		return ""
	}
	var m map[string]string
	if err := json.Unmarshal(r.RawValues, &m); err != nil {
		return string(r.RawValues)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(m))
	for _, k := range keys {
		if m[k] == "" {
			continue
		}
		parts = append(parts, k+"="+m[k])
	}
	return strings.Join(parts, "; ")
}
