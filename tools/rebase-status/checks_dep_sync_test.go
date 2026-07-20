package main

import "testing"

func TestInitSpecBranchMap(t *testing.T) {
	specs := []RepoSpec{
		{Org: "openshift", Repo: "velero", Branch: "oadp-dev"},
		{Org: "migtools", Repo: "kubevirt-velero-plugin", Branch: "main"},
		{Org: "openshift", Repo: "oadp-operator", Branch: "oadp-dev"},
	}

	initSpecBranchMap(specs)

	tests := []struct {
		key  string
		want string
	}{
		{"openshift/velero", "oadp-dev"},
		{"migtools/kubevirt-velero-plugin", "main"},
		{"openshift/oadp-operator", "oadp-dev"},
	}
	for _, tt := range tests {
		if got := specBranchMap[tt.key]; got != tt.want {
			t.Errorf("specBranchMap[%q] = %q, want %q", tt.key, got, tt.want)
		}
	}
}

func TestInitSpecBranchMapClearsOldEntries(t *testing.T) {
	initSpecBranchMap([]RepoSpec{
		{Org: "openshift", Repo: "velero", Branch: "oadp-1.5"},
	})
	if got := specBranchMap["openshift/velero"]; got != "oadp-1.5" {
		t.Fatalf("setup: specBranchMap[openshift/velero] = %q, want oadp-1.5", got)
	}

	initSpecBranchMap([]RepoSpec{
		{Org: "migtools", Repo: "kopia", Branch: "oadp-1.6"},
	})
	if _, ok := specBranchMap["openshift/velero"]; ok {
		t.Error("old entry openshift/velero should be gone after re-init")
	}
	if got := specBranchMap["migtools/kopia"]; got != "oadp-1.6" {
		t.Errorf("specBranchMap[migtools/kopia] = %q, want oadp-1.6", got)
	}
}
