package model

import (
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const (
	TenantStatusEnabled  = 1
	TenantStatusDisabled = 2
)

type Tenant struct {
	ID          string `json:"id" gorm:"primaryKey;type:varchar(64)"`
	Name        string `json:"name" gorm:"type:varchar(128);not null"`
	Status      int    `json:"status" gorm:"type:int;default:1;index"`
	CreatedTime int64  `json:"created_time" gorm:"bigint;index"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

type TenantAuditLog struct {
	Id         int    `json:"id"`
	TenantID   string `json:"tenant_id" gorm:"type:varchar(64);index"`
	UserID     int    `json:"user_id" gorm:"index"`
	Action     string `json:"action" gorm:"type:varchar(16);index"`
	Resource   string `json:"resource" gorm:"type:varchar(128);index"`
	ResourceID string `json:"resource_id" gorm:"type:varchar(64)"`
	Detail     string `json:"detail" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"bigint;index"`
}

func (t *Tenant) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(t.ID) == "" {
		t.ID = common.GetDefaultTenantID()
	}
	now := common.GetTimestamp()
	if t.CreatedTime == 0 {
		t.CreatedTime = now
	}
	t.UpdatedTime = now
	if t.Status == 0 {
		t.Status = TenantStatusEnabled
	}
	return nil
}

func (t *Tenant) BeforeUpdate(tx *gorm.DB) error {
	t.UpdatedTime = common.GetTimestamp()
	return nil
}

func (a *TenantAuditLog) BeforeCreate(tx *gorm.DB) error {
	if strings.TrimSpace(a.TenantID) == "" {
		a.TenantID = common.GetDefaultTenantID()
	}
	if a.CreatedAt == 0 {
		a.CreatedAt = common.GetTimestamp()
	}
	return nil
}

func EnsureDefaultTenant() error {
	if !common.MultiTenantEnabled {
		return nil
	}
	defaultID := common.GetDefaultTenantID()
	tenant := Tenant{
		ID:     defaultID,
		Name:   "Default Tenant",
		Status: TenantStatusEnabled,
	}
	return DB.Where("id = ?", defaultID).FirstOrCreate(&tenant).Error
}

func CreateTenantAuditLog(log *TenantAuditLog) error {
	if log == nil {
		return nil
	}
	return DB.Create(log).Error
}
