GO ?= go
V ?=
PKGS = $(shell go list ./...)
PKGFILES = $(shell find . \( -path ./vendor -o -path ./Godeps \) -prune \
		-o -type f -name '*.go' -print)
PKGFILES_notest = $(shell echo $(PKGFILES) | tr ' ' '\n' | grep -v _test.go)

GOLANGCI_LINT ?= golangci-lint

GO_TEST_TOOLS = \
	github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

VERSION = $(shell git describe --tags --dirty --exact-match 2>/dev/null || git rev-parse --short HEAD)
REVISION = $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
BRANCH = $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown)
BUILD_DATE = $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
BUILD_USER = $(shell id -un 2>/dev/null || echo unknown)@$(shell hostname 2>/dev/null || echo unknown)

VERSION_PKG = github.com/mendersoftware/mender-cli/cmd
GO_LDFLAGS = \
	-ldflags " \
		-X $(VERSION_PKG).Version=$(VERSION) \
		-X $(VERSION_PKG).Revision=$(REVISION) \
		-X $(VERSION_PKG).Branch=$(BRANCH) \
		-X $(VERSION_PKG).BuildDate=$(BUILD_DATE) \
		-X $(VERSION_PKG).BuildUser=$(BUILD_USER) \
		-X $(VERSION_PKG).Tags=$(GO_TAGS_CSV)"
BUILDFLAGS ?=

ifeq ($(V),1)
BUILDV = -v
endif

TAGS =
ifeq ($(LOCAL),1)
TAGS += local
endif

# Full list of build tags as space- and comma-separated forms. The CSV form is
# embedded into the binary (a space would break the -X linker flag).
empty :=
space := $(empty) $(empty)
comma := ,
GO_TAGS = $(strip nopkcs11 $(TAGS))
GO_TAGS_CSV = $(subst $(space),$(comma),$(GO_TAGS))

BUILDTAGS = -tags '$(GO_TAGS)'

build:
	CGO_ENABLED=0 $(GO) build $(BUILDFLAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDTAGS)

build-autocomplete-scripts: build
	@./mender-cli --generate-autocomplete

build-multiplatform:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(BUILDTAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDFLAGS) \
	     -o mender-cli.linux.amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(BUILDTAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDFLAGS) \
	     -o mender-cli.linux.arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build $(BUILDTAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDFLAGS) \
	     -o mender-cli.darwin.amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build $(BUILDTAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDFLAGS) \
	     -o mender-cli.darwin.arm64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build $(BUILDTAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDFLAGS) \
	     -o mender-cli.windows.amd64.exe
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 $(GO) build $(BUILDTAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDFLAGS) \
	     -o mender-cli.windows.arm64.exe

build-coverage:
	CGO_ENABLED=0 $(GO) build -cover -o mender-cli-test \
		-coverpkg $(shell echo $(PKGS) | tr  ' ' ',')

install:
	CGO_ENABLED=0 $(GO) install $(BUILDTAGS) $(GO_LDFLAGS) $(BUILDV) $(BUILDFLAGS)

install-autocomplete-scripts: build-autocomplete-scripts
	@echo "Installing Bash auto-complete script into ${DESTDIR}${PREFIX}/etc/bash_completion.d/"
	@install -d ${DESTDIR}$(PREFIX)/etc/bash_completion.d/
	@install -m 644 ./autocomplete/autocomplete.sh $(DESTDIR)$(PREFIX)/etc/bash_completion.d/
	@if which zsh >/dev/null 2>&1 ; then \
	echo "Installing zsh auto-complete script into ${DESTDIR}${PREFIX}/usr/local/share/zsh/site-functions/" && \
	install -d $(DESTDIR)$(PREFIX)/usr/local/share/zsh/site-functions/ && \
	install -m 644 ./autocomplete/autocomplete.zsh $(DESTDIR)$(PREFIX)/usr/local/share/zsh/site-functions/_mender-cli \
	; fi

clean:
	$(GO) clean
	rm -f coverage.txt coverage-tmp.txt

get-go-tools:
	set -e ; for t in $(GO_TEST_TOOLS); do \
		echo "-- installing $$t"; \
		$(GO) install $$t; \
	done

get-build-deps:
	apt-get update -qq
	apt-get install -yyq $(shell cat deb-requirements.txt)

get-deps: get-go-tools get-build-deps

test-unit:
	CGO_ENABLED=0 $(GO) test $(BUILDTAGS) $(BUILDV) $(PKGS)

build-acceptance:
	docker compose -f tests/acceptance/docker-compose.yml build acceptance $(BUILDFLAGS)

run-acceptance:
	docker compose -f tests/acceptance/docker-compose.yml run acceptance

gofmt-check:
	@echo "-- checking formatting with gofmt"
	@if [ -n "$$(gofmt -l $(PKGFILES))" ]; then \
		echo "the following files are not gofmt-compliant; run 'gofmt -w':"; \
		gofmt -l $(PKGFILES); \
		exit 1; \
	fi

vet:
	@echo "-- checking with go vet"
	CGO_ENABLED=0 $(GO) vet $(BUILDTAGS) $(PKGS)

lint:
	@echo "-- checking with golangci-lint"
	@command -v $(GOLANGCI_LINT) >/dev/null 2>&1 || { \
		echo "$(GOLANGCI_LINT) not found; install it with 'make get-go-tools'" \
			"or see https://golangci-lint.run/welcome/install/"; \
		exit 1; \
	}
	$(GOLANGCI_LINT) run

test-static: gofmt-check vet lint

cover: coverage
	$(GO) tool cover -func=coverage.txt

htmlcover: coverage
	$(GO) tool cover -html=coverage.txt

coverage:
	rm -f coverage.txt
	echo 'mode: set' > coverage.txt
	set -e ; for p in $(PKGS); do \
		rm -f coverage-tmp.txt;  \
		CGO_ENABLED=0 $(GO) test $(BUILDTAGS) -coverprofile=coverage-tmp.txt $$p ; \
		if [ -f coverage-tmp.txt ]; then \
			cat coverage-tmp.txt | grep -v 'mode:' >> coverage.txt || /bin/true; \
		fi; \
	done
	rm -f coverage-tmp.txt

.PHONY: build clean get-go-tools get-apt-deps get-deps test check \
	cover htmlcover coverage test-unit test-static gofmt-check vet lint
