.PHONY: generate verify-generate test syntax-check config-load verify-resolve-config verify-hooks verify-kopia-alignment verify-repos-yaml go-test auto-rebase-test cve-scan-test shellcheck

generate:
	@bash tools/generate-go-replace-velero.sh
	@bash tools/generate-verify-tag-sha.sh
	@bash tools/generate-version-matrix.sh
	@bash tools/generate-cve-scan-image-map.sh

verify-generate: generate
	@if ! git diff --quiet -- rebasebot-hook-scripts/go-replace_velero_*.sh rebasebot-hook-scripts/verify-tag-sha_*.sh docs/version-matrix.md tools/cve-scan/image-map.sh; then \
		echo ""; \
		echo "ERROR: Generated files are out of date."; \
		echo "Run 'make generate' and commit the changes."; \
		echo ""; \
		git diff --stat -- rebasebot-hook-scripts/go-replace_velero_*.sh rebasebot-hook-scripts/verify-tag-sha_*.sh docs/version-matrix.md tools/cve-scan/image-map.sh; \
		exit 1; \
	fi
	@echo "Generated files are up to date."

syntax-check:
	@echo "=== Syntax check — bash -n on all shell scripts ==="
	@fail=0; \
	for f in versions/*.env rebase-configs/*.env.sh rebasebot-hook-scripts/*.sh tools/*.sh tools/**/*.sh run-oadp-rebase.sh; do \
		[ -f "$$f" ] || continue; \
		bash -n "$$f" || { echo "FAIL: $$f"; fail=1; }; \
	done; \
	[ $$fail -eq 0 ] && echo "All files OK" || exit 1

config-load:
	@echo "=== Config load — source every config for every branch via run-oadp-rebase.sh -t ==="
	@fail_file=/tmp/oadp-config-load-$$$$; rm -f "$$fail_file"; \
	repo_data=$$(yq -r '.repos[] | [.repo, .config_prefix, (.main_only // false), (.dev_branch // "_NONE_"), (.min_branch // "_NONE_"), (.max_branch // "_NONE_")] | join("\t")' repos.yaml); \
	for branch in oadp-1.3 oadp-1.4 oadp-1.5 oadp-1.6 oadp-dev; do \
		echo "$$repo_data" | while IFS='	' read -r repo prefix main_only dev_br min_br max_br; do \
			[ -z "$$repo" ] && continue; \
			effective_branch="$$branch"; \
			if [ "$$main_only" = "true" ]; then \
				[ "$$branch" != "oadp-dev" ] && continue; \
				effective_branch="main"; \
			fi; \
			[ "$$dev_br" != "_NONE_" ] && [ "$$branch" = "oadp-dev" ] && effective_branch="$$dev_br"; \
			config_file="rebase-configs/$${prefix}_$${effective_branch}.env.sh"; \
			[ -f "$$config_file" ] || continue; \
			target="$${repo}-$${effective_branch}"; \
			output=$$(./run-oadp-rebase.sh -t "$$target" 2>&1); \
			rc=$$?; \
			if [ $$rc -ne 0 ]; then \
				echo "  FAIL: $$target"; \
				touch "$$fail_file"; \
			else \
				upstream=$$(echo "$$output" | grep 'Upstream:' | sed 's/.*Upstream:[[:space:]]*//'); \
				echo "  OK: $$target -> $$upstream"; \
			fi; \
		done; \
	done; \
	if [ -f "$$fail_file" ]; then rm -f "$$fail_file"; exit 1; fi; \
	echo "All configs OK"

verify-hooks:
	@echo "=== Hook references — every hook filename in configs exists in rebasebot-hook-scripts/ ==="
	@fail=0; \
	for config in rebase-configs/*.env.sh; do \
		for hook in $$(grep -o '/[a-zA-Z0-9_.-]*\.sh' "$$config" 2>/dev/null | sed 's|^/||'); do \
			[ -z "$$hook" ] && continue; \
			if [ ! -f "rebasebot-hook-scripts/$$hook" ]; then \
				echo "MISSING: $$hook (in $$(basename $$config))"; \
				fail=1; \
			fi; \
		done; \
	done; \
	[ $$fail -eq 0 ] && echo "Hook references OK" || { echo "ERROR: Missing hook scripts detected"; exit 1; }

verify-kopia-alignment:
	@echo "=== Kopia alignment — KOPIA_UPSTREAM_TAG matches what Velero's go.mod expects ==="
	@fail=0; \
	for f in versions/oadp-1.*.env; do \
		unset OADP_BRANCH VELERO_UPSTREAM_TAG KOPIA_UPSTREAM_TAG 2>/dev/null; \
		. "$$f"; \
		[ -z "$${VELERO_UPSTREAM_TAG:-}" ] && continue; \
		resolved=$$(bash tools/resolve-kopia-tag.sh "$$VELERO_UPSTREAM_TAG" 2>/dev/null) || { echo "  FAIL: $$OADP_BRANCH — could not resolve kopia for $$VELERO_UPSTREAM_TAG"; fail=1; continue; }; \
		if [ "$$resolved" = "$$KOPIA_UPSTREAM_TAG" ]; then \
			echo "  OK: $$OADP_BRANCH — $$KOPIA_UPSTREAM_TAG matches $$VELERO_UPSTREAM_TAG"; \
		else \
			echo "  MISMATCH: $$OADP_BRANCH — SSOT has $$KOPIA_UPSTREAM_TAG but Velero $$VELERO_UPSTREAM_TAG expects $$resolved"; \
			fail=1; \
		fi; \
	done; \
	[ $$fail -eq 0 ] && echo "Kopia alignment OK" || exit 1

verify-repos-yaml:
	@echo "=== repos.yaml — valid YAML and every config_prefix has matching config files ==="
	@yq eval 'true' repos.yaml > /dev/null || { echo "FAIL: repos.yaml is not valid YAML"; exit 1; }
	@fail=0; \
	for prefix in $$(yq -r '.repos[].config_prefix' repos.yaml); do \
		if ! ls rebase-configs/$${prefix}_*.env.sh >/dev/null 2>&1; then \
			echo "WARNING: No config files found for prefix: $$prefix"; \
		fi; \
	done; \
	echo "repos.yaml validation OK"

verify-resolve-config:
	@echo "=== resolve-config — every target resolves to the correct config name ==="
	@fail_file=/tmp/oadp-resolve-config-$$$$; rm -f "$$fail_file"; \
	repo_data=$$(yq -r '.repos[] | [.repo, .config_prefix, (.main_only // false), (.dev_branch // "_NONE_"), (.min_branch // "_NONE_"), (.max_branch // "_NONE_")] | join("\t")' repos.yaml); \
	for branch in oadp-1.3 oadp-1.4 oadp-1.5 oadp-1.6 oadp-dev; do \
		echo "$$repo_data" | while IFS='	' read -r repo prefix main_only dev_br min_br max_br; do \
			[ -z "$$repo" ] && continue; \
			effective_branch="$$branch"; \
			if [ "$$main_only" = "true" ]; then \
				[ "$$branch" != "oadp-dev" ] && continue; \
				effective_branch="main"; \
			fi; \
			[ "$$dev_br" != "_NONE_" ] && [ "$$branch" = "oadp-dev" ] && effective_branch="$$dev_br"; \
			config_file="rebase-configs/$${prefix}_$${effective_branch}.env.sh"; \
			[ -f "$$config_file" ] || continue; \
			grep -q 'SOURCE_UPSTREAM_REPO' "$$config_file" || continue; \
			target="$${repo}-$${effective_branch}"; \
			output=$$(bash tools/auto-rebase/resolve-config.sh "$$target" 2>&1) || { echo "  FAIL: $$target"; touch "$$fail_file"; continue; }; \
			config_name=$$(echo "$$output" | grep 'CONFIG_NAME=' | cut -d'"' -f2); \
			echo "  OK: $$target -> $$config_name"; \
		done; \
	done; \
	if [ -f "$$fail_file" ]; then rm -f "$$fail_file"; exit 1; fi; \
	echo "All resolve-config OK"

go-test:
	@echo "=== Go tests — unit tests for all Go tools ==="
	@fail=0; \
	for mod in tools/rebase-status tools/semver-compare tools/prow-merge-bot-configs/tui; do \
		echo "  Testing $$mod..."; \
		(cd "$$mod" && go test ./...) || { echo "  FAIL: $$mod"; fail=1; }; \
	done; \
	[ $$fail -eq 0 ] && echo "All Go tests passed" || exit 1

auto-rebase-test:
	@echo "=== Auto-rebase tests — decision, triage, and notification logic ==="
	@bash tools/auto-rebase/test.sh

cve-scan-test:
	@echo "=== CVE scan tests — remediation selection logic ==="
	@bash tools/cve-scan/test-remediate.sh

shellcheck:
	@echo "=== ShellCheck — lint all shell scripts ==="
	@command -v shellcheck >/dev/null 2>&1 || { echo "Error: shellcheck is not installed"; exit 1; }
	@fail=0; \
	for f in run-oadp-rebase.sh tools/*.sh tools/**/*.sh rebasebot-hook-scripts/*.sh; do \
		[ -f "$$f" ] || continue; \
		shellcheck -S warning "$$f" || fail=1; \
	done; \
	[ $$fail -eq 0 ] && echo "All files passed ShellCheck" || exit 1

test: verify-repos-yaml verify-generate syntax-check config-load verify-resolve-config verify-hooks go-test auto-rebase-test cve-scan-test
	@echo ""
	@echo "All checks passed."
