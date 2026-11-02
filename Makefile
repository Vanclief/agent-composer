# agc build entry points. The control panel (panel/) is a Vite app the agc
# binary embeds from panel/dist, so every target that builds agc builds the
# panel first. Without it agc still compiles and /panel/ answers 503.

.PHONY: panel types panel-check build install test rig-deploy rig-deploy-test

# Panel production build into panel/dist.
panel: panel/node_modules
	cd panel && npm run build

# Installs panel dependencies from package-lock.json when they are missing or
# older than the lockfile (a merged branch that added a dependency).
panel/node_modules: panel/package-lock.json
	cd panel && npm ci
	touch panel/node_modules

# Regenerates panel/src/api/schema.d.ts from docs/openapi.yaml.
types: panel/node_modules
	cd panel && npm run types

# Types drift, tsc, eslint and the Playwright smoke tests.
panel-check: panel/node_modules
	cd panel && npx playwright install chromium
	cd panel && npm run check

build: panel
	go build -o bin/agc .

# go install would name the binary after the module directory
# (agent-composer), so build straight to GOBIN/agc instead.
GOBIN ?= $(shell go env GOPATH)/bin

install: panel
	go build -o $(GOBIN)/agc .

test:
	go test ./...
	$(MAKE) panel-check

rig-deploy:
	scripts/rig-deploy.sh

# The deploy script's user units and restart path, against stub commands.
rig-deploy-test:
	scripts/rig-deploy_test.sh
