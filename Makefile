.PHONY: build test vet fmt lint docker-build

build:
	go build ./...

test:
	go test ./...

integration-test:
	go test -tags integration ./... -run TestIntegration -v

vet:
	go vet ./...

fmt:
	gofmt -l .

lint: fmt vet

docker-build:
	docker build -t ai-code-reviewer:local .
