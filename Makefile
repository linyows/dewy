TEST ?= ./...
LOGLEVEL ?= info

default: build

build:
	go build ./cmd/dewy

server:
	go run cmd/dewy/main.go server --registry ghr://linyows/dewy-testapp -p 8000 -l $(LOGLEVEL) -- $(HOME)/.go/src/github.com/linyows/dewy/current/dewy-testapp

assets:
	go run cmd/dewy/main.go assets --registry ghr://linyows/dewy-testapp -l $(LOGLEVEL)

protobuf:
	cd registry && go tool buf generate

test:
	@go test -race -v ./... -coverprofile=coverage.out -covermode=atomic | \
		grep -v '^=== RUN' | \
		sed -E 's/--- PASS:/\x1B[38;5;34m✔︎\x1B[0m/g' | \
		sed -E 's/--- FAIL:/\x1B[31m✘\x1B[0m/g' | \
		sed -E 's/^PASS$$/\x1B[38;5;34m✔︎ Pass\x1B[0m/' | \
		sed -E 's/^FAIL$$/\x1B[31m✘ Fail\x1B[0m/'

integration:
	go test -integration $(TEST) $(TESTARGS)

lint:
	go tool golangci-lint run ./...

ci: deps test lint
	git diff go.mod

xbuild:
	go tool goreleaser --rm-dist --snapshot --skip-validate

dist:
	@test -z $(GITHUB_TOKEN) || go tool goreleaser --rm-dist --skip-validate

clean:
	git checkout go.*
	git clean -f

.PHONY: default dist test deps
