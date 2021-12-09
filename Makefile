LDFLAGS += -X main.version=$$(git describe --always --abbrev=40 --dirty)

default: build

binary:
	go build -gcflags="opnsense/...=-e" -ldflags "${LDFLAGS}" -o . ./...

build: fmt-check lint-check vet-check binary

install:
	go install -ldflags "${LDFLAGS}"

vet-check:
	go vet ./opnsense ./cmd/terraform-provider-opnsense ./cmd/testsuite

lint-check:
	go run golang.org/x/lint/golint -set_exit_status ./opnsense ./cmd/terraform-provider-opnsense ./cmd/testsuite

fmt-check:
	go fmt ./opnsense ./cmd/terraform-provider-opnsense ./cmd/testsuite

clean:
	rm -f terraform-provider-opnsense testsuite

.PHONY: build install test testacc vet-check fmt-check lint-check terraform-provider-opnsense testsuite
