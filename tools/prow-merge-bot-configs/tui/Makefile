.PHONY: build clean run cross-build package

BINARY=prow-audit-tui
DIST=dist

PLATFORMS=linux darwin windows
ARCHES=amd64 arm64

build:
	go build -o $(BINARY) .

cross-build:
	@mkdir -p $(DIST)
	@for os in $(PLATFORMS); do \
		for arch in $(ARCHES); do \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			out=$(DIST)/$(BINARY)-$$os-$$arch$$ext; \
			echo "Building $$out"; \
			GOOS=$$os GOARCH=$$arch go build -o $$out . || exit 1; \
		done; \
	done

package: cross-build
	@for os in $(PLATFORMS); do \
		for arch in $(ARCHES); do \
			ext=""; \
			if [ "$$os" = "windows" ]; then ext=".exe"; fi; \
			bin=$(BINARY)-$$os-$$arch$$ext; \
			if [ "$$os" = "windows" ]; then \
				echo "Packaging $(DIST)/$(BINARY)-$$os-$$arch.zip"; \
				(cd $(DIST) && zip $(BINARY)-$$os-$$arch.zip $$bin && rm -f $$bin); \
			else \
				echo "Packaging $(DIST)/$(BINARY)-$$os-$$arch.tar.gz"; \
				tar -czf $(DIST)/$(BINARY)-$$os-$$arch.tar.gz -C $(DIST) $$bin && rm -f $(DIST)/$$bin; \
			fi; \
		done; \
	done

clean:
	rm -f $(BINARY)
	rm -rf $(DIST)

run: build
	./$(BINARY)
