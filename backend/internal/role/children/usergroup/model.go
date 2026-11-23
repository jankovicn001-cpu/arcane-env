package usergroup

import (
	"getarcane.dev/arcane/v3/internal/database"
)

type (
	Group struct {
		database.BaseModel

		Name        string `gorm:"column:name;not null"`
		Description string `gorm:"column:description;not null;default:''"`
	}

	Link struct {
		database.BaseModel

		UserGroupID string `gorm:"column:user_group_id;not null;index"`
		Kind        string `gorm:"column:kind;not null"`
		Value       string `gorm:"column:value;not null"`
	}

	Member struct {
		database.BaseModel

		UserGroupID string `gorm:"column:user_group_id;not null;index"`
		UserID      string `gorm:"column:user_id;not null;index"`
		Source      string `gorm:"column:source;not null;default:'manual'"`
	}

	Grant struct {
		database.BaseModel

		EnvironmentID      *string `gorm:"column:environment_id;index"`
		EnvironmentGroupID *string `gorm:"column:environment_group_id;index"`
		UserGroupID        string  `gorm:"column:user_group_id;not null;index"`
		RoleID             string  `gorm:"column:role_id;not null;index"`
	}
)

const (
	SourceManual = "manual"
	SourceOidc   = "oidc"
	SourceLdap   = "ldap"
)

func (Group) TableName() string { return "user_groups" }

func (Link) TableName() string { return "user_group_links" }

func (Member) TableName() string { return "user_group_members" }

func (Grant) TableName() string { return "user_group_role_assignments" }
