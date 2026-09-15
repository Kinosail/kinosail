.PHONY: check consumer-test coverage deadcode lint max-loc test test-race tidy-check vuln

check: max-loc tidy-check lint deadcode coverage test-race vuln consumer-test

max-loc:
	@../scripts/tooling/check-go-loc.sh packages 300

tidy-check:
	@go mod tidy -diff

lint:
	@golangci-lint run

deadcode:
	@cd .. && go run golang.org/x/tools/cmd/deadcode@v0.49.1-0.20260904150557-664b13f4863b -filter='github.com/MikeO7/kinosail/packages' ./apps/player/... ./apps/subtitles/... ./apps/dashboard/...

coverage:
	@../scripts/tooling/test-go-coverage.sh packages "$${KINOSAIL_PACKAGES_MIN_COVERAGE:-85.0}"

test:
	@go test ./...

test-race:
	@go test -race ./...

vuln:
	@govulncheck ./...

consumer-test:
	@cd ../apps/player && go test -run '^$$' ./...
	@cd ../apps/subtitles && go test -run '^$$' ./...
	@cd ../apps/dashboard && go test -run '^$$' ./...

# Preserve recipes; suppress every gate until the user removes .gates-disabled.
check max-loc tidy-check lint deadcode coverage test test-race vuln consumer-test: SHELL := $(abspath ../scripts/tooling/gate-shell.sh)
