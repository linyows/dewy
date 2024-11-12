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
	cd registry && buf generate

deps:
	go install github.com/goreleaser/goreleaser@latest
	go install github.com/bufbuild/buf/cmd/buf@latest

test:
	go test $(TEST) $(TESTARGS)
	go test -race $(TEST) $(TESTARGS) -coverprofile=coverage.out -covermode=atomic

integration:
	go test -integration $(TEST) $(TESTARGS)

lint:
	golangci-lint run ./...

ci: deps test lint
	git diff go.mod

xbuild:
	goreleaser --rm-dist --snapshot --skip-validate

dist:
	@test -z $(GITHUB_TOKEN) || goreleaser --rm-dist --skip-validate

clean:
	git checkout go.*
	git clean -f

.PHONY: default dist test deps
