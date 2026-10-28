default:
    @just --list

build:
    go build -trimpath -o dist/mufradat ./cmd/mufradat

test:
    go test -race ./...

vet:
    go vet ./...
    go vet -tags integration ./...

integration:
    go test -tags integration -count=1 ./...

lint:
    golangci-lint run
    go run ./cmd/nocomments .

python-test:
    cd python && python -m pytest -q

web-test:
    cd internal/review && node --test jstest/*.test.js && tsc -p .

check: vet lint test python-test web-test

ranking:
    go run ./cmd/mkranking

snapshot:
    goreleaser release --snapshot --clean
