package templates

import (
	"testing"

	"github.com/stretchr/testify/require"

	"getarcane.dev/types/v3"
)

func TestService_TestIncludesEnvironment(t *testing.T) {
	svc := NewService(func() string { return "http://localhost:3000" })
	content := svc.Test("Homelab Prod")

	_, htmlBody, err := content.RenderEmail()
	require.NoError(t, err)
	require.Contains(t, htmlBody, "Homelab Prod")
	for format, text := range content.Text {
		require.Contains(t, text, "Homelab Prod", format)
	}
}

func TestService_RenderVulnerabilitySummaryEmailTemplate_IncludesEnvironment(t *testing.T) {
	svc := NewService(func() string { return "http://localhost:3000" })

	_, htmlBody, err := svc.Vulnerability("Remote Alpha", types.NotificationDispatchVulnerabilityFound{
		CVEID:        "Daily Summary - 2026-01-09",
		ImageName:    "5 image(s) scanned, 2 with fixable vulnerabilities",
		FixedVersion: "7 fixable vulnerability record(s)",
		Severity:     "Critical:1 High:3 Medium:2 Low:1 Unknown:0",
		PkgName:      "CVE-2025-1234",
	}).RenderEmail()
	require.NoError(t, err)
	require.Contains(t, htmlBody, "Remote Alpha")
}

func TestService_RenderPruneReportEmailTemplate_IncludesEnvironment(t *testing.T) {
	svc := NewService(func() string { return "http://localhost:3000" })

	_, htmlBody, err := svc.PruneReport("Cluster West", &types.PruneAllResult{
		SpaceReclaimed:           3825205248,
		ContainerSpaceReclaimed:  503316480,
		ImageSpaceReclaimed:      2449473536,
		VolumeSpaceReclaimed:     641728512,
		BuildCacheSpaceReclaimed: 230162432,
	}).RenderEmail()
	require.NoError(t, err)
	require.Contains(t, htmlBody, "Cluster West")
	for _, size := range []string{"3.6 GiB", "480.0 MiB", "2.3 GiB", "612.0 MiB", "219.5 MiB"} {
		require.Contains(t, htmlBody, size)
	}
}
