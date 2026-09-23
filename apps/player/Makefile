.PHONY: bootstrap package-installer deploy-nox hooks install-nox-autodeploy browser-test check check-fast client-check container-test coverage deadcode installer-test native-build format lint local-pipeline local-pipeline-test performance-test media-server-benchmark-test playback-test test test-instance-check test-race tidy-check verify-changed verify-watch vuln secrets max-loc single-container shellcheck actionlint

bootstrap:
	brew bundle
	$(MAKE) hooks

hooks:
	@$(MAKE) -C ../.. hooks

package-installer:
	@python3 scripts/package-installer.py "$(OUTPUT)"

deploy-nox:
	@./scripts/deploy-nox.sh

install-nox-autodeploy:
	@./scripts/install-nox-autodeploy.sh

browser-test:
	@pnpm --dir e2e install --frozen-lockfile
	@CI=1 KINOSAIL_BROWSER_TEST=1 ./scripts/test-container.sh

client-check:
	@./apps/native/scripts/build-apple.sh ios
	@./apps/native/scripts/build-apple.sh tvos

client-build-ios:
	@./apps/native/scripts/build-apple.sh ios

client-build-tvos:
	@./apps/native/scripts/build-apple.sh tvos

.PHONY: client-build-ios client-build-tvos

check: check-fast test-race vuln

check-fast: max-loc single-container tidy-check lint deadcode coverage installer-test native-build shellcheck actionlint secrets local-pipeline-test

container-test:
	@./scripts/test-container.sh

coverage:
	@../../scripts/tooling/test-go-coverage.sh apps/player "$${KINOSAIL_MIN_COVERAGE:-89.0}"

deadcode:
	@../../scripts/tooling/with-go-module.sh go run golang.org/x/tools/cmd/deadcode@v0.49.1-0.20260904150557-664b13f4863b -filter='github.com/MikeO7/kinosail-player' ./...

installer-test:
	@python3 scripts/test-package-installer.py
	@./scripts/test-installer.sh
	@./scripts/test-remote-setup.sh

native-build:
	@./scripts/test-native-builds.sh

local-pipeline:
	@./scripts/local-pipeline.sh

local-pipeline-test:
	@./scripts/test-browser-projects.sh
	@./scripts/test-deploy-nox.sh
	@./scripts/test-verify-changed.sh

format:
	@../../scripts/tooling/with-go-module.sh golangci-lint fmt

lint:
	@../../scripts/tooling/with-go-module.sh golangci-lint run

performance-test:
	@../../scripts/tooling/with-go-module.sh go test github.com/MikeO7/kinosail/packages/library -run '^$$' -bench '^(BenchmarkScan|BenchmarkOrganize)$$' -benchmem -count=3
	@../../scripts/tooling/with-go-module.sh go test ./internal/server -run '^$$' -bench '^(BenchmarkLargeLibraryBrowse|BenchmarkSaveProgress10K|BenchmarkLibraryFind|BenchmarkLibrarySearch10K|BenchmarkProbeDecoration|BenchmarkPlaybackAdmissionWhileBackgroundWorkRuns)$$' -benchmem -count=3

media-server-benchmark-test:
	@python3 scripts/test_media_server_benchmark.py

playback-test:
	@./scripts/test-playback.sh

test:
	@../../scripts/tooling/with-go-module.sh go test ./...

test-instance-check:
	@./scripts/test-test-instance.sh

test-race:
	@../../scripts/tooling/with-go-module.sh go test -race ./...

verify-changed:
	@./scripts/verify-changed.sh "$${BASE:-origin/main}"

verify-watch:
	@./scripts/verify-watch.sh "$${BASE:-origin/main}"

tidy-check:
	@../../scripts/tooling/with-go-module.sh go mod tidy -diff

vuln:
	@../../scripts/tooling/with-go-module.sh govulncheck ./...

secrets:
	gitleaks git --config ../../.gitleaks.toml --no-banner --redact --gitleaks-ignore-path ../../.gitleaksignore .

max-loc:
	@../../scripts/quality/check-loc.sh --scope apps/player

single-container:
	@./scripts/check-single-container.sh

shellcheck:
	@../../scripts/tooling/shellcheck-tracked.sh

actionlint:
	actionlint

# The file cap is enabled; preserve the pause for all other gates.
browser-test client-check check check-fast container-test coverage deadcode installer-test native-build local-pipeline local-pipeline-test lint performance-test media-server-benchmark-test playback-test test test-instance-check test-race verify-changed verify-watch tidy-check vuln secrets single-container shellcheck actionlint: SHELL := $(abspath ../../scripts/tooling/gate-shell.sh)
