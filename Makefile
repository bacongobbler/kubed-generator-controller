NAME            = generator-controller
DIST_DIRS       = find * -maxdepth 0 -type d -exec

# go option
GO        ?= go
TAGS      := kqueue
TESTFLAGS :=
LDFLAGS   :=
GOFLAGS   :=
BINDIR    := $(CURDIR)/bin

# Required for globs to work correctly
SHELL=/bin/bash

.PHONY: all
all: build

.PHONY: build
build:
	GOBIN=$(BINDIR) $(GO) install $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' github.com/bacongobbler/draft-generator-controller/cmd/...
	mkdir -p $(BINDIR)/packs/default
	cp -R packs $(BINDIR)/packs/default/

.PHONY: build-cross
build-cross: LDFLAGS += -extldflags "-static"
build-cross:
	CGO_ENABLED=0 gox -output="_dist/{{.OS}}-{{.Arch}}/{{.Dir}}" -osarch='$(TARGETS)' $(GOFLAGS) -tags '$(TAGS)' -ldflags '$(LDFLAGS)' github.com/bacongobbler/draft-generator-controller/cmd/$(NAME)

.PHONY: dist
dist:
	( \
		cd _dist && \
		$(DIST_DIRS) cp ../LICENSE {} \; && \
		$(DIST_DIRS) cp ../README.md {} \; && \
		$(DIST_DIRS) mkdir -p {}/packs/default \; && \
		$(DIST_DIRS) cp -R ../packs {}/packs/default/ \; && \
		$(DIST_DIRS) tar -zcf $(NAME)-${VERSION}-{}.tar.gz {} \; && \
		$(DIST_DIRS) zip -r $(NAME)-${VERSION}-{}.zip {} \; \
	)

.PHONY: checksum
checksum:
	for f in _dist/*.{gz,zip} ; do \
		shasum -a 256 "$${f}"  | awk '{print $$1}' > "$${f}.sha256" ; \
	done

.PHONY: clean
clean:
	-rm -rf bin/
	-rm -rf _dist/

.PHONY: test
test: TESTFLAGS += -race -v
test: test-lint test-cover

test-cover:
	script/cover.sh

.PHONY: test-lint
test-lint:
	script/lint.sh

HAS_GOMETALINTER := $(shell command -v gometalinter;)
HAS_DEP := $(shell command -v dep;)
HAS_GOX := $(shell command -v gox;)
HAS_GIT := $(shell command -v git;)

.PHONY: bootstrap
bootstrap:
ifndef HAS_GOMETALINTER
	go get -u github.com/alecthomas/gometalinter
	gometalinter --install
endif
ifndef HAS_DEP
	go get -u github.com/golang/dep/cmd/dep
endif
ifndef HAS_GOX
	go get -u github.com/mitchellh/gox
endif
ifndef HAS_GIT
	$(error You must install git)
endif
	dep ensure

include versioning.mk

# Set VERSION to build release assets for a specific version
.PHONY: release-assets
release-assets: build-cross dist checksum
