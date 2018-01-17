TEST?=./...
NAME = "$(shell awk -F\" '/^const Name/ { print $$2; exit }' version.go)"
VERSION = "$(shell awk -F\" '/^const Version/ { print $$2; exit }' version.go)"

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
	go test -v $(TEST) $(TESTARGS) -timeout=30s -parallel=4
	go test -race $(TEST) $(TESTARGS)

ci:
	go test -v

dist:
	ghr v$(VERSION) pkg

.PHONY: default dist test test deps
