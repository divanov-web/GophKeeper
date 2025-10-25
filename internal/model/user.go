package model

import "time"

type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	Login     string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"`
	Balance   float64   `gorm:"not null;default:0"`
	Withdrawn float64   `gorm:"not null;default:0"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
