run:
	@go run ./cmd/tgp

build:
	@go build -v -o ./bin/ ./cmd/tgp

test:
	@golangci-lint run
	@go test -v ./... -cover -race

act:
	@act -l --container-architecture linux/amd64
	@act --container-architecture linux/amd64
