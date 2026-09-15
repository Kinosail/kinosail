.PHONY: actionlint browser-test check check-fast container-test coverage deploy-nox format hooks install-nox-autodeploy lint local-pipeline max-loc race secrets shellcheck single-container test test-instance-check test-race tidy-check verify-changed verify-watch vuln

deploy-nox:
	@./scripts/deploy-nox.sh

hooks:
	@$(MAKE) -C ../.. hooks

install-nox-autodeploy:
	@./scripts/install-nox-autodeploy.sh

test:
	@go test ./... -count=1

test-instance-check: browser-test

test-race:
	@go test -race ./... -count=1

race:
	@$(MAKE) test-race

format:
	@../../scripts/tooling/with-go-module.sh golangci-lint fmt

lint:
	@../../scripts/tooling/with-go-module.sh golangci-lint run --new-from-rev="$${BASE:-origin/main}"

tidy-check:
	@../../scripts/tooling/with-go-module.sh go mod tidy -diff

browser-test:
	@pnpm --dir e2e install --frozen-lockfile
	@CI=1 pnpm --dir e2e test

container-test:
	@./scripts/test-container.sh

max-loc:
	@../../scripts/tooling/check-go-loc.sh apps/dashboard 300

single-container:
	@./scripts/check-single-container.sh

secrets:
	@gitleaks git --config ../../.gitleaks.toml --no-banner --redact --gitleaks-ignore-path ../../.gitleaksignore .

shellcheck:
	@../../scripts/tooling/shellcheck-tracked.sh

actionlint:
	@if find .github/workflows -type f \( -name '*.yml' -o -name '*.yaml' \) -print -quit 2>/dev/null | grep -q .; then actionlint; else printf 'skip: no GitHub Actions workflows\n'; fi

vuln:
	@../../scripts/tooling/with-go-module.sh govulncheck ./...

check-fast: max-loc single-container tidy-check lint coverage shellcheck actionlint secrets
	@./scripts/test-deploy-nox.sh

check: check-fast test-race vuln

local-pipeline:
	@./scripts/local-pipeline.sh

verify-changed:
	@./scripts/verify-changed.sh "$${BASE:-origin/main}"

verify-watch:
	@while true; do ./scripts/verify-changed.sh || true; sleep 5; done

coverage:
	@go test ./... -cover

# Preserve recipes; suppress every gate until the user removes .gates-disabled.
test test-instance-check test-race race lint tidy-check browser-test container-test max-loc single-container secrets shellcheck actionlint vuln check-fast check local-pipeline verify-changed verify-watch coverage: SHELL := $(abspath ../../scripts/tooling/gate-shell.sh)
