ROOT_DIR := $(abspath .)
BIN_DIR			:= $(ROOT_DIR)/bin
ARTIFACTS_DIR	:= $(ROOT_DIR)/artifacts
CONTRACTS_DIR	:= contracts

TINYGO			:= tinygo
WASM_FLAGS		:= -gc=custom -scheduler=none -panic=trap -no-debug -target=wasm-unknown

TINYJSON		:= $(BIN_DIR)/tinyjson
TINYJSON_FLAGS	:= -snake_case -no_std_marshalers -pkg

.PHONY: test build clean contracts services sdk tinyjson

# Test all components
test:
	cd contracts/btc-mapping && tinygo test -v ./...
	go test -v ./services/...
	go test -v ./sdk/go/...
	cd sdk/ts && npm test

# Build all services
build: services sdk

# Build service binaries
services:
	cd services/oracle && go build -o ../../bin/oracle ./cmd
	cd services/router && go build -o ../../bin/router ./cmd
	cd services/indexer && go build -o ../../bin/indexer ./cmd

# Build SDK libraries
sdk:
	cd sdk/go && go build ./...
	cd sdk/ts && npm run build

# Build contracts
contracts:
	@mkdir -p $(ARTIFACTS_DIR)
	@set +e; \
	for dir in $(CONTRACTS_DIR)/*; do \
		if [ -d "$$dir" ] && [ -f "$$dir/main.go" ]; then \
			name=$$(basename $$dir); \
			wasm_file="$(ARTIFACTS_DIR)/$$name.wasm"; \
			\
			# Check if wasm exists and find if any file in 'dir' is newer \
			if [ -f "$$wasm_file" ] && [ -z "$$(find $$dir -type f -newer $$wasm_file)" ]; then \
				echo "  ⏩ $$name is up to date, skipping"; \
				continue; \
			fi; \
			\
			echo "Building contract $$name"; \
			( \
				cd $$dir && \
				$(TINYGO) build $(WASM_FLAGS) \
					-o $$wasm_file . && \
				echo "  ✅ $$name compiled"; \
			) || echo "  ⚠️  Failed to build $$name (continuing)"; \
		fi; \
	done

# Regenerate tinyjson code for contracts
tinyjson:
	@set +e; \
	echo "Searching for //tinyjson:json directives..."; \
	\
	# Ensure tinyjson exists \
	if ! command -v $(TINYJSON) >/dev/null 2>&1; then \
		echo "tinyjson not found, installing..."; \
		go install github.com/CosmWasm/tinyjson@latest || \
			{ echo "Failed to install tinyjson"; exit 1; }; \
	fi; \
	\
	# Find all directories containing files with tinyjson tags \
	find $(CONTRACTS_DIR) -type f -name '*.go' -print0 | \
	xargs -0 grep -l '^//tinyjson:json$$' | \
	xargs -n1 dirname | \
	sort -u | \
	awk '{ if ($$0 ~ /\/sdk$$/) print "0" $$0; else print "1" $$0 }' | \
	sort | \
	sed 's/^.//' | \
	while read dir; do \
		echo "Processing package in $$dir"; \
		\
		# Ensure cleanup happens on exit \
		trap 'rm -rf "$$dir/.tinyjson-tmp" 2>/dev/null' EXIT INT TERM; \
		\
		# Find the output file (assume it's *_tinyjson.go) \
		out=$$(find "$$dir" -maxdepth 1 -name '*_tinyjson.go' -print -quit); \
		\
		rebuild=0; \
		\
		# Rebuild if output missing \
		if [ -z "$$out" ] || [ ! -f "$$out" ]; then \
			echo "  Output file missing or not found"; \
			rebuild=1; \
		\
		# Rebuild if any source file with tinyjson tag is newer than output (excluding _tinyjson.go files) \
		elif find "$$dir" -maxdepth 1 -name '*.go' ! -name '*_tinyjson.go' -type f -newer "$$out" -exec grep -l '^//tinyjson:json$$' {} + | grep -q . ; then \
			echo "  Source files newer than output"; \
			rebuild=1; \
		\
		# Rebuild if output contains stub marshalers \
		elif [ -f "$$out" ] && \
			! awk '/func[[:space:]]+\([^)]+\)[[:space:]]+MarshalTinyJSON\(/ { inside=1; next } \
			inside && /\{\}[[:space:]]*$$/ { exit 1 } inside && /\{/ { inside=0 }' "$$out" && \
			! awk '/func[[:space:]]+\([^)]+\)[[:space:]]+UnmarshalTinyJSON\(/ { inside=1; next } \
			inside && /\{\}[[:space:]]*$$/ { exit 1 } inside && /\{/ { inside=0 }' "$$out"; then \
			echo "  ⚠️  stub-only function detected"; \
			rebuild=1; \
		fi; \
		\
		if [ "$$rebuild" -eq 0 ]; then \
			echo "  ⏩ $$out is up to date, skipping"; \
			continue; \
		fi; \
		\
		# Find a source file with tinyjson tag to use as template \
		src=$$(find "$$dir" -maxdepth 1 -name '*.go' -type f -exec grep -l '^//tinyjson:json$$' {} \; | head -n 1); \
		\
		if [ -z "$$src" ]; then \
			echo "  ⚠️  No source file with tinyjson tag found in $$dir"; \
			continue; \
		fi; \
		\
		# Get the original package name \
		original_pkg=$$(awk '/^package / {print $$2; exit}' "$$src"); \
		\
		if [ -z "$$original_pkg" ]; then \
			echo "  ⚠️  Could not determine package name in $$src"; \
			continue; \
		fi; \
		\
		if (mkdir -p "$$dir/.tinyjson-tmp" && \
			find "$$dir" -maxdepth 1 -name '*.go' ! -name '*_tinyjson.go' ! -name '*_test.go' -exec cp {} "$$dir/.tinyjson-tmp/" \; && \
			cd "$$dir/.tinyjson-tmp" && \
			for f in *.go; do \
				awk '/^package / {sub(/package .*/, "package tinyjson")} {print}' "$$f" > "$$f.tmp" && mv "$$f.tmp" "$$f"; \
			done && \
			$(TINYJSON) $(TINYJSON_FLAGS) -pkg -output_filename "$${original_pkg}_tinyjson.go" && \
			for f in *_tinyjson.go; do \
				[ -f "$$f" ] && sed "s/package tinyjson/package $$original_pkg/" "$$f" > "../$$f"; \
			done && \
			cd .. && \
			rm -rf .tinyjson-tmp); then \
			echo "  ✅  tinyjson created for $$dir" ; \
		else \
			echo "  ⚠️  tinyjson failed for $$dir (continuing)"; \
			rm -rf "$$dir/.tinyjson-tmp" 2>/dev/null; \
		fi; \
	done

# Clean build artifacts
clean:
	rm -rf bin/
	cd sdk/ts && npm run clean

# Install development dependencies
setup:
	go mod download
	cd sdk/ts && npm install
	# Install tinygo if not present
	which tinygo || echo "Please install TinyGo: https://tinygo.org/getting-started/install/"
	# Check for tinyjson binary
	[ -f bin/tinyjson ] || echo "tinyjson binary included in bin/tinyjson"

# Run E2E tests
e2e:
	cd e2e && go test -v -timeout 10m ./...

# Run unit tests
test:
	cd contracts/dex-router/test && go test -v ./...

# Run unit tests with coverage
test-cover:
	cd contracts/dex-router/test && go test -cover -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run E2E tests
e2e:
	./test-dex-e2e.sh

# Run demo
demo:
	node demo-dex.js

# Format code
fmt:
	go fmt ./...
	cd sdk/ts && npm run fmt



