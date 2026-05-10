package models

import (
	"time"
)

type TierConfig struct {
	ID                 uint   `gorm:"primaryKey" json:"id"`
	TierCode           string `gorm:"size:16;uniqueIndex;not null" json:"tier_code"`
	TierName           string `gorm:"size:32;not null" json:"tier_name"`
	MinLifetimePoints  int64  `gorm:"not null;default:0" json:"min_lifetime_points"`
}

func (TierConfig) TableName() string { return "tier_configs" }

type PointRules struct {
	ID                       uint            `gorm:"primaryKey" json:"id"`
	SpendYuanPerPoint        float64         `gorm:"column:spend_yuan_per_point;type:numeric(14,2);not null" json:"spend_yuan_per_point"`
	BirthdayMultiplier       int             `gorm:"column:birthday_multiplier;not null" json:"birthday_multiplier"`
	CampaignMultiplier       int             `gorm:"column:campaign_multiplier;not null" json:"campaign_multiplier"`
	CampaignEnd              *time.Time      `gorm:"column:campaign_end;type:date" json:"campaign_end"`
	ExpiryJanClearAfterYears int             `gorm:"column:expiry_jan_clear_after_years;not null" json:"expiry_jan_clear_after_years"`
	ActiveEventName          string          `gorm:"size:200;not null" json:"active_event_name"`
}

func (PointRules) TableName() string { return "point_rules" }

type Member struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	Phone           string         `gorm:"size:20;uniqueIndex;not null" json:"phone"`
	Name            string         `gorm:"size:100;not null" json:"name"`
	Birthday        *time.Time     `gorm:"type:date" json:"birthday"`
	RegisteredAt    time.Time      `gorm:"type:date;not null" json:"registered_at"`
	TierCode        string         `gorm:"size:16;not null" json:"tier_code"`
	LifetimePoints  int64          `gorm:"not null;default:0" json:"lifetime_points"`
	PointsBalance   int64          `gorm:"not null;default:0" json:"points_balance"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (Member) TableName() string { return "members" }

type PointTransaction struct {
	ID           uint            `gorm:"primaryKey" json:"id"`
	MemberID     uint            `gorm:"column:member_id;not null;index" json:"member_id"`
	Member       Member          `json:"member,omitempty"`
	Direction    string          `gorm:"size:3;not null" json:"direction"`
	Points       int64           `gorm:"not null" json:"points"`
	PointsDelta  int64           `gorm:"column:points_delta;not null" json:"points_delta"`
	SourceType   string          `gorm:"size:32;not null" json:"source_type"`
	Reason       string          `gorm:"type:text" json:"reason"`
	Operator     string          `gorm:"size:64" json:"operator"`
	MoneyAmount  *float64        `json:"money_amount"`
	ProductName  string          `gorm:"size:200" json:"product_name"`
	RedeemAmount *float64        `json:"redeem_amount"`
	OccurredAt   time.Time       `json:"occurred_at"`
	ExpiresAt    *time.Time      `gorm:"type:date" json:"expires_at"`
}

func (PointTransaction) TableName() string { return "point_transactions" }
