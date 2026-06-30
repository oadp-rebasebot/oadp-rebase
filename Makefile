.PHONY: generate verify-generate

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
