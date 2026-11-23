package environment

import (
	"time"

	"getarcane.dev/arcane/v3/internal/database"
	"getarcane.dev/arcane/v3/internal/environment/children/group"
)

type (
	Environment struct {
		database.BaseModel

		LastSeen              *time.Time `json:"lastSeen" gorm:"column:last_seen"`
		LastAgentTransport    *string    `json:"lastAgentTransport" gorm:"column:last_agent_transport"`
		AccessToken           *string    `json:"-" gorm:"column:access_token"`
		ApiKeyID              *string    `json:"-" gorm:"column:api_key_id"`
		ParentEnvironmentID   *string    `json:"-" gorm:"column:parent_environment_id"`
		SwarmNodeID           *string    `json:"-" gorm:"column:swarm_node_id"`
		EnrollmentRequestHash *string    `json:"-" gorm:"column:enrollment_request_hash"`
		GroupID               *string    `json:"groupId,omitempty" gorm:"column:group_id"`
		// Group is read-only; writes go through GroupID.
		Group *group.Group `json:"-" gorm:"foreignKey:GroupID;constraint:OnDelete:SET NULL"`

		Name    string `json:"name" sortable:"true" unorm:"nfc" trim:"true"`
		ApiUrl  string `json:"apiUrl" gorm:"column:api_url" sortable:"true"`
		Status  string `json:"status" sortable:"true"`
		Enabled bool   `json:"enabled" sortable:"true"`
		Hidden  bool   `json:"hidden" gorm:"column:hidden;default:false"`
	}

	EnvironmentStatus string
)

const (
	EnvironmentStatusOnline  EnvironmentStatus = "online"
	EnvironmentStatusOffline EnvironmentStatus = "offline"
	EnvironmentStatusError   EnvironmentStatus = "error"
	EnvironmentStatusPending EnvironmentStatus = "pending"
)

func (Environment) TableName() string { return "environments" }
