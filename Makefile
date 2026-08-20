APP := rdc-exporter

# The image assembles pre-built artifacts from the selected TheRock release.
ROCM_VERSION       ?= 7.13.0
ROCM_ARCHS         ?= gfx950
THEROCK_COMMIT     ?=
THEROCK_ROCM_ROOT  ?=
RUNTIME_ROOT       ?= runtime-root
DISTROLESS_IMAGE   ?= gcr.io/distroless/python3-debian13@sha256:b340f07acd3692d739cbc28450b8876b4770ee01967aa9e4193c3bcec7bd235e

BUILD_DATE         ?= $(shell date +'%Y%m%d')
BUILD_TIMESTAMP    ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')
RDC_EXPORTER_COMMIT ?= $(shell git rev-parse HEAD)
IMAGE_REPO         ?= ghcr.io/maple52046/rdc-exporter
IMAGE_TAG          ?= $(IMAGE_REPO):v1-rocm$(ROCM_VERSION)-$(BUILD_DATE)

DOCKER_BUILD_ARGS := --build-arg BUILD_DATE="$(BUILD_TIMESTAMP)" \
	--build-arg ROCM_VERSION="$(ROCM_VERSION)" \
	--build-arg ROCM_ARCHS="$(ROCM_ARCHS)" \
	--build-arg THEROCK_COMMIT="$(THEROCK_COMMIT)" \
	--build-arg RDC_EXPORTER_COMMIT="$(RDC_EXPORTER_COMMIT)" \
	--build-arg DISTROLESS_IMAGE="$(DISTROLESS_IMAGE)"

.PHONY: ALL build clean image image-verify prepare-runtime print-image
ALL: build

# Build the CGO application against RDC installed at /opt/rocm.
build:
	go fmt ./...
	@echo "Building $(APP)..."
	mkdir -p ./bin
	rm -f ./bin/$(APP)
	CGO_ENABLED=1 go build -o ./bin/$(APP) ./cmd/rdc-exporter/main.go

# Assemble the final-image ROCm closure from a complete TheRock distribution.
prepare-runtime:
	@test -n "$(THEROCK_ROCM_ROOT)" || { echo "THEROCK_ROCM_ROOT is required" >&2; exit 2; }
	@test ! -e "$(RUNTIME_ROOT)" || { echo "$(RUNTIME_ROOT) already exists; move or remove it before rebuilding" >&2; exit 2; }
	./scripts/prepare-runtime-root.sh "$(THEROCK_ROCM_ROOT)" "$(RUNTIME_ROOT)"

# BuildKit receives only Dockerfile, bin/rdc-exporter, and runtime-root via .dockerignore.
image:
	@test -n "$(THEROCK_COMMIT)" || { echo "THEROCK_COMMIT is required" >&2; exit 2; }
	@test -x "./bin/$(APP)" || { echo "run make build against the matching TheRock distribution first" >&2; exit 2; }
	@test -f "$(RUNTIME_ROOT)/opt/rocm/.info/version" || { echo "run make prepare-runtime first" >&2; exit 2; }
	docker build $(DOCKER_BUILD_ARGS) -t "$(IMAGE_TAG)" .

# Verify required paths without assuming a shell exists in the distroless image.
image-verify: image
	./scripts/verify-distroless-image.sh "$(IMAGE_TAG)"

print-image:
	@echo "$(IMAGE_TAG)"

# runtime-root is intentionally retained because regenerating it requires the
# matching TheRock distribution. Remove it explicitly when changing releases.
clean:
	rm -rf ./bin
