package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
)

// CheckVeleroTagAlignment computes velero tag alignment for a branch.
// Returns nil if no VELERO_TAG_SHA is set (oadp-dev) or no repos have velero deps.
func CheckVeleroTagAlignment(
	statuses []RepoStatus,
	versionsVars map[string]string,
	client *GitHubClient,
	branch string,
) *VeleroTagAlignment {
	if versionsVars == nil {
		return nil
	}

	tagSHA := versionsVars["VELERO_TAG_SHA"]
	if tagSHA == "" {
		return nil
	}
	upstreamTag := versionsVars["VELERO_UPSTREAM_TAG"]

	type repoVeleroDep struct {
		orgRepo    string
		pinnedHash string
	}
	var veleroDeps []repoVeleroDep

	for _, rs := range statuses {
		if rs.Spec.Skip {
			continue
		}
		if rs.Spec.Org == "openshift" && rs.Spec.Repo == "velero" {
			continue
		}
		for _, ds := range rs.DepSyncs {
			if ds.Org == "openshift" && ds.Repo == "velero" && ds.HaveHash != "" {
				veleroDeps = append(veleroDeps, repoVeleroDep{
					orgRepo:    rs.Spec.FullName(),
					pinnedHash: ds.HaveHash,
				})
				break
			}
		}
	}

	if len(veleroDeps) == 0 {
		return nil
	}

	latestCommitSHA, latestErr := client.HeadCommitSHA("openshift", "velero", branch)
	latestError := ""
	if latestErr != nil || latestCommitSHA == "" {
		if latestErr != nil {
			latestError = latestErr.Error()
		} else {
			latestError = fmt.Sprintf("branch %q not found", branch)
		}
		fmt.Fprintf(os.Stderr, "warning: latest openshift/velero commit: %s\n", latestError)
	}

	repos := make([]VeleroTagRepo, len(veleroDeps))
	var wg sync.WaitGroup

	for i, dep := range veleroDeps {
		wg.Add(1)
		go func(idx int, d repoVeleroDep) {
			defer wg.Done()
			parts := strings.SplitN(d.orgRepo, "/", 2)
			repos[idx] = VeleroTagRepo{
				Org:        parts[0],
				Repo:       parts[1],
				PinnedHash: d.pinnedHash,
				AtLatest:   latestError == "" && strings.HasPrefix(latestCommitSHA, d.pinnedHash),
			}

			status, err := client.CompareStatus("openshift", "velero", tagSHA, d.pinnedHash)
			if err != nil {
				repos[idx].CompareError = err.Error()
				fmt.Fprintf(os.Stderr, "warning: velero tag alignment: %s: %v\n", d.orgRepo, err)
				return
			}
			repos[idx].Aligned = (status == "identical" || status == "ahead")
		}(i, dep)
	}

	wg.Wait()

	sort.Slice(repos, func(i, j int) bool {
		if repos[i].Aligned != repos[j].Aligned {
			return !repos[i].Aligned
		}
		return repos[i].Org+"/"+repos[i].Repo < repos[j].Org+"/"+repos[j].Repo
	})

	allAligned := true
	for _, r := range repos {
		if !r.Aligned {
			allAligned = false
			break
		}
	}

	return &VeleroTagAlignment{
		VeleroTag:       upstreamTag,
		VeleroTagSHA:    tagSHA,
		LatestCommitSHA: latestCommitSHA,
		LatestError:     latestError,
		Repos:           repos,
		AllAligned:      allAligned,
	}
}
