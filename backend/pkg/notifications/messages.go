package notifications

import (
	"fmt"
	"strings"

	"github.com/docker/go-units"

	"getarcane.dev/types/v3"
)

type (
	MessageFormat string
)

const (
	MessageFormatPlain        MessageFormat = "plain"
	MessageFormatMarkdown     MessageFormat = "markdown"
	MessageFormatNtfyMarkdown MessageFormat = "ntfy_markdown"
	MessageFormatSlack        MessageFormat = "slack"
	MessageFormatHTML         MessageFormat = "html"
)

func formatNotificationTitle(format MessageFormat, title string) string {
	switch format {
	case MessageFormatPlain:
		return title
	case MessageFormatMarkdown:
		return fmt.Sprintf("**%s**", title)
	case MessageFormatSlack:
		return fmt.Sprintf("*%s*", title)
	case MessageFormatHTML:
		return fmt.Sprintf("<b>%s</b>", title)
	default:
		return title
	}
}

func formatNotificationLabel(format MessageFormat, label string) string {
	switch format {
	case MessageFormatPlain:
		return label + ":"
	case MessageFormatMarkdown:
		return fmt.Sprintf("**%s:**", label)
	case MessageFormatSlack:
		return fmt.Sprintf("*%s:*", label)
	case MessageFormatHTML:
		return fmt.Sprintf("<b>%s:</b>", label)
	default:
		return label + ":"
	}
}

func BuildVulnerabilitySummaryNotificationMessage(format MessageFormat, environmentName, summaryLabel, overview, fixableCount, severityBreakdown, sampleCVEs string) string {
	var message strings.Builder

	if format == MessageFormatNtfyMarkdown {
		fmt.Fprintf(&message, "🖥️ %s\n", environmentName)

		if strings.TrimSpace(summaryLabel) != "" {
			fmt.Fprintf(&message, "\n📊 **%s**\n", summaryLabel)
		}
		if strings.TrimSpace(overview) != "" {
			fmt.Fprintf(&message, "📦 %s\n", overview)
		}
		if strings.TrimSpace(fixableCount) != "" {
			fmt.Fprintf(&message, "🛠️ %s\n", fixableCount)
		}
		if strings.TrimSpace(severityBreakdown) != "" {
			severityItems := strings.Fields(severityBreakdown)
			fmt.Fprintf(&message, "🚨 %s\n", strings.Join(severityItems, " • "))
		}
		if strings.TrimSpace(sampleCVEs) != "" {
			fmt.Fprintf(&message, "🔎 `%s`\n", sampleCVEs)
		}

		return message.String()
	}

	fmt.Fprintf(&message, "%s\n\n", formatNotificationTitle(format, "📊 Daily Vulnerability Summary"))

	if strings.TrimSpace(summaryLabel) != "" {
		fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Summary"), summaryLabel)
	}
	fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Environment"), environmentName)
	if strings.TrimSpace(overview) != "" {
		fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Overview"), overview)
	}
	if strings.TrimSpace(fixableCount) != "" {
		fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Fixable Vulnerabilities"), fixableCount)
	}
	if strings.TrimSpace(severityBreakdown) != "" {
		fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Severity Breakdown"), severityBreakdown)
	}
	if strings.TrimSpace(sampleCVEs) != "" {
		fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Sample CVEs"), sampleCVEs)
	}

	return message.String()
}

func BuildPruneReportNotificationMessage(format MessageFormat, environmentName string, result *types.PruneAllResult) string {
	var message strings.Builder

	if format == MessageFormatNtfyMarkdown {
		fmt.Fprintf(&message, "🖥️ %s\n", environmentName)
		fmt.Fprintf(
			&message,
			"💾 **%s** reclaimed\n\n",
			FormatBytes(result.SpaceReclaimed),
		)
		fmt.Fprintf(&message, "📦 Images: %s\n", FormatBytes(result.ImageSpaceReclaimed))
		fmt.Fprintf(&message, "🧱 Containers: %s\n", FormatBytes(result.ContainerSpaceReclaimed))
		fmt.Fprintf(&message, "💿 Volumes: %s\n", FormatBytes(result.VolumeSpaceReclaimed))
		fmt.Fprintf(&message, "🛠️ Build cache: %s\n", FormatBytes(result.BuildCacheSpaceReclaimed))
		if len(result.Errors) > 0 {
			fmt.Fprintf(&message, "\n⚠️ **%d error(s)**\n", len(result.Errors))
			for _, pruneErr := range result.Errors {
				fmt.Fprintf(&message, "- %s\n", pruneErr)
			}
		}

		return message.String()
	}

	fmt.Fprintf(&message, "%s\n\n", formatNotificationTitle(format, "🧹 System Prune Report"))
	fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Environment"), environmentName)
	fmt.Fprintf(&message, "%s %s\n\n", formatNotificationLabel(format, "Total Space Reclaimed"), FormatBytes(result.SpaceReclaimed))
	fmt.Fprintf(&message, "%s\n", formatNotificationLabel(format, "Breakdown"))
	fmt.Fprintf(&message, "- Containers: %s\n", FormatBytes(result.ContainerSpaceReclaimed))
	fmt.Fprintf(&message, "- Images: %s\n", FormatBytes(result.ImageSpaceReclaimed))
	fmt.Fprintf(&message, "- Volumes: %s\n", FormatBytes(result.VolumeSpaceReclaimed))
	fmt.Fprintf(&message, "- Build Cache: %s\n", FormatBytes(result.BuildCacheSpaceReclaimed))
	if len(result.Errors) > 0 {
		fmt.Fprintf(&message, "\n%s\n", formatNotificationLabel(format, "Errors"))
		for _, pruneErr := range result.Errors {
			fmt.Fprintf(&message, "- %s\n", pruneErr)
		}
	}
	return message.String()
}

func BuildAutoHealNotificationMessage(format MessageFormat, environmentName, containerName string) string {
	var message strings.Builder

	if format == MessageFormatNtfyMarkdown {
		fmt.Fprintf(&message, "🖥️ %s\n\n", environmentName)
		fmt.Fprintf(&message, "🐳 **%s**\n", containerName)
		fmt.Fprintln(&message, "🔄 Restarted automatically")
		fmt.Fprintln(&message, "⚠️ Reason: container was unhealthy")

		return message.String()
	}

	fmt.Fprintf(&message, "%s\n\n", formatNotificationTitle(format, "Auto Heal"))
	fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Environment"), environmentName)
	fmt.Fprintf(&message, "%s %s\n", formatNotificationLabel(format, "Container"), containerName)
	fmt.Fprintf(&message, "%s Automatically restarted because it was unhealthy.\n", formatNotificationLabel(format, "Status"))
	return message.String()
}

func BuildTestNotificationMessage(format MessageFormat, environmentName string) string {
	if format == MessageFormatNtfyMarkdown {
		return fmt.Sprintf("🖥️ %s\n\n✅ Your notification setup is working correctly!\n", environmentName)
	}
	return fmt.Sprintf("%s\n\n%s %s\nYour notification setup is working correctly!\n",
		formatNotificationTitle(format, "Test Notification"), formatNotificationLabel(format, "Environment"), environmentName)
}

func FormatBytes(bytes uint64) string {
	if bytes < units.KiB {
		return fmt.Sprintf("%d B", bytes)
	}

	return units.CustomSize("%.1f %s", float64(bytes), units.KiB, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"})
}

func BuildEmailSubject(environmentName, subject string) string {
	trimmedEnvironmentName := strings.TrimSpace(environmentName)
	if trimmedEnvironmentName == "" {
		return subject
	}

	return fmt.Sprintf("[%s] %s", SanitizeForEmail(trimmedEnvironmentName), subject)
}
