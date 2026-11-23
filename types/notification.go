package types

type (
	NotificationProvider string

	NotificationSettings struct {
		Config   JsonObject           `json:"config"`
		Provider NotificationProvider `json:"provider"`
		ID       uint                 `json:"id" readOnly:"true"`
		Enabled  bool                 `json:"enabled"`
	}

	NotificationTestResponse struct {
		Message string `json:"message"`

		Warning string `json:"warning,omitempty"`
	}

	NotificationDispatchKind string

	NotificationDispatchVulnerabilityFound struct {
		CVEID            string `json:"cveId"`
		CVELink          string `json:"cveLink"`
		Severity         string `json:"severity"`
		ImageName        string `json:"imageName"`
		FixedVersion     string `json:"fixedVersion,omitempty"`
		PkgName          string `json:"pkgName,omitempty"`
		InstalledVersion string `json:"installedVersion,omitempty"`
	}

	NotificationDispatchPruneReport struct {
		Result PruneAllResult `json:"result"`
	}

	NotificationDispatchAutoHeal struct {
		ContainerName string `json:"containerName"`
		ContainerID   string `json:"containerId"`
	}

	NotificationDispatchRequest struct {
		VulnerabilityFound *NotificationDispatchVulnerabilityFound `json:"vulnerabilityFound,omitempty"`
		PruneReport        *NotificationDispatchPruneReport        `json:"pruneReport,omitempty"`
		AutoHeal           *NotificationDispatchAutoHeal           `json:"autoHeal,omitempty"`
		Kind               NotificationDispatchKind                `json:"kind"`
	}

	NotificationDispatchResponse struct {
		Message   string `json:"message"`
		Delivered int    `json:"delivered"`
	}
)

const (
	NotificationProviderDiscord                NotificationProvider     = "discord"
	NotificationProviderEmail                  NotificationProvider     = "email"
	NotificationProviderTelegram               NotificationProvider     = "telegram"
	NotificationProviderSignal                 NotificationProvider     = "signal"
	NotificationProviderSlack                  NotificationProvider     = "slack"
	NotificationProviderNtfy                   NotificationProvider     = "ntfy"
	NotificationProviderPushover               NotificationProvider     = "pushover"
	NotificationProviderGotify                 NotificationProvider     = "gotify"
	NotificationProviderMatrix                 NotificationProvider     = "matrix"
	NotificationProviderGoogleChat             NotificationProvider     = "googlechat"
	NotificationProviderGeneric                NotificationProvider     = "generic"
	NotificationDispatchKindVulnerabilityFound NotificationDispatchKind = "vulnerability_found"
	NotificationDispatchKindPruneReport        NotificationDispatchKind = "prune_report"
	NotificationDispatchKindAutoHeal           NotificationDispatchKind = "auto_heal"
)
