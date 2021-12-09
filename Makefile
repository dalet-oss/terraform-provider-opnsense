LDFLAGS += -X main.version=$$(git describe --always --abbrev=40 --dirty)

default: build

terraform-provider-opnsense:
	go build -ldflags "${LDFLAGS}"

build: fmt-check lint-check vet-check terraform-provider-opnsense

install:
	go install -ldflags "${LDFLAGS}"

vet-check:
	go vet ./opnsense .

lint-check:
	go run golang.org/x/lint/golint -set_exit_status ./opnsense .

fmt-check:
	go fmt ./opnsense .

clean:
	rm -f terraform-provider-opnsense testsuite

.PHONY: build install test testacc vet-check fmt-check lint-check terraform-provider-opnsense
