VERSION ?= $(shell \
	tag=$$(git describe --abbrev=0 --tags 2>/dev/null || echo v0.0.0); \
	echo $${tag}-dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
OUT_PATH ?= dist
BIN_PATH := bin
PKG_NAME := wireport
GO_MODULE := wireport
ARCHS := amd64 arm64
OSES := linux darwin windows
PACKAGE ?= unknown
DOCKER_IMAGE ?= ghcr.io/multionlabs/wireport

# Package metadata
PKG_DESCRIPTION := wireport - ingress proxy and VPN tunnel
PKG_MAINTAINER := MultionLabs <support@multionlabs.com>
PKG_VENDOR := MultionLabs
PKG_HOMEPAGE := https://github.com/MultionLabs/wireport

# Docker image for building packages
FPM_IMAGE := debian:bookworm-slim

# Function: build_binary_cmd <os> <arch>
# Compiles the server binary for the given OS/ARCH pair with proper ldflags.
build_binary_cmd = echo "Building for $(1)-$(2)..."; \
	CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) go build \
	-ldflags "-X '$(GO_MODULE)/version.Version=$(VERSION)' -X '$(GO_MODULE)/version.Arch=$(2)' -X '$(GO_MODULE)/version.OS=$(1)' -X '$(GO_MODULE)/version.Commit=$(COMMIT)' -X '$(GO_MODULE)/version.Date=$(DATE)' -X '$(GO_MODULE)/version.Package=$(PACKAGE)'" \
	-o $(BIN_PATH)/$(PKG_NAME)-$(1)-$(2)$(if $(filter windows,$(1)),.exe,) ./cmd/server;

build-binaries:
	@mkdir -p $(BIN_PATH)
	@$(foreach os,$(OSES), \
		$(foreach arch,$(ARCHS), \
			$(call build_binary_cmd,$(os),$(arch))))

build-deb: PACKAGE=deb
build-deb: build-binaries
	@mkdir -p $(OUT_PATH)
	@echo "Building Debian packages for Linux..."
	@$(foreach arch,$(ARCHS), \
		echo "  Building for linux-$(arch)..."; \
		fpm -s dir -t deb -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
			--description "$(PKG_DESCRIPTION)" \
			--maintainer "$(PKG_MAINTAINER)" \
			--vendor "$(PKG_VENDOR)" \
			--url "$(PKG_HOMEPAGE)" \
			--force \
			-p $(OUT_PATH)/$(PKG_NAME)-debian-$(arch).deb \
			$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME);)

build-rpm: PACKAGE=rpm
build-rpm: build-binaries
	@mkdir -p $(OUT_PATH)
	@echo "Building RPM packages for Linux..."
	@$(foreach arch,$(ARCHS), \
		echo "  Building for linux-$(arch)..."; \
		fpm -s dir -t rpm -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
			--description "$(PKG_DESCRIPTION)" \
			--maintainer "$(PKG_MAINTAINER)" \
			--vendor "$(PKG_VENDOR)" \
			--url "$(PKG_HOMEPAGE)" \
			--force \
			-p $(OUT_PATH)/$(PKG_NAME)-rpm-$(arch).rpm \
			$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME);)

build-macos-pkg: PACKAGE=pkg
build-macos-pkg:
	@mkdir -p $(BIN_PATH)
	@$(foreach arch,$(ARCHS), \
		$(call build_binary_cmd,darwin,$(arch)))
	@mkdir -p $(OUT_PATH)
	@echo "Packaging into PKG..."
	@$(foreach arch,$(ARCHS), \
		echo "  Packaging darwin-$(arch)..."; \
		fpm -s dir -t osxpkg -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
			--description "$(PKG_DESCRIPTION)" \
			--maintainer "$(PKG_MAINTAINER)" \
			--vendor "$(PKG_VENDOR)" \
			--url "$(PKG_HOMEPAGE)" \
			--force \
			--osxpkg-identifier-prefix com.multionlabs \
			-p $(OUT_PATH)/$(PKG_NAME)-macos-$(arch).pkg \
			$(BIN_PATH)/$(PKG_NAME)-darwin-$(arch)=/usr/local/bin/$(PKG_NAME);)

build-macos-zip: PACKAGE=zip
build-macos-zip:
	@mkdir -p $(BIN_PATH)
	@$(foreach arch,$(ARCHS), \
		$(call build_binary_cmd,darwin,$(arch)))
	@mkdir -p $(OUT_PATH)
	@echo "Packaging into ZIP..."
	@$(foreach arch,$(ARCHS), \
		echo "  Packaging darwin-$(arch)..."; \
		zip -j $(OUT_PATH)/$(PKG_NAME)-macos-$(arch).zip $(BIN_PATH)/$(PKG_NAME)-darwin-$(arch);)

build-windows-zip: PACKAGE=zip
build-windows-zip:
	@mkdir -p $(BIN_PATH)
	@$(foreach arch,$(ARCHS), \
		$(call build_binary_cmd,windows,$(arch)))
	@mkdir -p $(OUT_PATH)
	@echo "Packaging into ZIP..."
	@$(foreach arch,$(ARCHS), \
		echo "  Packaging windows-$(arch)..."; \
		zip -j $(OUT_PATH)/$(PKG_NAME)-windows-$(arch).zip $(BIN_PATH)/$(PKG_NAME)-windows-$(arch).exe;)

build-pkgs-docker: build-deb-docker build-rpm-docker

build-deb-docker: PACKAGE=deb
build-deb-docker:
	@mkdir -p $(BIN_PATH)
	@$(foreach arch,$(ARCHS), \
		$(call build_binary_cmd,linux,$(arch)))
	@mkdir -p $(OUT_PATH)
	@echo "Packaging into DEB using Docker..."
	@$(foreach arch,$(ARCHS), \
		echo "  Packaging linux-$(arch)..."; \
		docker run --rm -v $(PWD):/workspace -w /workspace $(FPM_IMAGE) \
			bash -c "apt-get update && \
			apt-get install -y ruby ruby-dev build-essential && \
			gem install fpm && \
			rm -f $(OUT_PATH)/$(PKG_NAME)-linux-$(arch).deb && \
			fpm -s dir -t deb -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
				--description '$(PKG_DESCRIPTION)' \
				--maintainer '$(PKG_MAINTAINER)' \
				--vendor '$(PKG_VENDOR)' \
				--url '$(PKG_HOMEPAGE)' \
				-p $(OUT_PATH)/$(PKG_NAME)-linux-$(arch).deb \
				$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME)";)

build-rpm-docker: PACKAGE=rpm
build-rpm-docker:
	@mkdir -p $(BIN_PATH)
	@$(foreach arch,$(ARCHS), \
		$(call build_binary_cmd,linux,$(arch)))
	@mkdir -p $(OUT_PATH)
	@echo "Packaging into RPM using Docker..."
	@$(foreach arch,$(ARCHS), \
		echo "  Packaging linux-$(arch)..."; \
		docker run --rm -v $(PWD):/workspace -w /workspace $(FPM_IMAGE) \
			bash -c "apt-get update && \
			apt-get install -y ruby ruby-dev build-essential rpm && \
			gem install fpm && \
			rm -f $(OUT_PATH)/$(PKG_NAME)-linux-$(arch).rpm && \
			fpm -s dir -t rpm -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
				--description '$(PKG_DESCRIPTION)' \
				--maintainer '$(PKG_MAINTAINER)' \
				--vendor '$(PKG_VENDOR)' \
				--url '$(PKG_HOMEPAGE)' \
				-p $(OUT_PATH)/$(PKG_NAME)-linux-$(arch).rpm \
				$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME)";)

build-tar-docker: PACKAGE=tar
build-tar-docker:
	@mkdir -p $(BIN_PATH)
	@$(foreach arch,$(ARCHS), \
		$(call build_binary_cmd,linux,$(arch)))
	@mkdir -p $(OUT_PATH)
	@echo "Packaging into TAR using Docker..."
	@$(foreach arch,$(ARCHS), \
		echo "  Packaging linux-$(arch)..."; \
		docker run --rm -v $(PWD):/workspace -w /workspace $(FPM_IMAGE) \
			bash -c "apt-get update && \
			apt-get install -y ruby ruby-dev build-essential && \
			gem install fpm && \
			rm -f $(OUT_PATH)/$(PKG_NAME)-linux-$(arch).tar && \
			fpm -s dir -t tar -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
				--description '$(PKG_DESCRIPTION)' \
				--maintainer '$(PKG_MAINTAINER)' \
				--vendor '$(PKG_VENDOR)' \
				--url '$(PKG_HOMEPAGE)' \
				-p $(OUT_PATH)/$(PKG_NAME)-linux-$(arch).tar \
				$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME)";)

build-pkgs: build-macos-pkg build-macos-zip build-windows-zip build-deb-docker build-rpm-docker build-tar-docker

docker-build-push:
	@echo "Building and pushing Docker image..."
	docker buildx build --platform=linux/amd64,linux/arm64 -t $(DOCKER_IMAGE):$(VERSION) . && docker push $(DOCKER_IMAGE):$(VERSION)

clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_PATH) $(OUT_PATH)
