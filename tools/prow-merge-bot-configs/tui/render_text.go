package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// ANSI 256-color codes matching the colorblind-friendly TUI palette.
// Uses same colors as theme.go: blue (OK), orange (warn), magenta (issue), cyan (info).
var (
	tOK     = "\033[38;5;74m"  // blue (#4A9BDB approximated)
	tWarn   = "\033[38;5;214m" // orange (#E69F00 approximated)
	tIssue  = "\033[38;5;175m" // magenta/pink (#CC79A7 approximated)
	tInfo   = "\033[38;5;117m" // cyan (#56B4E9 approximated)
	tBold   = "\033[1m"
	tReset  = "\033[0m"
)

func init() {
	// Disable colors if stdout is not a terminal.
	fi, err := os.Stdout.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		tOK, tWarn, tIssue, tInfo, tBold, tReset = "", "", "", "", "", ""
	}
}

func severityPrefix(s Severity) string {
	switch s {
	case SeverityOK:
		return tOK + "[OK]" + tReset + "    "
	case SeverityInfo:
		return tInfo + "[INFO]" + tReset + "  "
	case SeverityWarning:
		return tWarn + "[WARN]" + tReset + "  "
	case SeverityIssue:
		return tIssue + "[ISSUE]" + tReset + " "
	default:
		return "        "
	}
}

// RenderText produces colored terminal text output of the audit report,
// matching the section order from audit.sh:
// 1. Header, 2. No Prow Config, 3-5. Repo groups, 6. Comparison, 7. Tide, 8. Summary
func RenderText(report *AuditReport) string {
	if report == nil {
		return "No audit data available."
	}

	var b strings.Builder

	// 1. Header
	fmt.Fprintf(&b, "\n%sProw Merge Bot Configuration Audit%s\n", tBold, tReset)
	fmt.Fprintf(&b, "Source: %s\n", report.Source)
	fmt.Fprintf(&b, "Date: %s\n", report.Timestamp.Format("2006-01-02T15:04:05Z"))
	if report.RateLimit != nil {
		fmt.Fprintf(&b, "API: %d/%d remaining (resets %s)\n",
			report.RateLimit.Remaining, report.RateLimit.Limit,
			report.RateLimit.ResetAt.Format("15:04 UTC"))
	}
	b.WriteString("\n")

	// 2. Repos With No Prow Config (Expected)
	fmt.Fprintf(&b, "%s=== Repos With No Prow Config (Expected) ===%s\n", tBold, tReset)
	noConfigFound := false
	for _, g := range report.Groups {
		for _, repo := range g.Repos {
			if !repo.HasConfig {
				noConfigFound = true
				fmt.Fprintf(&b, "%s[ISSUE]%s %s: No prow config found in openshift/release\n", tIssue, tReset, repo.Name)
			}
		}
	}
	if !noConfigFound {
		fmt.Fprintf(&b, "%s[OK]%s    All repos have prow config.\n", tOK, tReset)
	}
	b.WriteString("\n")

	// 3-5. Repo groups (in order)
	for _, g := range report.Groups {
		fmt.Fprintf(&b, "%s=== %s ===%s\n", tBold, g.Name, tReset)
		fmt.Fprintf(&b, "%s\n\n", g.Description)

		for _, repo := range g.Repos {
			for _, f := range repo.Findings {
				fmt.Fprintf(&b, "%s%s\n", severityPrefix(f.Severity), f.Message)
				if f.SuggestedFix != "" {
					fmt.Fprintf(&b, "  Suggested fix:\n")
					for _, line := range strings.Split(f.SuggestedFix, "\n") {
						fmt.Fprintf(&b, "    %s\n", line)
					}
				}
			}
		}
		b.WriteString("\n")
	}

	// 6. Cross-repo comparison
	fmt.Fprintf(&b, "%s=== Cross-Repo Field Comparison (Outliers) ===%s\n", tBold, tReset)
	b.WriteString("Shows fields where a repo differs from the majority value within its group.\n")

	for _, groupDef := range DefaultGroups() {
		fields, ok := report.Comparison[groupDef.Type]
		if !ok {
			continue
		}
		hasOutliers := false
		for _, fc := range fields {
			if len(fc.Outliers) > 0 {
				hasOutliers = true
				break
			}
		}
		if !hasOutliers {
			continue
		}

		fmt.Fprintf(&b, "\n%s--- %s ---%s\n", tBold, groupDef.Name, tReset)
		for field, fc := range fields {
			if len(fc.Outliers) == 0 {
				continue
			}
			b.WriteString("\n")
			fmt.Fprintf(&b, "  %s (majority: %s)\n", field, fc.Majority)
			for _, outlier := range fc.Outliers {
				fmt.Fprintf(&b, "    %s%s%s: %s (differs from majority: %s)\n",
					tWarn, outlier.Repo, tReset, outlier.Value, fc.Majority)
			}
		}
	}
	b.WriteString("\n")

	// 7. Tide Branch Coverage
	fmt.Fprintf(&b, "%s=== Tide Branch Coverage ===%s\n", tBold, tReset)
	b.WriteString("Branches included in Tide merge queries for each repo:\n\n")
	for _, g := range report.Groups {
		for _, repo := range g.Repos {
			if !repo.HasConfig {
				continue
			}
			if len(repo.Tide.IncludedBranches) > 0 {
				fmt.Fprintf(&b, "  %s:\n    %s\n", repo.Name, strings.Join(repo.Tide.IncludedBranches, ","))
			} else {
				fmt.Fprintf(&b, "%s[WARN]%s  %s: No includedBranches found in Tide config (uses excludedBranches or all-branch query)\n",
					tWarn, tReset, repo.Name)
			}
		}
	}
	b.WriteString("\n")

	// 8. Summary
	fmt.Fprintf(&b, "%s=== Summary ===%s\n", tBold, tReset)
	fmt.Fprintf(&b, "Issues:   %s%d%s\n", tIssue, report.Summary.Issues, tReset)
	fmt.Fprintf(&b, "Warnings: %s%d%s\n", tWarn, report.Summary.Warnings, tReset)
	fmt.Fprintf(&b, "Info:     %s%d%s\n", tInfo, report.Summary.Info, tReset)

	if report.Summary.Issues > 0 {
		b.WriteString("\nIssues indicate missing or broken configuration that should be fixed.\n")
	}
	if report.Summary.Warnings > 0 {
		b.WriteString("\nWarnings indicate deviations from the expected pattern for the repo type.\n")
		b.WriteString("Some may be intentional — review each case.\n")
	}

	return b.String()
}

// RenderMarkdown produces markdown output (no ANSI colors) matching audit.sh --format markdown.
func RenderMarkdown(report *AuditReport) string {
	if report == nil {
		return "No audit data available."
	}

	var b strings.Builder

	fmt.Fprintf(&b, "# Prow Merge Bot Configuration Audit\n\n")
	fmt.Fprintf(&b, "**Source:** %s\n\n", report.Source)
	fmt.Fprintf(&b, "**Date:** %s\n\n", report.Timestamp.Format("2006-01-02T15:04:05Z"))
	if report.RateLimit != nil {
		fmt.Fprintf(&b, "**API Rate Limit:** %d/%d remaining (resets %s)\n\n",
			report.RateLimit.Remaining, report.RateLimit.Limit,
			report.RateLimit.ResetAt.Format("15:04 UTC"))
	}

	for _, g := range report.Groups {
		fmt.Fprintf(&b, "## %s\n\n", g.Name)
		fmt.Fprintf(&b, "%s\n\n", g.Description)

		for _, repo := range g.Repos {
			worst := repo.WorstSeverity()
			icon := SeverityIcon(worst)
			fmt.Fprintf(&b, "### %s %s\n\n", icon, repo.Name)

			if !repo.HasConfig {
				b.WriteString("[ISSUE] No prow config found.\n\n")
				continue
			}

			for _, f := range repo.Findings {
				prefix := "[" + f.Severity.String() + "]"
				fmt.Fprintf(&b, "- %s %s\n", prefix, f.Message)
				if f.SuggestedFix != "" {
					fmt.Fprintf(&b, "  ```yaml\n")
					for _, line := range strings.Split(f.SuggestedFix, "\n") {
						fmt.Fprintf(&b, "  %s\n", line)
					}
					fmt.Fprintf(&b, "  ```\n")
				}
			}
			b.WriteString("\n")
		}
	}

	// Cross-repo comparison
	if len(report.Comparison) > 0 {
		b.WriteString("## Cross-Repo Field Comparison (Outliers)\n\n")
		for _, groupDef := range DefaultGroups() {
			fields, ok := report.Comparison[groupDef.Type]
			if !ok {
				continue
			}
			fmt.Fprintf(&b, "### %s\n\n", groupDef.Name)
			for field, fc := range fields {
				if len(fc.Outliers) == 0 {
					continue
				}
				fmt.Fprintf(&b, "**%s** (majority: %s)\n\n", field, fc.Majority)
				for _, outlier := range fc.Outliers {
					fmt.Fprintf(&b, "- %s: %s\n", outlier.Repo, outlier.Value)
				}
				b.WriteString("\n")
			}
		}
	}

	// Tide Branch Coverage
	b.WriteString("## Tide Branch Coverage\n\n")
	b.WriteString("Branches included in Tide merge queries for each repo:\n\n")
	for _, g := range report.Groups {
		for _, repo := range g.Repos {
			if !repo.HasConfig {
				continue
			}
			if len(repo.Tide.IncludedBranches) > 0 {
				fmt.Fprintf(&b, "- **%s**: %s\n", repo.Name, strings.Join(repo.Tide.IncludedBranches, ", "))
			} else {
				fmt.Fprintf(&b, "- **%s**: ⚠ No includedBranches found\n", repo.Name)
			}
		}
	}
	b.WriteString("\n")

	// Summary
	fmt.Fprintf(&b, "## Summary\n\n")
	fmt.Fprintf(&b, "- Issues: %d\n", report.Summary.Issues)
	fmt.Fprintf(&b, "- Warnings: %d\n", report.Summary.Warnings)
	fmt.Fprintf(&b, "- Info: %d\n", report.Summary.Info)

	if report.Summary.Issues > 0 {
		b.WriteString("\nIssues indicate missing or broken configuration that should be fixed.\n")
	}
	if report.Summary.Warnings > 0 {
		b.WriteString("\nWarnings indicate deviations from the expected pattern for the repo type.\n")
		b.WriteString("Some may be intentional — review each case.\n")
	}

	return b.String()
}

// RenderTextPlain produces plain text output (no ANSI colors) suitable for clipboard.
func RenderTextPlain(report *AuditReport) string {
	if report == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Prow Merge Bot Configuration Audit\n")
	fmt.Fprintf(&b, "Source: %s\n", report.Source)
	fmt.Fprintf(&b, "Date: %s\n", report.Timestamp.Format("2006-01-02T15:04:05Z"))
	if report.RateLimit != nil {
		fmt.Fprintf(&b, "API: %d/%d remaining (resets %s)\n",
			report.RateLimit.Remaining, report.RateLimit.Limit,
			report.RateLimit.ResetAt.Format("15:04 UTC"))
	}
	b.WriteString("\n")
	for gi := range report.Groups {
		b.WriteString(RenderGroupText(report, gi))
	}
	fmt.Fprintf(&b, "Summary: %d issues, %d warnings, %d info\n",
		report.Summary.Issues, report.Summary.Warnings, report.Summary.Info)
	return b.String()
}

// RenderRepoText renders a single repo's audit findings as plain text (no ANSI).
func RenderRepoText(report *AuditReport, repoName string) string {
	if report == nil {
		return ""
	}
	for _, g := range report.Groups {
		for _, repo := range g.Repos {
			if repo.Name != repoName {
				continue
			}
			var b strings.Builder
			fmt.Fprintf(&b, "%s (%s)\n", repo.Name, g.Name)
			if !repo.HasConfig {
				b.WriteString("  No prow config found.\n")
				return b.String()
			}
			for _, f := range repo.Findings {
				fmt.Fprintf(&b, "  [%s] %s\n", f.Severity.String(), f.Message)
			}
			// Fields
			for _, field := range []string{
				"require_self_approval", "review_acts_as_lgtm", "enforce_admins",
				"required_approving_review_count", "dismiss_stale_reviews",
				"allow_force_pushes", "merge_method",
			} {
				val := repo.Fields[field]
				if val == "" {
					val = "NOT_SET"
				}
				fmt.Fprintf(&b, "  %-32s %s\n", field+":", val)
			}
			// Plugins
			approve := "no"
			if repo.Plugins.HasApprovePlugin || repo.Plugins.HasApproveSection {
				approve = "yes"
			}
			lgtm := "no"
			if repo.Plugins.HasLgtmSection {
				lgtm = "yes"
			}
			fmt.Fprintf(&b, "  Plugins: approve=%s lgtm=%s\n", approve, lgtm)
			// Tide
			if len(repo.Tide.IncludedBranches) > 0 {
				fmt.Fprintf(&b, "  Tide branches: %s\n", strings.Join(repo.Tide.IncludedBranches, ", "))
			} else {
				b.WriteString("  Tide branches: none\n")
			}
			// OWNERS
			if repo.Owners.HasFile {
				fmt.Fprintf(&b, "  %s: %d approvers, %d reviewers\n",
					repo.Owners.FilePath, len(repo.Owners.Approvers), len(repo.Owners.Reviewers))
			} else {
				b.WriteString("  OWNERS: missing\n")
			}
			return b.String()
		}
	}
	return ""
}

// RenderGroupText renders all repos in a group as plain text (no ANSI).
func RenderGroupText(report *AuditReport, groupIdx int) string {
	if report == nil || groupIdx < 0 || groupIdx >= len(report.Groups) {
		return ""
	}
	g := report.Groups[groupIdx]
	var b strings.Builder
	fmt.Fprintf(&b, "=== %s ===\n", g.Name)
	fmt.Fprintf(&b, "%s\n\n", g.Description)
	for _, repo := range g.Repos {
		b.WriteString(RenderRepoText(report, repo.Name))
		b.WriteString("\n")
	}
	return b.String()
}

// RenderJSON marshals the report as indented JSON.
func RenderJSON(report *AuditReport) (string, error) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling report to JSON: %w", err)
	}
	return string(data), nil
}
