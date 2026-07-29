SHELL := /bin/sh
.DEFAULT_GOAL := check

.PHONY: format format-check tidy-check vet lint test test-race docs hooks-check pre-commit pre-push hooks check

format:
	gofmt -w .

format-check:
	@files="$$(gofmt -l .)"; \
	if [ -n "$$files" ]; then \
		printf '%s\n' "Go files need formatting:" "$$files"; \
		exit 1; \
	fi

tidy-check:
	go mod tidy -diff

vet:
	go vet ./...

lint: format-check vet

test:
	go test ./...

test-race:
	go test -race ./...

docs:
	go run ./tools/validate-docs

hooks-check:
	@test -x .githooks/pre-commit
	@test -x .githooks/pre-push
	@sh -n .githooks/pre-commit .githooks/pre-push

pre-commit:
	.githooks/pre-commit

pre-push:
	.githooks/pre-push </dev/null

hooks:
	git config core.hooksPath .githooks
	@printf '%s\n' "Git hooks installed from .githooks"

check: tidy-check lint test test-race docs hooks-check
