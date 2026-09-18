.PHONY: build test check format architecture vuln
build:
	go build -o bin/api-engine ./cmd/api-engine
format:
	gofmt -w $$(find cmd internal tools test -name '*.go' 2>/dev/null)
architecture:
	go run ./tools/architecture-check
check: architecture
	test -z "$$(gofmt -l $$(find cmd internal tools test -name '*.go' 2>/dev/null))"
	go vet ./...
	go test ./...
	go test -race ./...
test:
	go test ./...
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...

.PHONY: coverage fuzz
coverage:
	go test -coverpkg=./internal/definition/domain,./internal/definition/application,./internal/execution/domain -coverprofile=coverage.out ./internal/definition/... ./internal/api/http/mapper/... ./internal/execution/domain ./internal/worker/...
	go run ./tools/coverage-check coverage.out
fuzz:
	go test ./internal/definition/domain -run='^$$' -fuzz=FuzzDuration -fuzztime=2s
	go test ./internal/definition/domain -run='^$$' -fuzz=FuzzSelector -fuzztime=2s
	go test ./internal/definition/domain -run='^$$' -fuzz=FuzzPathBinding -fuzztime=2s
	go test ./internal/fetch/application -run='^$$' -fuzz=FuzzRequestURLBoundary -fuzztime=2s
	go test ./internal/definition/infrastructure/schema -run='^$$' -fuzz=FuzzDecodeJSON -fuzztime=2s
