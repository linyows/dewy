TEST ?= ./...
FILES ?= $(shell go list ./... | grep -v vendor)
NAME = "$(shell awk -F\" '/^const Name/ { print $$2; exit }' version.go)"
VERSION = "$(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)"
GOVERSION = $(shell go version | awk '{ if (sub(/go version go/, "v")) print }' | awk '{print $$1 "-" $$2}')

ifeq ("$(shell uname)","Darwin")
NCPU ?= $(shell sysctl hw.ncpu | cut -f2 -d' ')
else
NCPU ?= $(shell cat /proc/cpuinfo | grep processor | wc -l)
endif
TEST_OPTIONS=-timeout 30s -parallel $(NCPU)

default: test

deps:
	go get -u github.com/golang/dep/...
	dep ensure

depsdev: deps
	go get github.com/golang/lint/golint
	go get github.com/pierrre/gotestcover

test:
	go test $(TEST) $(TESTARGS) $(TEST_OPTIONS)
	go test -race $(TEST) $(TESTARGS)

integration:
	go test -integration $(TEST) $(TESTARGS) $(TEST_OPTIONS)

lint:
	golint -set_exit_status $(FILES)

ci: depsdev test

dist:
	go get github.com/goreleaser/goreleaser
	git tag | grep v$(VERSION) || git tag v$(VERSION)
	git push origin v$(VERSION)
	GOVERSION=$(GOVERSION) goreleaser --rm-dist

.PHONY: default dist test test deps
