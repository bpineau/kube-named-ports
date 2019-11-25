
all: build

tools:
	which golangci-lint || ( curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh| sh -s -- -b $$(go env GOPATH)/bin v1.21.0 )
	which goveralls || go get github.com/mattn/goveralls

lint:
	golangci-lint --concurrency=1 --deadline=300s --disable-all \
		--enable=golint \
		--enable=vet \
		--enable=vetshadow \
		--enable=varcheck \
		--enable=errcheck \
		--enable=structcheck \
		--enable=deadcode \
		--enable=ineffassign \
		--enable=dupl \
		--enable=varcheck \
		--enable=interfacer \
		--enable=goconst \
		--enable=megacheck \
		--enable=unparam \
		--enable=misspell \
		--enable=gas \
		--enable=goimports \
		--enable=gocyclo \
		run ./...

fmt:
	go fmt ./...

build:
	env CGO_ENABLED=0 go build -i

install:
	env CGO_ENABLED=0 go install

clean:
	rm -rf dist/
	go clean -i

coverall:
	goveralls -service=travis-ci -package github.com/bpineau/kube-named-ports/pkg/...

test:
	go test -i github.com/bpineau/kube-named-ports/...
	go test -race -cover github.com/bpineau/kube-named-ports/...

.PHONY: tools lint fmt install clean coverall test all
