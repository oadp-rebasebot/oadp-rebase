package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCheckVeleroTagAlignmentChecksLatestBranchCommit(t *testing.T) {
	const latestSHA = "latest1234567890"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/openshift/velero/commits/oadp-1.6":
			_, _ = w.Write([]byte(`{"sha":"` + latestSHA + `"}`))
		case "/repos/openshift/velero/compare/required123...latest123456":
			_, _ = w.Write([]byte(`{"status":"ahead"}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := &GitHubClient{httpClient: server.Client(), baseURL: server.URL, cache: make(map[string][]byte)}
	statuses := []RepoStatus{{
		Spec:     RepoSpec{Org: "openshift", Repo: "plugin", Branch: "oadp-1.6"},
		DepSyncs: []DepSync{{Org: "openshift", Repo: "velero", HaveHash: "latest123456"}},
	}}

	alignment := CheckVeleroTagAlignment(statuses, map[string]string{
		"VELERO_UPSTREAM_TAG": "v1.16.2",
		"VELERO_TAG_SHA":      "required123",
	}, client, "oadp-1.6")

	if alignment.LatestCommitSHA != latestSHA {
		t.Errorf("LatestCommitSHA = %q, want %q", alignment.LatestCommitSHA, latestSHA)
	}
	if !alignment.Repos[0].AtLatest {
		t.Error("dependency pinned to branch HEAD should be marked latest")
	}
}

func TestRenderMarkdownVeleroTagShowsLatestCommitStatus(t *testing.T) {
	alignment := &VeleroTagAlignment{
		VeleroTag:       "v1.16.2",
		VeleroTagSHA:    "required123456",
		LatestCommitSHA: "latest123456",
		AllAligned:      true,
		Repos: []VeleroTagRepo{
			{Org: "openshift", Repo: "plugin-current", PinnedHash: "latest123456", Aligned: true, AtLatest: true},
			{Org: "openshift", Repo: "plugin-stale", PinnedHash: "required123456", Aligned: true, AtLatest: false},
		},
	}

	var output bytes.Buffer
	renderMarkdownVeleroTag(&output, alignment)

	if !strings.Contains(output.String(), "| Repo | Pinned Commit | Tag | Latest |") {
		t.Error("missing latest commit column")
	}
	if !strings.Contains(output.String(), "Latest `openshift/velero` commit: `latest123456`") {
		t.Error("missing latest commit SHA")
	}
	if !strings.Contains(output.String(), "plugin-current](https://github.com/openshift/plugin-current) | [`latest123456`]"+
		"(https://github.com/openshift/velero/commit/latest123456) | :white_check_mark: | :white_check_mark:") {
		t.Error("current commit should be marked as latest")
	}
	if !strings.Contains(output.String(), "plugin-stale](https://github.com/openshift/plugin-stale) | [`required123456`]"+
		"(https://github.com/openshift/velero/commit/required123456) | :white_check_mark: | :x:") {
		t.Error("older aligned commit should be marked as not latest")
	}
}
