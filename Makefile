BIN_NAME      ?= shinzo-evm-relayer
CMD_PACKAGE   ?= ./cmd/relayer
OUT_DIR       ?= build

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -X 'main.version=$(VERSION)' -X 'main.commit=$(COMMIT)' -X 'main.date=$(DATE)'

.PHONY: all build install clean tidy fmt vet test \
        build-linux-amd64 build-linux-arm64 build-darwin-arm64 release

all: build

build:
	@mkdir -p $(OUT_DIR)
	@echo ">> building $(BIN_NAME) ($(VERSION))"
	@go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BIN_NAME) $(CMD_PACKAGE)

install:
	@echo ">> installing $(BIN_NAME) ($(VERSION))"
	@go install -ldflags "$(LDFLAGS)" $(CMD_PACKAGE)

clean:
	@echo ">> cleaning"
	@rm -rf $(OUT_DIR)

tidy:
	@go mod tidy

fmt:
	@echo ">> go fmt"
	@go fmt ./...

vet:
	@echo ">> go vet"
	@go vet ./...

test:
	@go test ./...

build-linux-amd64:
	@mkdir -p $(OUT_DIR)
	@echo ">> linux_amd64"
	@GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BIN_NAME)_linux_amd64 $(CMD_PACKAGE)

build-linux-arm64:
	@mkdir -p $(OUT_DIR)
	@echo ">> linux_arm64"
	@GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BIN_NAME)_linux_arm64 $(CMD_PACKAGE)

build-darwin-arm64:
	@mkdir -p $(OUT_DIR)
	@echo ">> darwin_arm64"
	@GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(OUT_DIR)/$(BIN_NAME)_darwin_arm64 $(CMD_PACKAGE)

release: build-linux-amd64 build-linux-arm64 build-darwin-arm64
	@echo ">> artifacts in $(OUT_DIR)"
