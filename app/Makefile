VERSION ?= 0.9.0
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
OUT_PATH ?= dist
BIN_PATH := bin
PKG_NAME := wireport
GO_MODULE := wireport
ARCHS := amd64 arm64
OSES := linux darwin windows

# Package metadata
PKG_DESCRIPTION := wireport - ingress proxy and VPN tunnel
PKG_MAINTAINER := MultionLabs <team@multionlabs.com>
PKG_VENDOR := MultionLabs
PKG_HOMEPAGE := https://github.com/MultionLabs/wireport

# Docker image for building packages
FPM_IMAGE := debian:bookworm-slim

build-pkgs: build-binaries build-deb build-rpm build-macos

build-binaries:
	@mkdir -p $(BIN_PATH)
	@$(foreach os,$(OSES), \
		$(foreach arch,$(ARCHS), \
			echo "Building for $(os)-$(arch)..."; \
			CGO_ENABLED=0 GOOS=$(os) GOARCH=$(arch) go build \
			-ldflags "-X '$(GO_MODULE)/version.Version=$(VERSION)' -X '$(GO_MODULE)/version.Arch=$(arch)' -X '$(GO_MODULE)/version.OS=$(os)' -X '$(GO_MODULE)/version.Commit=$(COMMIT)' -X '$(GO_MODULE)/version.Date=$(DATE)'" \
			-o $(BIN_PATH)/$(PKG_NAME)-$(os)-$(arch)$(if $(filter windows,$(os)),.exe,) ./cmd/server;))

build-deb:
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
			--depends iptables \
			--depends wireguard-tools \
			--depends iproute2 \
			-p $(OUT_PATH)/$(PKG_NAME)_$(VERSION)-1_debian_$(arch).deb \
			$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME);)

build-rpm:
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
			--depends iptables \
			--depends wireguard-tools \
			--depends iproute \
			-p $(OUT_PATH)/$(PKG_NAME)_$(VERSION)-1_rpm_$(arch).rpm \
			$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME);)

build-macos:
	@mkdir -p $(OUT_PATH)
	@echo "Building macOS packages..."
	@$(foreach arch,$(ARCHS), \
		echo "  Building for darwin-$(arch)..."; \
		fpm -s dir -t osxpkg -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
			--description "$(PKG_DESCRIPTION)" \
			--maintainer "$(PKG_MAINTAINER)" \
			--vendor "$(PKG_VENDOR)" \
			--url "$(PKG_HOMEPAGE)" \
			--force \
			--osxpkg-identifier-prefix com.multionlabs \
			-p $(OUT_PATH)/$(PKG_NAME)_$(VERSION)-1_macos_$(arch).pkg \
			$(BIN_PATH)/$(PKG_NAME)-darwin-$(arch)=/usr/local/bin/$(PKG_NAME);)

build-pkgs-docker: build-binaries build-deb-docker build-rpm-docker

build-deb-docker:
	@mkdir -p $(OUT_PATH)
	@echo "Building Debian packages using Docker..."
	@$(foreach arch,$(ARCHS), \
		echo "  Building for linux-$(arch)..."; \
		docker run --rm -v $(PWD):/workspace -w /workspace $(FPM_IMAGE) \
			bash -c "apt-get update && \
			apt-get install -y ruby ruby-dev build-essential && \
			gem install fpm && \
			rm -f $(OUT_PATH)/$(PKG_NAME)_$(VERSION)-1_debian_$(arch).deb && \
			fpm -s dir -t deb -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
				--description '$(PKG_DESCRIPTION)' \
				--maintainer '$(PKG_MAINTAINER)' \
				--vendor '$(PKG_VENDOR)' \
				--url '$(PKG_HOMEPAGE)' \
				--depends iptables \
				--depends wireguard-tools \
				--depends iproute2 \
				-p $(OUT_PATH)/$(PKG_NAME)_$(VERSION)-1_debian_$(arch).deb \
				$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME)";)

build-rpm-docker:
	@mkdir -p $(OUT_PATH)
	@echo "Building RPM packages using Docker..."
	@$(foreach arch,$(ARCHS), \
		echo "  Building for linux-$(arch)..."; \
		docker run --rm -v $(PWD):/workspace -w /workspace $(FPM_IMAGE) \
			bash -c "apt-get update && \
			apt-get install -y ruby ruby-dev build-essential rpm && \
			gem install fpm && \
			rm -f $(OUT_PATH)/$(PKG_NAME)_$(VERSION)-1_rpm_$(arch).rpm && \
			fpm -s dir -t rpm -n $(PKG_NAME) -v $(VERSION) -a $(arch) \
				--description '$(PKG_DESCRIPTION)' \
				--maintainer '$(PKG_MAINTAINER)' \
				--vendor '$(PKG_VENDOR)' \
				--url '$(PKG_HOMEPAGE)' \
				--depends iptables \
				--depends wireguard-tools \
				--depends iproute \
				-p $(OUT_PATH)/$(PKG_NAME)_$(VERSION)-1_rpm_$(arch).rpm \
				$(BIN_PATH)/$(PKG_NAME)-linux-$(arch)=/usr/bin/$(PKG_NAME)";)


clean:
	@echo "Cleaning build artifacts..."
	@rm -rf $(BIN_PATH) $(OUT_PATH)
