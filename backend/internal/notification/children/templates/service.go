package templates

import (
	"fmt"
	"strings"
	"time"

	"kit.getarcane.app"

	"getarcane.dev/arcane/v3/pkg/notifications"
	"getarcane.dev/types/v3"
)

type (
	Service struct {
		appURL func() string
	}
)

func NewService(appURL func() string) *Service {
	return &Service{appURL: appURL}
}

func (s *Service) Vulnerability(environmentName string, payload types.NotificationDispatchVulnerabilityFound) notifications.Content {
	defaultTitle := notifications.BuildEmailSubject(environmentName, "Daily Vulnerability Summary")
	return notifications.Content{
		Text: notifications.TextByFormat(func(format notifications.MessageFormat) string {
			return notifications.BuildVulnerabilitySummaryNotificationMessage(
				format,
				environmentName,
				payload.CVEID,
				payload.ImageName,
				payload.FixedVersion,
				payload.Severity,
				payload.PkgName,
			)
		}),
		Title:        defaultTitle,
		NtfyTitle:    "🛡️ Daily Vulnerability Summary",
		DefaultTitle: defaultTitle,
		RenderEmail: func() (string, string, error) {
			htmlBody, err := renderEmail(emailData{
				Title: "Daily Vulnerability Summary",
				Intro: payload.CVEID,
				Rows: []emailRow{
					{Label: "Environment", Value: environmentName},
					{Label: "Overview", Value: payload.ImageName},
					{Label: "Fixable Vulnerabilities", Value: payload.FixedVersion},
					{Label: "Severity Breakdown", Value: payload.Severity},
					{Label: "Sample CVEs", Value: payload.PkgName},
				},
				AppURL: s.appURL(),
			})
			if err != nil {
				return "", "", fmt.Errorf("failed to render summary email: %w", err)
			}
			subject := notifications.BuildEmailSubject(environmentName, "Daily Vulnerability Summary: "+notifications.SanitizeForEmail(payload.CVEID))
			return subject, htmlBody, nil
		},
		RequireNtfyTopic:     true,
		ValidatePushoverUser: true,
	}
}

func (s *Service) PruneReport(environmentName string, result *types.PruneAllResult) notifications.Content {
	defaultTitle := notifications.BuildEmailSubject(environmentName, "System Prune Report")
	return notifications.Content{
		Text: notifications.TextByFormat(func(format notifications.MessageFormat) string {
			return notifications.BuildPruneReportNotificationMessage(format, environmentName, result)
		}),
		Title:        defaultTitle,
		NtfyTitle:    kit.Ternary(len(result.Errors) > 0, "⚠️ System Prune Finished with Errors", "🧹 System Prune Complete"),
		DefaultTitle: defaultTitle,
		RenderEmail: func() (string, string, error) {
			htmlBody, err := renderEmail(emailData{
				Title: "System Prune Report",
				Rows: []emailRow{
					{Label: "Environment", Value: environmentName},
					{Label: "Status", Value: kit.Ternary(len(result.Errors) > 0, "Finished with errors: "+strings.Join(result.Errors, "; "), "Completed")},
					{Label: "Total Reclaimed", Value: notifications.FormatBytes(result.SpaceReclaimed)},
					{Label: "Containers", Value: notifications.FormatBytes(result.ContainerSpaceReclaimed)},
					{Label: "Images", Value: notifications.FormatBytes(result.ImageSpaceReclaimed)},
					{Label: "Volumes", Value: notifications.FormatBytes(result.VolumeSpaceReclaimed)},
					{Label: "Build Cache", Value: notifications.FormatBytes(result.BuildCacheSpaceReclaimed)},
					{Label: "Time", Value: time.Now().Format(time.RFC1123)},
				},
				AppURL: s.appURL(),
			})
			if err != nil {
				return "", "", fmt.Errorf("failed to render email: %w", err)
			}
			subject := notifications.BuildEmailSubject(environmentName, fmt.Sprintf(
				"System Prune Report: %s Reclaimed%s",
				notifications.FormatBytes(result.SpaceReclaimed),
				kit.Ternary(len(result.Errors) > 0, fmt.Sprintf(", %d Error(s)", len(result.Errors)), ""),
			))
			return subject, htmlBody, nil
		},
	}
}

func (s *Service) AutoHeal(environmentName, containerName string) notifications.Content {
	defaultTitle := notifications.BuildEmailSubject(environmentName, "Auto Heal")
	return notifications.Content{
		Text: notifications.TextByFormat(func(format notifications.MessageFormat) string {
			return notifications.BuildAutoHealNotificationMessage(format, environmentName, containerName)
		}),
		Title:        defaultTitle,
		NtfyTitle:    "❤️‍🩹 Container Auto-Healed",
		DefaultTitle: defaultTitle,
		RenderEmail: func() (string, string, error) {
			htmlBody, err := renderEmail(emailData{
				Title: "Container Auto-Healed",
				Intro: "Automatically restarted because it was unhealthy.",
				Rows: []emailRow{
					{Label: "Environment", Value: environmentName},
					{Label: "Container", Value: containerName},
				},
				AppURL: s.appURL(),
			})
			if err != nil {
				return "", "", fmt.Errorf("failed to render email: %w", err)
			}
			subject := notifications.BuildEmailSubject(environmentName, fmt.Sprintf("Auto Heal: Container '%s' Restarted", containerName))
			return subject, htmlBody, nil
		},
	}
}

func (s *Service) Test(environmentName string) notifications.Content {
	defaultTitle := notifications.BuildEmailSubject(environmentName, "Test Notification")
	return notifications.Content{
		Text: notifications.TextByFormat(func(format notifications.MessageFormat) string {
			return notifications.BuildTestNotificationMessage(format, environmentName)
		}),
		Title:        defaultTitle,
		NtfyTitle:    "✅ Test Notification",
		DefaultTitle: defaultTitle,
		RenderEmail: func() (string, string, error) {
			htmlBody, err := renderEmail(emailData{
				Title:  "Test Email",
				Intro:  "Your email setup is working correctly!",
				Rows:   []emailRow{{Label: "Environment", Value: environmentName}},
				AppURL: s.appURL(),
			})
			if err != nil {
				return "", "", fmt.Errorf("failed to render test email: %w", err)
			}
			return notifications.BuildEmailSubject(environmentName, "Test Email from Arcane"), htmlBody, nil
		},
	}
}
