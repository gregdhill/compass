.PHONY: test
test:
	@go test -v ./...

.PHONY: install
install:
	@go build -o ${GOPATH}/bin/compass \
		-ldflags "-X github.com/monax/compass/cmd/project.commit=$(shell git rev-parse --short HEAD)" \
		./cmd/main.go