.PHONY: build clean sign

# Plugin details
PLUGIN_NAME := example_custom_plugin
VERSION := 1.0.0
OUTPUT := $(PLUGIN_NAME).so

# Build the plugin as a .so file
build:
	@echo "Building custom plugin..."
	go build -buildmode=plugin -o $(OUTPUT) plugin.go
	@echo "Plugin built successfully: $(OUTPUT)"

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f $(OUTPUT) $(OUTPUT).sig private.key public.key
	@echo "Clean complete"

# Generate a key pair for signing (development only)
generate-keys:
	@echo "Generating ED25519 key pair..."
	@go run generate_keys.go
	@echo "Keys generated: private.key, public.key"

# Sign the plugin (requires private.key)
sign: build
	@echo "Signing plugin..."
	@go run sign_plugin.go $(OUTPUT) private.key
	@echo "Plugin signed: $(OUTPUT).sig"

# Build and sign in one step
release: build sign
	@echo "Release build complete!"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod download
	@echo "Dependencies installed"

# Run a quick test build to verify the plugin compiles
test-build:
	@echo "Testing plugin build..."
	go build -buildmode=plugin -o /tmp/test_$(OUTPUT) plugin.go
	@rm -f /tmp/test_$(OUTPUT)
	@echo "Test build successful!"

help:
	@echo "Available targets:"
	@echo "  build          - Build the plugin .so file"
	@echo "  clean          - Remove build artifacts"
	@echo "  generate-keys  - Generate ED25519 key pair for signing"
	@echo "  sign           - Sign the built plugin"
	@echo "  release        - Build and sign the plugin"
	@echo "  deps           - Install Go dependencies"
	@echo "  test-build     - Test that the plugin compiles"
	@echo "  help           - Show this help message"

