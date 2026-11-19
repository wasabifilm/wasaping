VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := windows/amd64 windows/arm64 darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

.PHONY: build test release deb macapp icon oui winicon clean

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
	@go run ./scripts/debgen -bin dist/wasaping-linux-amd64 -arch amd64 -version $(VERSION) -out dist
	@go run ./scripts/debgen -bin dist/wasaping-linux-arm64 -arch arm64 -version $(VERSION) -out dist
	@VERSION=$(VERSION) ./scripts/build_macapp.sh
	@rm -rf dist/wasaping.app
	@cd dist && (sha256sum * 2>/dev/null || shasum -a 256 *) > SHA256SUMS

# .deb packages for Debian, Ubuntu and Raspberry Pi OS (amd64, arm64)
deb:
	@mkdir -p dist
	@for a in amd64 arm64; do \
		CGO_ENABLED=0 GOOS=linux GOARCH=$$a go build -trimpath -ldflags "$(LDFLAGS)" -o dist/wasaping-linux-$$a . || exit 1; \
		go run ./scripts/debgen -bin dist/wasaping-linux-$$a -arch $$a -version $(VERSION) -out dist || exit 1; \
	done

macapp:
	@VERSION=$(VERSION) ./scripts/build_macapp.sh

icon:
	python3 scripts/make_icon.py

# icon of the Windows .exe (rsrc_windows_*.syso, linked automatically)
winicon:
	go run ./scripts/winres

# refresh the vendor database built into the program (scanner/oui.txt.gz)
oui:
	go run ./scripts/ouigen

clean:
	rm -rf dist wasaping wasaping.exe
