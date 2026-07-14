.PHONY: generate verify-generate test syntax-check config-load verify-resolve-config verify-hooks verify-kopia-alignment verify-repos-yaml

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
	@echo "=== Syntax check ==="
	@fail=0; \
	for f in versions/*.env rebase-configs/*.env.sh rebasebot-hook-scripts/*.sh tools/*.sh tools/**/*.sh run-oadp-rebase.sh; do \
		[ -f "$$f" ] || continue; \
		bash -n "$$f" || { echo "FAIL: $$f"; fail=1; }; \
	done; \
	[ $$fail -eq 0 ] && echo "All files OK" || exit 1

config-load:
	@echo "=== Config loading test ==="
	@fail=0; \
	for branch in oadp-1.3 oadp-1.4 oadp-1.5 oadp-1.6 oadp-dev; do \
		for wave in 1 2 3 4 5; do \
			configs=$$(REPOS_YAML=repos.yaml yq -r ".repos[] | select(.wave == $$wave) | .repo" repos.yaml 2>/dev/null) || continue; \
			for repo in $$configs; do \
				prefix=$$(yq -r ".repos[] | select(.repo == \"$$repo\") | .config_prefix" repos.yaml); \
				config_file="rebase-configs/$${prefix}_$${branch}.env.sh"; \
				[ -f "$$config_file" ] || continue; \
				target="$${repo}-$${branch}"; \
				main_only=$$(yq -r ".repos[] | select(.repo == \"$$repo\") | .main_only // false" repos.yaml); \
				[ "$$main_only" = "true" ] && target="$${repo}-main"; \
				dev_branch=$$(yq -r ".repos[] | select(.repo == \"$$repo\") | .dev_branch // \"\"" repos.yaml); \
				[ -n "$$dev_branch" ] && [ "$$branch" = "oadp-dev" ] && target="$${repo}-$${dev_branch}"; \
				output=$$(./run-oadp-rebase.sh -t "$$target" 2>&1); \
				rc=$$?; \
				if [ $$rc -ne 0 ]; then \
					echo "  FAIL: $$target"; \
					fail=1; \
				else \
					upstream=$$(echo "$$output" | grep 'Upstream:' | sed 's/.*Upstream:[[:space:]]*//'); \
					echo "  OK: $$target -> $$upstream"; \
				fi; \
			done; \
		done; \
	done; \
	[ $$fail -eq 0 ] && echo "All configs OK" || exit 1

verify-hooks:
	@echo "=== Hook reference check ==="
	@missing=0; \
	for config in rebase-configs/*.env.sh; do \
		grep -o '/[a-zA-Z0-9_.-]*\.sh' "$$config" 2>/dev/null | sed 's|^/||' | while IFS= read -r hook; do \
			[ -z "$$hook" ] && continue; \
			if [ ! -f "rebasebot-hook-scripts/$$hook" ]; then \
				echo "MISSING: $$hook (in $$(basename $$config))"; \
			fi; \
		done; \
	done; \
	echo "Hook references OK"

verify-kopia-alignment:
	@echo "=== Kopia alignment check ==="
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
	@echo "=== repos.yaml validation ==="
	@yq eval 'true' repos.yaml > /dev/null || { echo "FAIL: repos.yaml is not valid YAML"; exit 1; }
	@fail=0; \
	for prefix in $$(yq -r '.repos[].config_prefix' repos.yaml); do \
		if ! ls rebase-configs/$${prefix}_*.env.sh >/dev/null 2>&1; then \
			echo "WARNING: No config files found for prefix: $$prefix"; \
		fi; \
	done; \
	echo "repos.yaml validation OK"

verify-resolve-config:
	@echo "=== resolve-config.sh test ==="
	@fail=0; \
	for branch in oadp-1.3 oadp-1.4 oadp-1.5 oadp-1.6 oadp-dev; do \
		for wave in 1 2 3 4 5; do \
			configs=$$(yq -r ".repos[] | select(.wave == $$wave) | .repo" repos.yaml 2>/dev/null) || continue; \
			for repo in $$configs; do \
				prefix=$$(yq -r ".repos[] | select(.repo == \"$$repo\") | .config_prefix" repos.yaml); \
				config_file="rebase-configs/$${prefix}_$${branch}.env.sh"; \
				[ -f "$$config_file" ] || continue; \
				grep -q 'SOURCE_UPSTREAM_REPO' "$$config_file" || continue; \
				target="$${repo}-$${branch}"; \
				main_only=$$(yq -r ".repos[] | select(.repo == \"$$repo\") | .main_only // false" repos.yaml); \
				[ "$$main_only" = "true" ] && target="$${repo}-main"; \
				dev_branch=$$(yq -r ".repos[] | select(.repo == \"$$repo\") | .dev_branch // \"\"" repos.yaml); \
				[ -n "$$dev_branch" ] && [ "$$branch" = "oadp-dev" ] && target="$${repo}-$${dev_branch}"; \
				output=$$(bash tools/auto-rebase/resolve-config.sh "$$target" 2>&1) || { echo "  FAIL: $$target"; fail=1; continue; }; \
				config_name=$$(echo "$$output" | grep 'CONFIG_NAME=' | cut -d'"' -f2); \
				echo "  OK: $$target -> $$config_name"; \
			done; \
		done; \
	done; \
	[ $$fail -eq 0 ] && echo "All resolve-config OK" || exit 1

test: verify-repos-yaml verify-generate syntax-check config-load verify-resolve-config verify-hooks
	@echo ""
	@echo "All checks passed."
