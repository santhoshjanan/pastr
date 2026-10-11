# Pinned tool versions. staticcheck is a main-branch build, because no release
# can read this Go version's packages yet; move to a release when one can.
STATICCHECK ?= v0.7.0-0.dev.0.20261009230814-452d5bb86b45
GOVULNCHECK ?= v1.9.0

.PHONY: test race lint run build docker

test:
	CGO_ENABLED=0 go test ./...

race:
	go test -race ./...

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK) ./...
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK) ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/pastr ./cmd/pastr

run: build
	PASTR_ADDR=127.0.0.1:8080 PASTR_BASE_URL=http://localhost:8080 PASTR_DB_PATH=./data/pastr.db \
	PASTR_ADMIN_USERNAME=admin PASTR_ADMIN_PASSWORD=change-me-please ./bin/pastr

docker:
	docker build -t pastr:dev .
