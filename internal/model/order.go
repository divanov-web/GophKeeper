package model

import (
	"time"
)

type OrderStatus string

const (
	OrderStatusNew        OrderStatus = "NEW"
	OrderStatusProcessing OrderStatus = "PROCESSING"
	OrderStatusInvalid    OrderStatus = "INVALID"
	OrderStatusProcessed  OrderStatus = "PROCESSED"
)

type Order struct {
	ID        int64       `gorm:"primaryKey;autoIncrement"`
	Number    string      `gorm:"uniqueIndex;not null" json:"number"`
	UserID    int64       `gorm:"index;not null" json:"-"`
	User      User        `gorm:"foreignKey:UserID;references:ID" json:"-"`
	Status    OrderStatus `gorm:"not null" json:"status"`
	Accrual   *float64    `json:"accrual,omitempty"`
	CreatedAt time.Time   `gorm:"autoCreateTime" json:"created_at"`
}
