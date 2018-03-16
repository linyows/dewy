TEST?=./...
NAME = "$(shell awk -F\" '/^const Name/ { print $$2; exit }' version.go)"
VERSION = "$(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)"

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
	go get -u github.com/mitchellh/gox
	go get -u github.com/tcnksm/ghr

test:
	go test $(TEST) $(TESTARGS) $(TEST_OPTIONS)
	go test -race $(TEST) $(TESTARGS)

integration:
	go test -integration $(TEST) $(TESTARGS) $(TEST_OPTIONS)

ci: depsdev test

dist:
	ghr v$(VERSION) pkg

.PHONY: default dist test test deps
