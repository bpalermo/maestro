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

.PHONY: generate-certs
generate-certs:
	@openssl genpkey -algorithm RSA -out ./certs/tls.key -pkeyopt rsa_keygen_bits:2048
	@openssl req -new -key ./certs/tls.key -out ./certs/tls.csr -subj "/CN=maestro-server.maestro.svc" -addext "subjectAltName = DNS:maestro-server.maestro.svc.cluster.local"
	@openssl req -x509 -sha256 -newkey rsa:2048 -keyout ./certs/ca.key -out ./certs/ca.crt -days 650 -subj "/CN=RootCA" -nodes
	@openssl x509 -copy_extensions copy -req -CA ./certs/ca.crt -CAkey ./certs/ca.key -in ./certs/tls.csr -out ./certs/tls.crt -days 650 -CAcreateserial
