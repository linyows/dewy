TEST ?= ./...
REVISION = $$(git describe --always)
DATE = $$(LC_ALL=c date -u +%a,\ %d\ %b\ %Y\ %H:%M:%S\ GMT)
LOGLEVEL ?= info
ifeq ("$(shell uname)","Darwin")
NCPU ?= $(shell sysctl hw.ncpu | cut -f2 -d' ')
else
NCPU ?= $(shell cat /proc/cpuinfo | grep processor | wc -l)
endif
TEST_OPTIONS=-timeout 30s -parallel $(NCPU)

default: build

build:
	go build ./cmd/dewy

server: build
	./dewy server -r linyows/dewy-testapp -a dewy-testapp_darwin_amd64.tar.gz \
		-p 8000 -l $(LOGLEVEL) -- $(HOME)/.go/src/github.com/linyows/dewy/current/dewy-testapp

assets: build
	./dewy assets -r linyows/dewy-testapp -a dewy-testapp_darwin_amd64.tar.gz -l $(LOGLEVEL)

deps:
	go get golang.org/x/lint/golint
	go get github.com/goreleaser/goreleaser

test:
	go test $(TEST) $(TESTARGS) $(TEST_OPTIONS)
	go test -race $(TEST) $(TESTARGS) -coverprofile=coverage.txt -covermode=atomic

integration:
	go test -integration $(TEST) $(TESTARGS) $(TEST_OPTIONS)

lint:
	golint -set_exit_status $(TEST)

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
