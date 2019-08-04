GOFMT_FILES?=$$(find . -name '*.go' | grep -v vendor)
NAME=clouddk-cloud-controller-manager
TARGETS=linux
VERSION=0.1.0

default: build

build:
	go build -o "bin/$(NAME)"

fmt:
	gofmt -w $(GOFMT_FILES)

test:
	go test -v

init:
	go get ./...

targets: $(TARGETS)

$(TARGETS):
	GOOS=$@ GOARCH=amd64 CGO_ENABLED=0 go build \
		-o "dist/$@/$(NAME)" \
		-a -ldflags '-extldflags "-static"'
	zip \
		-j "dist/$(NAME)_v$(VERSION)_$@_amd64.zip" \
		"dist/$@/$(NAME)"

.PHONY: build fmt test init targets $(TARGETS)
