# agc build entry points. The control panel (panel/) is a Vite app the agc
# binary embeds from panel/dist, so every target that builds agc builds the
# panel first. Without it agc still compiles and /panel/ answers 503.

.PHONY: panel types panel-check build install test rig-deploy

# Panel production build into panel/dist.
panel: | panel/node_modules
	cd panel && npm run build

# Installs panel dependencies from package-lock.json when they are missing.
panel/node_modules:
	cd panel && npm ci

# Regenerates panel/src/api/schema.d.ts from docs/openapi.yaml.
types: | panel/node_modules
	cd panel && npm run types

# Types drift, tsc, eslint and the Playwright smoke tests.
panel-check: | panel/node_modules
	cd panel && npx playwright install chromium
	cd panel && npm run check

build: panel
	go build -o bin/agc .

install: panel
	go install .

test:
	go test ./...
	$(MAKE) panel-check

rig-deploy:
	scripts/rig-deploy.sh
