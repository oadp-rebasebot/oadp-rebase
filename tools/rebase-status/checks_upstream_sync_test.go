package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCheckUpstreamSync(t *testing.T) {
	tests := []struct {
		name       string
		upstream   string
		response   string
		wantStatus Status
	}{
		{"upstream contained", "https://github.com/kubevirt/kubevirt-velero-plugin:release-v0.8", `{"status":"ahead"}`, StatusOK},
		{"upstream has new commits", "https://github.com/kubevirt/kubevirt-velero-plugin:release-v0.8", `{"status":"diverged"}`, StatusFail},
		{"downstream only", "", "", StatusNA},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.upstream == "" {
					t.Errorf("unexpected request: %s", r.URL.Path)
					return
				}
				if r.URL.Path != "/repos/kubevirt/kubevirt-velero-plugin/compare/release-v0.8...migtools:oadp-1.5" {
					t.Errorf("request path = %q", r.URL.Path)
				}
				_, _ = w.Write([]byte(tt.response))
			}))
			defer server.Close()

			client := &GitHubClient{httpClient: server.Client(), baseURL: server.URL, cache: make(map[string][]byte)}
			result := checkUpstreamSync(client, &RepoSpec{
				Org: "migtools", Repo: "kubevirt-velero-plugin", Branch: "oadp-1.5", Upstream: tt.upstream,
			})
			if result.Status != tt.wantStatus {
				t.Errorf("status = %v, want %v", result.Status, tt.wantStatus)
			}
		})
	}
}
