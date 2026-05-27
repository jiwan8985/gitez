BINARY  := gez
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"
UNAME   := $(shell uname -s 2>/dev/null || echo Windows)

.PHONY: all build install clean release \
        build-mac-arm build-mac-amd build-linux build-windows

# ── Default ────────────────────────────────────────────────────────────────────
all: build

# ── Local build (current OS/arch) ─────────────────────────────────────────────
build:
ifeq ($(UNAME),Windows)
	go build $(LDFLAGS) -o $(BINARY).exe .
	@echo Done: $(BINARY).exe
else
	go build $(LDFLAGS) -o $(BINARY) .
	@echo Done: $(BINARY)
endif

# ── Install (Mac / Linux) ──────────────────────────────────────────────────────
install: build
ifeq ($(UNAME),Darwin)
	@mkdir -p /usr/local/bin
	cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "✔  설치 완료: /usr/local/bin/$(BINARY)"
else ifeq ($(UNAME),Linux)
	@mkdir -p /usr/local/bin
	cp $(BINARY) /usr/local/bin/$(BINARY)
	@echo "✔  설치 완료: /usr/local/bin/$(BINARY)"
else
	@echo "Windows: gez.exe 를 PATH 가 포함된 폴더에 복사하세요"
	@echo "  예) copy gez.exe C:\\Windows\\System32\\gez.exe"
endif

# ── Cross-compile targets ──────────────────────────────────────────────────────
build-mac-arm:
	@mkdir -p dist
	GOOS=darwin  GOARCH=arm64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-arm64 .

build-mac-amd:
	@mkdir -p dist
	GOOS=darwin  GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-darwin-amd64 .

build-linux:
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-linux-amd64 .

build-windows:
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o dist/$(BINARY)-windows-amd64.exe .

# ── Release (all platforms) ────────────────────────────────────────────────────
release: build-mac-arm build-mac-amd build-linux build-windows
	@echo ""
	@echo "✔  전체 플랫폼 빌드 완료:"
	@ls -lh dist/

# ── Clean ──────────────────────────────────────────────────────────────────────
clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/
