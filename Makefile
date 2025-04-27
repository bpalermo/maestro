CURRENT_DIR = $(shell pwd)

.PHONY: lint-fix
lint-fix:
	@docker run \
		-e DEFAULT_BRANCH=main \
		-e RUN_LOCAL=true \
		-e FIX_SHELL_SHFMT=true \
		-e FIX_YAML_PRETTIER=true \
		-v "$(CURRENT_DIR)":/tmp/lint \
		--rm \
		ghcr.io/super-linter/super-linter:v7.3.0
