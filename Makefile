.PHONY: test
test:
	@go test -v ./...

.PHONY: install
install:
	@go build -o ${GOPATH}/bin/compass \
		-ldflags "-X github.com/monax/compass/cmd.commit=$(shell git rev-parse --short HEAD)" .

.PHONY: release
release: install
	$(eval COMPASS_VERSION := $(shell compass version --short))
	git tag ${COMPASS_VERSION}
	git push origin ${COMPASS_VERSION}