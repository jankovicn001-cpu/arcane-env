package types

import (
	"time"
)

type (
	VersionInfo struct {
		CurrentVersion   string   `json:"currentVersion"`
		CurrentTag       string   `json:"currentTag,omitempty"`
		CurrentDigest    string   `json:"currentDigest,omitempty"`
		Revision         string   `json:"revision"`
		ShortRevision    string   `json:"shortRevision"`
		GoVersion        string   `json:"goVersion"`
		NodeVersion      string   `json:"nodeVersion"`
		SvelteKitVersion string   `json:"svelteKitVersion"`
		BuildTime        string   `json:"buildTime,omitempty"`
		DisplayVersion   string   `json:"displayVersion"`
		EnabledFeatures  []string `json:"enabledFeatures,omitempty"`
		IsSemverVersion  bool     `json:"isSemverVersion"`
		IsStableVersion  bool     `json:"isStableVersion"`
	}

	VersionStreamEvent struct {
		Timestamp time.Time    `json:"timestamp"`
		Info      *VersionInfo `json:"info,omitempty"`
		Type      string       `json:"type"`
	}
)
