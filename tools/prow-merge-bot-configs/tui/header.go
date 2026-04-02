package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func renderHeaderWithQueue(width int, report *AuditReport, loading bool, spinnerStr string, queueLoading map[string]bool, watchInterval time.Duration, watchPaused bool) string {
	if report == nil {
		content := "Prow Audit │ Loading..."
		if spinnerStr != "" {
			content += " " + spinnerStr
		}
		return HeaderStyle.Width(width).Render(content)
	}

	source := report.Source
	ts := report.Timestamp.Format("2006-01-02 15:04Z")

	ratePart := ""
	if report.RateLimit != nil {
		remaining := report.RateLimit.Remaining
		limit := report.RateLimit.Limit
		resetAt := report.RateLimit.ResetAt.Format("15:04 UTC")
		var rateStyle lipgloss.Style
		switch {
		case remaining == 0:
			rateStyle = lipgloss.NewStyle().Foreground(ColorIssue)
			ratePart = fmt.Sprintf("API: %s", rateStyle.Render(fmt.Sprintf("exhausted — resets %s", resetAt)))
		case remaining < 100:
			rateStyle = lipgloss.NewStyle().Foreground(ColorWarn)
			ratePart = fmt.Sprintf("API: %s", rateStyle.Render(fmt.Sprintf("%d/%d resets %s", remaining, limit, resetAt)))
		default:
			rateStyle = lipgloss.NewStyle().Foreground(ColorOK)
			ratePart = fmt.Sprintf("API: %s", rateStyle.Render(fmt.Sprintf("%d/%d", remaining, limit)))
		}

		// GraphQL rate limit (used by gh pr list in merge queue).
		if report.RateLimit.GraphQLLimit > 0 {
			gqlRemaining := report.RateLimit.GraphQLRemaining
			gqlLimit := report.RateLimit.GraphQLLimit
			gqlResetAt := report.RateLimit.GraphQLResetAt.Format("15:04 UTC")
			var gqlStyle lipgloss.Style
			switch {
			case gqlRemaining == 0:
				gqlStyle = lipgloss.NewStyle().Foreground(ColorIssue)
				ratePart += fmt.Sprintf(" GQL: %s", gqlStyle.Render(fmt.Sprintf("exhausted — resets %s", gqlResetAt)))
			case gqlRemaining < 100:
				gqlStyle = lipgloss.NewStyle().Foreground(ColorWarn)
				ratePart += fmt.Sprintf(" GQL: %s", gqlStyle.Render(fmt.Sprintf("%d/%d resets %s", gqlRemaining, gqlLimit, gqlResetAt)))
			default:
				gqlStyle = lipgloss.NewStyle().Foreground(ColorOK)
				ratePart += fmt.Sprintf(" GQL: %s", gqlStyle.Render(fmt.Sprintf("%d/%d", gqlRemaining, gqlLimit)))
			}
		}
	}

	left := fmt.Sprintf("Prow Audit │ %s │ %s", source, ts)
	if ratePart != "" {
		left += " │ " + ratePart
	}

	// Show loading indicators.
	if loading {
		left += " │ " + spinnerStr + " refreshing..."
	} else if len(queueLoading) > 0 {
		var repos []string
		for repo := range queueLoading {
			// Show just the short repo name.
			parts := strings.SplitN(repo, "/", 2)
			if len(parts) == 2 {
				repos = append(repos, parts[1])
			} else {
				repos = append(repos, repo)
			}
		}
		loadingText := strings.Join(repos, ", ")
		if len(repos) > 3 {
			loadingText = fmt.Sprintf("%d repos", len(repos))
		}
		left += " │ " + spinnerStr + " queue: " + loadingText
	}

	// Watch mode indicator.
	if watchInterval > 0 {
		if watchPaused {
			left += " │ ⏸ paused"
		} else {
			left += fmt.Sprintf(" │ ⟳ %s", watchInterval)
		}
	}

	return HeaderStyle.Width(width).Render(left)
}
