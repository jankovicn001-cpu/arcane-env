package group

import (
	"getarcane.dev/arcane/v3/internal/database"
)

type (
	Group struct {
		database.BaseModel

		Name        string `gorm:"column:name;not null"`
		Description string `gorm:"column:description;not null;default:''"`
	}
)

func (Group) TableName() string { return "environment_groups" }
