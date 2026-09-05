BIN := bin/pulumi-resource-coolify
VERSION ?= $(shell node -p "require('./pulumi-plugin.json').version")

.PHONY: build test vet schema gen-sdk sdk-nodejs patch-sdk install-local publish-sdk-npm clean

build:
	go build -o $(BIN) ./cmd/pulumi-resource-coolify

test:
	go test ./...

vet:
	go vet ./...

schema: build
	pulumi package get-schema $(BIN) > schema.json

# Regenerate the checked-in TypeScript SDK and normalize its package name so it
# can be consumed either from the local path or published to npm.
gen-sdk: sdk-nodejs patch-sdk

sdk-nodejs: build
	rm -rf sdk/nodejs
	pulumi package gen-sdk ./$(BIN) --language nodejs --out sdk
	rm -rf sdk/nodejs/node_modules

patch-sdk:
	cp LICENSE sdk/nodejs/LICENSE
	node patch-sdk.js
	cd sdk/nodejs && npm install --package-lock-only --ignore-scripts --no-audit --no-fund

# Build the plugin and install it into the local Pulumi plugin cache so `pulumi`
# resolves it without a GitHub release. Run this after $make build on the main
# branch; consumers reference the SDK with `"@bambamboole/coolify": "file:..."`.
install-local: build
	@pulumi plugin rm resource coolify $(VERSION) --yes 2>/dev/null || true
	rm -rf bin/plugin
	mkdir -p bin/plugin
	cp $(BIN) bin/plugin/pulumi-resource-coolify
	cp pulumi-plugin.json bin/plugin/pulumi-plugin.json
	tar czf bin/pulumi-resource-coolify-$(VERSION).tar.gz -C bin/plugin pulumi-resource-coolify pulumi-plugin.json
	pulumi plugin install resource coolify $(VERSION) --file bin/pulumi-resource-coolify-$(VERSION).tar.gz

publish-sdk-npm:
	cd sdk/nodejs && npm publish --access public

clean:
	rm -rf bin sdk schema.json