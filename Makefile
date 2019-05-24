.PHONY: test
test:
	@go test -v ./...

.PHONY: install
install:
	@go build -o ${GOPATH}/bin/compass cmd/main.go