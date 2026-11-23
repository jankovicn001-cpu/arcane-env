package notifications

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"getarcane.dev/types/v3"
)

func TestNtfyNotificationMessagesVisualLayout(t *testing.T) {
	tests := []struct {
		name        string
		build       func() string
		contains    []string
		notContains []string
	}{
		{
			name: "vulnerability summary",
			build: func() string {
				return BuildVulnerabilitySummaryNotificationMessage(
					MessageFormatNtfyMarkdown,
					"Local Docker",
					"Daily Summary - 2026-10-07",
					"5 image(s) scanned, 2 with fixable vulnerabilities",
					"7 fixable vulnerability record(s)",
					"Critical:1 High:3 Medium:2 Low:1 Unknown:0",
					"CVE-2025-1234, CVE-2025-5678",
				)
			},
			contains: []string{
				"📊 **Daily Summary - 2026-10-07**",
				"📦 5 image(s) scanned, 2 with fixable vulnerabilities",
				"🛠️ 7 fixable vulnerability record(s)",
				"🚨 Critical:1 • High:3 • Medium:2 • Low:1 • Unknown:0",
				"🔎 `CVE-2025-1234, CVE-2025-5678`",
			},
			notContains: []string{
				"Daily Vulnerability Summary",
				"Severity Breakdown:",
			},
		},
		{
			name: "prune report",
			build: func() string {
				return BuildPruneReportNotificationMessage(
					MessageFormatNtfyMarkdown,
					"Local Docker",
					&types.PruneAllResult{
						SpaceReclaimed:           3825205248,
						ContainerSpaceReclaimed:  503316480,
						ImageSpaceReclaimed:      2449473536,
						VolumeSpaceReclaimed:     641728512,
						BuildCacheSpaceReclaimed: 230162432,
					},
				)
			},
			contains: []string{
				"💾 **3.6 GiB** reclaimed",
				"📦 Images: 2.3 GiB",
				"🧱 Containers: 480.0 MiB",
				"💿 Volumes: 612.0 MiB",
				"🛠️ Build cache: 219.5 MiB",
			},
			notContains: []string{
				"System Prune Report",
				"Breakdown:",
			},
		},
		{
			name: "auto heal",
			build: func() string {
				return BuildAutoHealNotificationMessage(
					MessageFormatNtfyMarkdown,
					"Local Docker",
					"jellyfin",
				)
			},
			contains: []string{
				"🐳 **jellyfin**",
				"🔄 Restarted automatically",
				"⚠️ Reason: container was unhealthy",
			},
			notContains: []string{
				"Container Auto-Healed",
				"Status:",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			message := tt.build()

			require.True(
				t,
				strings.HasPrefix(message, "🖥️ Local Docker"),
			)

			for _, expected := range tt.contains {
				require.Contains(t, message, expected)
			}

			for _, unexpected := range tt.notContains {
				require.NotContains(t, message, unexpected)
			}
		})
	}
}
