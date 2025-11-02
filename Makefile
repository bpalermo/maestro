.PHONY: gazelle
gazelle:
	@bazel run //:gazelle

.PHONY: tidy
tidy: gazelle
	@bazel mod tidy

.PHONY: build
build:
	@bazel build //...

.PHONY: test
test: gazelle
	@bazel test --test_output=all //...

.PHONY: load
load-image: gazelle
	@bazel run //:load -- --repository maestro --tag latest

.PHONY: push
push-image:
	@bazel run //:push -- --repository maestro --tag latest

.PHONY: run-agent
run-agent:
	@bazel run //maestro-agent/cmd/server

.PHONY: clean
clean:
	@bazel clean --expunge
