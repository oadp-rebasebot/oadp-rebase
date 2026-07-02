package main

import (
	"testing"
)

func TestParseGoModRequires(t *testing.T) {
	tests := []struct {
		name string
		gomod string
		want map[string]string
	}{
		{
			name: "block form with direct and indirect",
			gomod: `module example.com/foo

go 1.25.0

require (
	google.golang.org/grpc v1.81.1
	golang.org/x/net v0.55.0
	github.com/stretchr/testify v1.11.1 // indirect
)
`,
			want: map[string]string{
				"google.golang.org/grpc": "v1.81.1",
				"golang.org/x/net":      "v0.55.0",
			},
		},
		{
			name: "single-line require",
			gomod: `module example.com/foo

require github.com/pkg/errors v0.9.1
`,
			want: map[string]string{
				"github.com/pkg/errors": "v0.9.1",
			},
		},
		{
			name: "multiple require blocks",
			gomod: `module example.com/foo

require (
	google.golang.org/grpc v1.81.1
)

require (
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0
)
`,
			want: map[string]string{
				"google.golang.org/grpc": "v1.81.1",
				"golang.org/x/sys":      "v0.45.0",
			},
		},
		{
			name: "empty go.mod",
			gomod: `module example.com/foo
`,
			want: map[string]string{},
		},
		{
			name: "only indirect deps",
			gomod: `module example.com/foo

require (
	golang.org/x/net v0.55.0 // indirect
)
`,
			want: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGoModRequires(tt.gomod)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %v", len(got), len(tt.want), got)
			}
			for mod, wantVer := range tt.want {
				if gotVer, ok := got[mod]; !ok {
					t.Errorf("missing module %s", mod)
				} else if gotVer != wantVer {
					t.Errorf("module %s: got %s, want %s", mod, gotVer, wantVer)
				}
			}
		})
	}
}

func TestParseGoModReplaces(t *testing.T) {
	tests := []struct {
		name  string
		gomod string
		want  map[string]replaceEntry
	}{
		{
			name: "standard replace",
			gomod: `module example.com/foo

replace github.com/vmware-tanzu/velero => github.com/openshift/velero v0.10.2-0.20250520-abc12345
`,
			want: map[string]replaceEntry{
				"github.com/vmware-tanzu/velero": {ReplacementModule: "github.com/openshift/velero", Version: "v0.10.2-0.20250520-abc12345"},
			},
		},
		{
			name: "versioned LHS replace",
			gomod: `module example.com/foo

replace github.com/kopia/kopia v0.22.3 => github.com/migtools/kopia v0.0.0-20250520-def456
`,
			want: map[string]replaceEntry{
				"github.com/kopia/kopia": {ReplacementModule: "github.com/migtools/kopia", Version: "v0.0.0-20250520-def456"},
			},
		},
		{
			name: "block form replace",
			gomod: `module example.com/foo

replace (
	github.com/vmware-tanzu/velero => github.com/openshift/velero v0.10.2-0.20250520-abc123
	github.com/kopia/kopia => github.com/migtools/kopia v0.0.0-20250520-def456
)
`,
			want: map[string]replaceEntry{
				"github.com/vmware-tanzu/velero": {ReplacementModule: "github.com/openshift/velero", Version: "v0.10.2-0.20250520-abc123"},
				"github.com/kopia/kopia":         {ReplacementModule: "github.com/migtools/kopia", Version: "v0.0.0-20250520-def456"},
			},
		},
		{
			name: "local path replace skipped",
			gomod: `module example.com/foo

replace github.com/foo/bar => ./staging/bar
`,
			want: map[string]replaceEntry{},
		},
		{
			name: "no replaces",
			gomod: `module example.com/foo
`,
			want: map[string]replaceEntry{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseGoModReplaces(tt.gomod)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d: %v", len(got), len(tt.want), got)
			}
			for mod, wantEntry := range tt.want {
				if gotEntry, ok := got[mod]; !ok {
					t.Errorf("missing module %s", mod)
				} else {
					if gotEntry.ReplacementModule != wantEntry.ReplacementModule {
						t.Errorf("module %s: replacement got %s, want %s", mod, gotEntry.ReplacementModule, wantEntry.ReplacementModule)
					}
					if gotEntry.Version != wantEntry.Version {
						t.Errorf("module %s: version got %s, want %s", mod, gotEntry.Version, wantEntry.Version)
					}
				}
			}
		})
	}
}

func TestCompareGoModVersions(t *testing.T) {
	noRepl := map[string]replaceEntry{}

	tests := []struct {
		name         string
		downReqs     map[string]string
		downReplaces map[string]replaceEntry
		upReqs       map[string]string
		upReplaces   map[string]replaceEntry
		wantCount    int
		wantModules  []string // modules that should appear in drift
	}{
		{
			name:         "downstream older — drift detected",
			downReqs:     map[string]string{"google.golang.org/grpc": "v1.79.3"},
			downReplaces: noRepl,
			upReqs:       map[string]string{"google.golang.org/grpc": "v1.81.1"},
			upReplaces:   noRepl,
			wantCount:    1,
			wantModules:  []string{"google.golang.org/grpc"},
		},
		{
			name:         "downstream newer — no drift",
			downReqs:     map[string]string{"google.golang.org/grpc": "v1.82.0"},
			downReplaces: noRepl,
			upReqs:       map[string]string{"google.golang.org/grpc": "v1.81.1"},
			upReplaces:   noRepl,
			wantCount:    0,
		},
		{
			name:         "identical versions — no drift",
			downReqs:     map[string]string{"google.golang.org/grpc": "v1.81.1"},
			downReplaces: noRepl,
			upReqs:       map[string]string{"google.golang.org/grpc": "v1.81.1"},
			upReplaces:   noRepl,
			wantCount:    0,
		},
		{
			name:         "module only in upstream — no drift",
			downReqs:     map[string]string{},
			downReplaces: noRepl,
			upReqs:       map[string]string{"google.golang.org/grpc": "v1.81.1"},
			upReplaces:   noRepl,
			wantCount:    0,
		},
		{
			name:         "module only in downstream — no drift",
			downReqs:     map[string]string{"example.com/extra": "v1.0.0"},
			downReplaces: noRepl,
			upReqs:       map[string]string{},
			upReplaces:   noRepl,
			wantCount:    0,
		},
		{
			name: "pseudo-versions — older timestamp is drift",
			downReqs: map[string]string{
				"golang.org/x/crypto": "v0.0.0-20250101000000-aaa111aaa111",
			},
			downReplaces: noRepl,
			upReqs: map[string]string{
				"golang.org/x/crypto": "v0.0.0-20250601000000-bbb222bbb222",
			},
			upReplaces:  noRepl,
			wantCount:   1,
			wantModules: []string{"golang.org/x/crypto"},
		},
		{
			name: "same-repo replace pins older — drift detected",
			downReqs: map[string]string{
				"github.com/kopia/kopia": "v0.22.3",
			},
			downReplaces: map[string]replaceEntry{
				"github.com/kopia/kopia": {ReplacementModule: "github.com/migtools/kopia", Version: "v0.0.0-20250101000000-old123old123"},
			},
			upReqs: map[string]string{
				"github.com/kopia/kopia": "v0.22.3",
			},
			upReplaces: map[string]replaceEntry{
				"github.com/kopia/kopia": {ReplacementModule: "github.com/migtools/kopia", Version: "v0.0.0-20250601000000-new456new456"},
			},
			wantCount:   1,
			wantModules: []string{"github.com/kopia/kopia"},
		},
		{
			name: "fork-based replace skipped — downstream replaces to different repo",
			downReqs: map[string]string{
				"github.com/vmware-tanzu/velero": "v1.18.0",
			},
			downReplaces: map[string]replaceEntry{
				"github.com/vmware-tanzu/velero": {ReplacementModule: "github.com/openshift/velero", Version: "v0.10.2-0.20260701-abc123ab"},
			},
			upReqs: map[string]string{
				"github.com/vmware-tanzu/velero": "v1.18.0",
			},
			upReplaces: noRepl,
			wantCount:  0,
		},
		{
			name: "invalid version — skipped",
			downReqs: map[string]string{
				"example.com/bad": "not-a-version",
			},
			downReplaces: noRepl,
			upReqs: map[string]string{
				"example.com/bad": "v1.0.0",
			},
			upReplaces: noRepl,
			wantCount:  0,
		},
		{
			name: "multiple drifts",
			downReqs: map[string]string{
				"google.golang.org/grpc":   "v1.79.3",
				"go.opentelemetry.io/otel": "v1.42.0",
				"golang.org/x/net":         "v0.55.0",
			},
			downReplaces: noRepl,
			upReqs: map[string]string{
				"google.golang.org/grpc":   "v1.81.1",
				"go.opentelemetry.io/otel": "v1.44.0",
				"golang.org/x/net":         "v0.55.0",
			},
			upReplaces:  noRepl,
			wantCount:   2,
			wantModules: []string{"google.golang.org/grpc", "go.opentelemetry.io/otel"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			drifts := compareGoModVersions(tt.downReqs, tt.downReplaces, tt.upReqs, tt.upReplaces)
			if len(drifts) != tt.wantCount {
				t.Fatalf("got %d drifts, want %d: %+v", len(drifts), tt.wantCount, drifts)
			}
			if tt.wantModules != nil {
				driftMods := make(map[string]bool)
				for _, d := range drifts {
					driftMods[d.Module] = true
				}
				for _, mod := range tt.wantModules {
					if !driftMods[mod] {
						t.Errorf("expected drift for %s but not found", mod)
					}
				}
			}
		})
	}
}
