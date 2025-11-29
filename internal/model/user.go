package model

import "time"

type User struct {
	ID        int64     `gorm:"primaryKey;autoIncrement"`
	Login     string    `gorm:"uniqueIndex;not null"`
	Password  string    `gorm:"not null"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}
