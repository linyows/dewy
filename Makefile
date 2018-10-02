TEST ?= ./...
FILES ?= $(shell go list ./... | grep -v vendor)
NAME = "$(shell awk -F\" '/^const Name/ { print $$2; exit }' version.go)"
VERSION = "$(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)"
GOVERSION = $(shell go version | awk '{ if (sub(/go version go/, "v")) print }' | awk '{print $$1 "-" $$2}')
GOENV ?= GO111MODULE=on

ifeq ("$(shell uname)","Darwin")
NCPU ?= $(shell sysctl hw.ncpu | cut -f2 -d' ')
else
NCPU ?= $(shell cat /proc/cpuinfo | grep processor | wc -l)
endif
TEST_OPTIONS=-timeout 30s -parallel $(NCPU)

default: test

build:
	$(GOENV) go build -o dewy cmd/dewy/main.go

run: build
	$(GOENV) ./dewy server -r linyows/dewy-testapp -a dewy-testapp_darwin_amd64.tar.gz \
		-p 8000 -l info -- $(HOME)/.go/src/github.com/linyows/dewy/current/dewy-testapp

deps:
	$(GOENV) go get github.com/golang/lint/golint
	$(GOENV) go get github.com/pierrre/gotestcover
	$(GOENV) go get github.com/goreleaser/goreleaser

test:
	$(GOENV) go test -v $(TEST) $(TESTARGS) $(TEST_OPTIONS)
	$(GOENV) go test -race $(TEST) $(TESTARGS)

integration:
	$(GOENV) go test -integration $(TEST) $(TESTARGS) $(TEST_OPTIONS)

lint:
	golint -set_exit_status $(FILES)

ci: deps test

dist:
	git tag | grep v$(VERSION) || git tag v$(VERSION)
	git push origin v$(VERSION)
	GOVERSION=$(GOVERSION) goreleaser --rm-dist

.PHONY: default dist test deps
