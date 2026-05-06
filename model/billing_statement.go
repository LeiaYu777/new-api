package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

const BillingStatementStatusClosed = "closed"

type BillingStatement struct {
	Id            int    `json:"id"`
	UserId        int    `json:"user_id" gorm:"index:idx_billing_statements_user_period;uniqueIndex:idx_billing_statement_scope,priority:3"`
	Username      string `json:"username" gorm:"type:varchar(191);default:''"`
	PeriodStart   int64  `json:"period_start" gorm:"bigint;index:idx_billing_statements_user_period;index:idx_billing_statements_model_period;uniqueIndex:idx_billing_statement_scope,priority:1"`
	PeriodEnd     int64  `json:"period_end" gorm:"bigint;index:idx_billing_statements_user_period;index:idx_billing_statements_model_period;uniqueIndex:idx_billing_statement_scope,priority:2"`
	ModelName     string `json:"model_name" gorm:"type:varchar(191);default:'';index:idx_billing_statements_model_period;uniqueIndex:idx_billing_statement_scope,priority:4"`
	ChannelId     int    `json:"channel_id" gorm:"default:0;uniqueIndex:idx_billing_statement_scope,priority:5"`
	Group         string `json:"group" gorm:"column:group_name;type:varchar(64);default:'';uniqueIndex:idx_billing_statement_scope,priority:6"`
	BillingSource string `json:"billing_source" gorm:"type:varchar(32);default:'wallet';uniqueIndex:idx_billing_statement_scope,priority:7"`
	ConsumeQuota  int64  `json:"consume_quota" gorm:"bigint;not null;default:0"`
	RefundQuota   int64  `json:"refund_quota" gorm:"bigint;not null;default:0"`
	NetQuota      int64  `json:"net_quota" gorm:"bigint;not null;default:0"`
	RequestCount  int64  `json:"request_count" gorm:"bigint;not null;default:0"`
	RefundCount   int64  `json:"refund_count" gorm:"bigint;not null;default:0"`
	TotalTokens   int64  `json:"total_tokens" gorm:"bigint;not null;default:0"`
	Status        string `json:"status" gorm:"type:varchar(32);not null;default:'closed';index"`
	CreatedAt     int64  `json:"created_at" gorm:"bigint"`
	UpdatedAt     int64  `json:"updated_at" gorm:"bigint"`
}

func (BillingStatement) TableName() string {
	return "billing_statements"
}

func (statement *BillingStatement) BeforeCreate(tx *gorm.DB) error {
	now := common.GetTimestamp()
	if statement.BillingSource == "" {
		statement.BillingSource = billingSourceWallet
	}
	if statement.Status == "" {
		statement.Status = BillingStatementStatusClosed
	}
	statement.CreatedAt = now
	statement.UpdatedAt = now
	return nil
}

func (statement *BillingStatement) BeforeUpdate(tx *gorm.DB) error {
	statement.UpdatedAt = common.GetTimestamp()
	return nil
}

type BillingStatementFilter struct {
	PeriodStart   int64
	PeriodEnd     int64
	ModelName     string
	Username      string
	UserId        int
	Channel       int
	Group         string
	BillingSource string
	Status        string
}

type BillingStatementGenerateResult struct {
	GeneratedCount int                 `json:"generated_count"`
	Statements     []*BillingStatement `json:"statements"`
	Summary        *BillingSummary     `json:"summary"`
}

func applyBillingStatementFilters(tx *gorm.DB, filter BillingStatementFilter, includePeriodRange bool) (*gorm.DB, error) {
	if includePeriodRange {
		if filter.PeriodStart > 0 {
			tx = tx.Where("period_start >= ?", filter.PeriodStart)
		}
		if filter.PeriodEnd > 0 {
			tx = tx.Where("period_end <= ?", filter.PeriodEnd)
		}
	}
	if filter.ModelName != "" {
		modelNamePattern, err := sanitizeLikePattern(filter.ModelName)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("model_name LIKE ? ESCAPE '!'", modelNamePattern)
	}
	if filter.Username != "" {
		tx = tx.Where("username = ?", filter.Username)
	}
	if filter.UserId > 0 {
		tx = tx.Where("user_id = ?", filter.UserId)
	}
	if filter.Channel != 0 {
		tx = tx.Where("channel_id = ?", filter.Channel)
	}
	if filter.Group != "" {
		tx = tx.Where("group_name = ?", filter.Group)
	}
	if filter.BillingSource != "" {
		normalized, err := normalizeBillingSource(filter.BillingSource)
		if err != nil {
			return nil, err
		}
		tx = tx.Where("billing_source = ?", normalized)
	}
	if filter.Status != "" {
		tx = tx.Where("status = ?", filter.Status)
	}
	return tx, nil
}

func GetBillingStatements(filter BillingStatementFilter, startIdx int, num int) (statements []*BillingStatement, total int64, err error) {
	if num <= 0 || num > 100 {
		num = common.ItemsPerPage
	}
	tx := DB.Model(&BillingStatement{})
	tx, err = applyBillingStatementFilters(tx, filter, true)
	if err != nil {
		return nil, 0, err
	}
	if err = tx.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = tx.Order("period_start desc, id desc").Limit(num).Offset(startIdx).Find(&statements).Error
	return statements, total, err
}

func GetUserBillingStatements(userId int, filter BillingStatementFilter, startIdx int, num int) (statements []*BillingStatement, total int64, err error) {
	if userId <= 0 {
		return nil, 0, errors.New("invalid user_id")
	}
	filter.UserId = userId
	filter.Username = ""
	return GetBillingStatements(filter, startIdx, num)
}

func GenerateBillingStatements(filter BillingStatementFilter) (*BillingStatementGenerateResult, error) {
	if filter.PeriodStart <= 0 || filter.PeriodEnd <= filter.PeriodStart {
		return nil, errors.New("invalid billing statement period")
	}
	normalizedSource, err := normalizeBillingSource(filter.BillingSource)
	if err != nil {
		return nil, err
	}
	filter.BillingSource = normalizedSource

	status := filter.Status
	if status == "" {
		status = BillingStatementStatusClosed
	}

	summary, err := GetBillingSummary(
		filter.PeriodStart,
		filter.PeriodEnd,
		filter.ModelName,
		filter.Username,
		filter.UserId,
		filter.Channel,
		filter.Group,
		"",
		filter.BillingSource,
		logSearchCountLimit,
	)
	if err != nil {
		return nil, err
	}

	statements := make([]*BillingStatement, 0, len(summary.Items))
	for _, item := range summary.Items {
		source := item.BillingSource
		if source == "" {
			source = billingSourceWallet
		}
		statements = append(statements, &BillingStatement{
			UserId:        item.UserId,
			Username:      item.Username,
			PeriodStart:   filter.PeriodStart,
			PeriodEnd:     filter.PeriodEnd,
			ModelName:     item.ModelName,
			ChannelId:     item.ChannelId,
			Group:         item.Group,
			BillingSource: source,
			ConsumeQuota:  item.ConsumeQuota,
			RefundQuota:   item.RefundQuota,
			NetQuota:      item.NetQuota,
			RequestCount:  item.RequestCount,
			RefundCount:   item.RefundCount,
			TotalTokens:   item.TotalTokens,
			Status:        status,
		})
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		deleteTx := tx.Where("period_start = ? AND period_end = ?", filter.PeriodStart, filter.PeriodEnd)
		deleteFilter := filter
		deleteFilter.PeriodStart = 0
		deleteFilter.PeriodEnd = 0
		deleteFilter.Status = ""
		var err error
		deleteTx, err = applyBillingStatementFilters(deleteTx, deleteFilter, false)
		if err != nil {
			return err
		}
		if err = deleteTx.Delete(&BillingStatement{}).Error; err != nil {
			return err
		}
		if len(statements) == 0 {
			return nil
		}
		return tx.Create(&statements).Error
	})
	if err != nil {
		return nil, err
	}

	return &BillingStatementGenerateResult{
		GeneratedCount: len(statements),
		Statements:     statements,
		Summary:        summary,
	}, nil
}
