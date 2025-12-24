APP_NAME := matching-engine
CMD_PATH := ./cmd/matching-engine
BUILD_DIR := bin
ROOT_DIR := ../..

post-install:
	@echo "➡ Generating go.mod for proto-generated code..."
	@mkdir -p $(ROOT_DIR)/packages/proto-defs/go/generated
	@printf "module github.com/sameerkrdev/nerve/packages/proto-defs/go/generated\n\ngo 1.21\n" > \
	$(ROOT_DIR)/packages/proto-defs/go/generated/go.mod

dev:
	air

build:
	@echo "➡ Building $(APP_NAME)..."
	go build -o $(BUILD_DIR)/$(APP_NAME) $(CMD_PATH)

tidy:
	@echo "➡ Tidying modules..."
	go mod tidy
