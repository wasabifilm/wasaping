VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

.PHONY: build test release clean

build:
	go build -ldflags "$(LDFLAGS)" -o wasaping .

test:
	go vet ./...
	go test ./...

release: clean
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=""; [ $$os = windows ] && ext=".exe"; \
		echo "→ $$os/$$arch"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags "$(LDFLAGS)" \
			-o dist/wasaping-$$os-$$arch$$ext . || exit 1; \
	done
	@cd dist && (sha256sum * 2>/dev/null || shasum -a 256 *) > SHA256SUMS

clean:
	rm -rf dist wasaping
