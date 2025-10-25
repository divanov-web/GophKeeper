package model

import "time"

type Withdrawal struct {
	ID          int64     `gorm:"primaryKey;autoIncrement"`
	UserID      int64     `gorm:"index;not null"`
	Order       string    `gorm:"not null"`
	Sum         float64   `gorm:"not null"`
	ProcessedAt time.Time `gorm:"autoCreateTime" json:"processed_at"`
}
