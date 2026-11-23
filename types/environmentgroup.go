package types

import (
	"time"
)

type (
	EnvironmentGroup struct {
		CreatedAt        time.Time `json:"createdAt" doc:"Creation timestamp"`
		UpdatedAt        time.Time `json:"updatedAt" doc:"Last update timestamp"`
		ID               string    `json:"id" doc:"Environment group ID"`
		Name             string    `json:"name" doc:"Environment group name"`
		Description      string    `json:"description" doc:"Environment group description"`
		EnvironmentCount int       `json:"environmentCount" doc:"Number of agents assigned to the group"`
	}

	EnvironmentGroupRef struct {
		ID          string `json:"id" doc:"Environment group ID"`
		Name        string `json:"name" doc:"Environment group name"`
		Description string `json:"description" doc:"Environment group description"`
	}

	CreateEnvironmentGroupRequest struct {
		Name        string `json:"name" minLength:"1" maxLength:"64" doc:"Environment group name, unique case-insensitively" unorm:"nfc" trim:"true"`
		Description string `json:"description,omitempty" required:"false" maxLength:"256" doc:"Environment group description" unorm:"nfc" trim:"true"`
	}

	UpdateEnvironmentGroupRequest struct {
		Name        *string `json:"name,omitempty" required:"false" minLength:"1" maxLength:"64" doc:"Environment group name, unique case-insensitively" unorm:"nfc" trim:"true"`
		Description *string `json:"description,omitempty" required:"false" maxLength:"256" doc:"Environment group description" unorm:"nfc" trim:"true"`
	}
)
