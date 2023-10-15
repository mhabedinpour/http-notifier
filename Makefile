################################
# Dependency related commands
################################
.PHONY: dependency
dependency: install-golangci-lint install-mockgen

.PHONY: install-mockgen
install-mockgen:
	go install go.uber.org/mock/mockgen@v0.2.0

.PHONY: install-golangci-lint
install-golangci-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.3

################################
# CI related commands
################################

.PHONY: lint
lint:
	./scripts/lint.sh

.PHONY: test
test:
	./scripts/test.sh

.PHONY: install-hooks
install-hooks:
	cp .hooks/pre-commit .git/hooks/pre-commit
	sudo chmod +x .git/hooks/pre-commit

.PHONY: uninstall-hooks
uninstall-hooks:
	rm .git/hooks/pre-commit

.PHONY: generate
generate: install-mockgen
	mockgen -source=pkg/notifier/writer.go -destination=pkg/notifier/mock-writer.go -package=notifier
