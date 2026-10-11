# The main branch of staticcheck, until a release can read this Go version's packages.
STATICCHECK ?= master

.PHONY: test race lint run build docker

test:
	CGO_ENABLED=0 go test ./...

race:
	go test -race ./...

lint:
	test -z "$$(gofmt -l .)"
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck@$(STATICCHECK) ./...
	go run golang.org/x/vuln/cmd/govulncheck@latest ./...

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/pastr ./cmd/pastr

run: build
	PASTR_BASE_URL=http://localhost:8080 PASTR_DB_PATH=./data/pastr.db \
	PASTR_ADMIN_USERNAME=admin PASTR_ADMIN_PASSWORD=change-me-please ./bin/pastr

docker:
	docker build -t pastr:dev .
