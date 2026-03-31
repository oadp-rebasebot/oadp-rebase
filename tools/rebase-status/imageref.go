package main

import "strings"

// FetchImageReferences fetches and parses bundle/image-references from the
// oadp-operator repo for the given branch. Returns nil, nil if the file
// doesn't exist (e.g., on oadp-dev).
func FetchImageReferences(client *GitHubClient, branch string) ([]ImageRefEntry, error) {
	content, err := client.FileContent("openshift", "oadp-operator", "bundle/image-references", branch)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, nil
	}
	return ParseImageReferences(string(content)), nil
}

// ParseImageReferences parses the image-references ImageStream YAML,
// extracting both active and commented-out entries. Commented entries
// are marked with CommentedOut=true.
func ParseImageReferences(content string) []ImageRefEntry {
	var entries []ImageRefEntry
	lines := strings.Split(content, "\n")

	var currentName string
	var commented bool
	var inFrom bool

	for _, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)

		// Detect and strip comment prefix
		isComment := false
		line := trimmed
		if strings.HasPrefix(line, "#") {
			isComment = true
			line = strings.TrimSpace(strings.TrimLeft(line, "#"))
			if line == "" {
				continue
			}
		}

		// Look for "- name: <art-name>"
		if strings.HasPrefix(line, "- name:") {
			currentName = strings.TrimSpace(strings.TrimPrefix(line, "- name:"))
			commented = isComment
			inFrom = false
			continue
		}

		if currentName == "" {
			continue
		}

		// Track "from:" block
		if strings.HasPrefix(line, "from:") {
			inFrom = true
			continue
		}

		// Skip "kind: DockerImage"
		if strings.HasPrefix(line, "kind:") {
			continue
		}

		// Inside from: block, capture "name:" (the image reference)
		if inFrom && strings.HasPrefix(line, "name:") {
			ref := strings.TrimSpace(strings.TrimPrefix(line, "name:"))
			entry := ImageRefEntry{
				ARTName:      currentName,
				ImageRef:     ref,
				CommentedOut: commented,
			}
			parseImageRef(&entry)
			entries = append(entries, entry)
			currentName = ""
			inFrom = false
		}
	}

	return entries
}

// parseImageRef extracts namespace, repo, and tag from an image reference
// like "quay.io/konveyor/velero-plugin-for-gcp:oadp-1.6".
func parseImageRef(entry *ImageRefEntry) {
	ref := entry.ImageRef

	// Extract tag
	colonIdx := strings.LastIndex(ref, ":")
	if colonIdx > 0 {
		entry.Tag = ref[colonIdx+1:]
		ref = ref[:colonIdx]
	}

	// Strip registry prefix (e.g., "quay.io/")
	if idx := strings.Index(ref, "/"); idx > 0 && strings.Contains(ref[:idx], ".") {
		ref = ref[idx+1:]
	}

	// First segment is namespace, rest is repo
	slashIdx := strings.Index(ref, "/")
	if slashIdx > 0 {
		entry.Namespace = ref[:slashIdx]
		entry.QuayRepo = ref[slashIdx+1:]
	}
}
