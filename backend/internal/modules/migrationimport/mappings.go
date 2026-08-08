package migrationimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

// MappingPresetBody is the payload of POST /imports/mappings.
type MappingPresetBody struct {
	Kind    string         `json:"kind"`
	Name    string         `json:"name"`
	Columns []string       `json:"columns"`
	Mapping map[string]int `json:"mapping"`
}

// SaveMappingPreset upserts one tenant-level mapping preset (kind + name is
// the identity; saving the same name overwrites the previous scheme).
func (s *Service) SaveMappingPreset(c *gin.Context, body MappingPresetBody, adminID *uuid.UUID) (*ImportMappingPreset, error) {
	kind, err := normalizeKind(body.Kind)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		return nil, fmt.Errorf("方案名称不能为空")
	}
	if len([]rune(name)) > 64 {
		return nil, fmt.Errorf("方案名称最长 64 字")
	}
	if len(body.Mapping) == 0 {
		return nil, fmt.Errorf("字段映射（mapping）不能为空")
	}
	if len(body.Mapping) > MaxMappingColumns || len(body.Columns) > MaxMappingColumns {
		return nil, fmt.Errorf("映射方案最多支持 %d 列", MaxMappingColumns)
	}
	for key, idx := range body.Mapping {
		if idx < 0 || idx >= MaxMappingColumns {
			return nil, fmt.Errorf("字段 %s 的列序号超出范围（0-%d）", key, MaxMappingColumns-1)
		}
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	mappingJSON, err := json.Marshal(body.Mapping)
	if err != nil {
		return nil, err
	}
	columnsJSON, err := json.Marshal(body.Columns)
	if err != nil {
		return nil, err
	}
	var preset ImportMappingPreset
	err = s.DB.WithContext(c.Request.Context()).Transaction(func(tx *gorm.DB) error {
		ferr := tx.Where("tenant_id = ? AND kind = ? AND name = ?", tid, kind, name).First(&preset).Error
		if ferr != nil && !errors.Is(ferr, gorm.ErrRecordNotFound) {
			return ferr
		}
		if errors.Is(ferr, gorm.ErrRecordNotFound) {
			var cnt int64
			if err := tx.Model(&ImportMappingPreset{}).
				Where("tenant_id = ? AND kind = ?", tid, kind).Count(&cnt).Error; err != nil {
				return err
			}
			if cnt >= MaxMappingPresetsPerKind {
				return fmt.Errorf("每类导入最多保存 %d 个映射方案", MaxMappingPresetsPerKind)
			}
			preset = ImportMappingPreset{TenantID: tid, Kind: kind, Name: name, CreatedBy: adminID}
		}
		preset.Mapping = datatypes.JSON(mappingJSON)
		preset.Columns = datatypes.JSON(columnsJSON)
		return tx.Save(&preset).Error
	})
	if err != nil {
		return nil, err
	}
	return &preset, nil
}

// MaxMappingPresetsPerKind bounds saved mapping schemes per tenant + kind.
const MaxMappingPresetsPerKind = 50

// MaxMappingColumns bounds the column count and the column indices a mapping
// preset may reference. Import files never have anywhere near this many
// columns; the bound keeps stored presets from carrying arbitrary indices.
const MaxMappingColumns = 200

// ListMappingPresets returns the tenant's mapping presets for one kind.
func (s *Service) ListMappingPresets(c *gin.Context, kind string) ([]ImportMappingPreset, error) {
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return nil, err
	}
	var out []ImportMappingPreset
	if err := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ? AND kind = ?", tid, k).
		Order("updated_at DESC").Limit(MaxMappingPresetsPerKind).Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteMappingPreset removes one preset of the request tenant.
func (s *Service) DeleteMappingPreset(c *gin.Context, id uuid.UUID) error {
	tid, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		return err
	}
	res := s.DB.WithContext(c.Request.Context()).
		Where("tenant_id = ?", tid).Delete(&ImportMappingPreset{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// ListMappings GET /imports/mappings?kind=.
func (h *Handler) ListMappings(c *gin.Context) {
	out, err := h.Svc.ListMappingPresets(c, c.Query("kind"))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	if out == nil {
		out = []ImportMappingPreset{}
	}
	response.OK(c, gin.H{"list": out})
}

// SaveMapping POST /imports/mappings.
func (h *Handler) SaveMapping(c *gin.Context) {
	var body MappingPresetBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid body")
		return
	}
	out, err := h.Svc.SaveMappingPreset(c, body, adminUUID(c))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, out)
}

// DeleteMapping DELETE /imports/mappings/:id.
func (h *Handler) DeleteMapping(c *gin.Context) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return
	}
	if err := h.Svc.DeleteMappingPreset(c, id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.Fail(c, 404, response.CodeNotFound, "映射方案不存在")
			return
		}
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
