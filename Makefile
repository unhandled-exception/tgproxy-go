.PHONY: run
run:
	@go run ./cmd/tgp

.PHONY: build
build:
	@go build -v -o ./bin/ ./cmd/tgp

.PHONY: test
test:
	@go test ./... -cover -coverpkg=./...

.PHONY: test/cover
test/cover:
	@go test -v -coverpkg=./... -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out
	@go tool cover -html=coverage.out -o coverage.html

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: act
act:
	@act -l --container-architecture linux/amd64
	@act -j tests --container-architecture linux/amd64
