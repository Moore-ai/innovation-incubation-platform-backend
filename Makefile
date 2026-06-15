.PHONY: run build test test-integration swagger clean

# Use GOTOOLCHAIN=local to avoid version mismatch
GOTOOLCHAIN=local

run:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go run ./cmd/server/

build:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go build -o bin/server ./cmd/server/

test:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go test ./internal/... -count=1 -v

test-integration:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go test ./integration/... -tags=integration -count=1 -v

test-coverage:
	GOTOOLCHAIN=$(GOTOOLCHAIN) go test ./internal/... -coverprofile=coverage.out
	GOTOOLCHAIN=$(GOTOOLCHAIN) go tool cover -html=coverage.out

swagger:
	swag init -g cmd/server/main.go -o docs

clean:
	rm -rf bin/ coverage.out
