APPS := player subtitles dashboard

.PHONY: hooks check verify-changed max-loc quality quality-static container-test test-instance-check packages-check tooling-check worktree-lease worktree-heartbeat worktree-audit worktree-cleanup agent-finish

hooks:
	@hooks="$$(git rev-parse --git-common-dir)/hooks"; \
	install -m 755 scripts/tooling/pre-commit.sh "$$hooks/pre-commit"; \
	install -m 755 scripts/tooling/pre-push-main.sh "$$hooks/pre-push"

define run_all
	@set -eu; for app in $(APPS); do \
		echo "==> $$app: $(1)"; \
		$(2) $(MAKE) -C "apps/$$app" "$(1)"; \
	done
endef

packages-check:
	@$(MAKE) -C packages check

tooling-check:
	@python3 -m unittest discover -s scripts/ci -p 'test_*.py'
	@./scripts/tooling/test-source-tools.sh
	@./scripts/tooling/test-scan-deployment-image.sh
	@python3 scripts/quality/test_dependency_integrity.py
	@python3 scripts/quality/test_check_duplicates.py
	@GOWORK=off go -C scripts/quality/metrics test ./...
	@shellcheck -x scripts/tooling/*.sh
	@./scripts/tooling/worktree_guard.py audit
	@./scripts/tooling/test-worktree-guard.py
	@python3 scripts/tooling/test-pre-push-environment.py
	@./scripts/tooling/test-quality-controls.sh
	@python3 scripts/tooling/test-gates-paused.py
	@./scripts/tooling/test-check-go-loc.sh
	@python3 scripts/tooling/test-file-loc.py
	@python3 ./scripts/tooling/test-container-context-input.py
	@./scripts/tooling/test-go-coverage-input.sh
	@pnpm --dir scripts/quality install --frozen-lockfile
	@node scripts/tooling/test-script-lint.mjs
	@node --test scripts/quality/browser-script-bundles.test.mjs
	@python3 scripts/tooling/test-verify-deleted-e2e.py
	@./scripts/tooling/test-architecture-explorer.py
	@./scripts/tooling/generate-architecture-explorer.py player --check
	@./scripts/tooling/generate-architecture-explorer.py subtitles --check
	@./scripts/tooling/test-deploy-nox-app.sh
	@./scripts/tooling/test-deploy-nox-remote.sh
	@./scripts/tooling/test-nox-autodeploy.sh
	@python3 scripts/tooling/test-deploy-apple-devices.py
	@./scripts/ci/validate-workflows.sh

check: tooling-check packages-check
	$(call run_all,check)

verify-changed: tooling-check packages-check
	$(call run_all,verify-changed,KINOSAIL_PACKAGES_VERIFIED=1)

max-loc:
	@./scripts/quality/check-loc.sh

quality-static:
	@./scripts/quality/check-static.sh

quality:
	@./scripts/quality/check-full.sh

container-test:
	@./scripts/tooling/test-container-context.sh
	$(call run_all,container-test)

test-instance-check:
	$(call run_all,test-instance-check)

worktree-lease:
	@./scripts/tooling/worktree_guard.py lease --task "$(TASK)"

worktree-heartbeat:
	@./scripts/tooling/worktree_guard.py heartbeat --task "$(TASK)"

worktree-audit:
	@./scripts/tooling/worktree_guard.py audit

worktree-cleanup:
	@./scripts/tooling/worktree_guard.py cleanup

agent-finish:
	@./scripts/tooling/worktree_guard.py finish --task "$(TASK)"

# The file cap is enabled; preserve the pause for all other gates.
packages-check tooling-check check verify-changed quality-static quality container-test test-instance-check: SHELL := $(abspath scripts/tooling/gate-shell.sh)
