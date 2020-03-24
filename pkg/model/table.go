package model

import "time"

// InfraApply
type InfraApply struct {
	ID          int32     `gorm:"primary_key"`
	DeviceCode  string    `gorm:"column:device_code"`
	Applyer     string    `gorm:"column:applyer"`
	Status      string    `gorm:"column:status"` // init|refused|approved|expired
	SubjectName string    `gorm:"column:subject_name"`
	ReviewId    string    `gorm:"column:review_id"`
	ExpiresAt   time.Time `gorm:"column:expires_at"`
	ReviewedAt  time.Time `gorm:"column:review_at"`
}

// TableName is the getter for tables' names
func (c *InfraApply) TableName() string {
	return "t_subject_apply"
}
