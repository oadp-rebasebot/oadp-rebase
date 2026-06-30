.PHONY: generate verify-generate test syntax-check config-load verify-hooks

generate:
	@bash tools/generate-verify-tag-sha.sh
	@bash tools/generate-version-matrix.sh

verify-generate: generate
	@if ! git diff --quiet -- rebasebot-hook-scripts/verify-tag-sha_*.sh docs/version-matrix.md; then \
		echo ""; \
		echo "ERROR: Generated files are out of date."; \
		echo "Run 'make generate' and commit the changes."; \
		echo ""; \
		git diff --stat -- rebasebot-hook-scripts/verify-tag-sha_*.sh docs/version-matrix.md; \
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
		for config in $$(grep -E "^\s+[a-z].*-$${branch}\)" run-oadp-rebase.sh | sed 's/).*//' | awk '{print $$1}'); do \
			output=$$(./run-oadp-rebase.sh -t "$$config" 2>&1); \
			rc=$$?; \
			if [ $$rc -ne 0 ]; then \
				echo "  FAIL: $$config"; \
				fail=1; \
			else \
				upstream=$$(echo "$$output" | grep 'Upstream:' | sed 's/.*Upstream:[[:space:]]*//'); \
				echo "  OK: $$config -> $$upstream"; \
			fi; \
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

test: verify-generate syntax-check config-load verify-hooks
	@echo ""
	@echo "All checks passed."
