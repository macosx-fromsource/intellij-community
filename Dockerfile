# Global ARG (must be declared before the first FROM to be usable in later FROMs).
ARG BUILD_IMAGE=docker.io/golang:1.26

# Build the manager binary. This is the public image: the bridge
# (backend-for-frontend) feature is gated behind the `bridge` build tag and is
# NOT compiled in here. See Dockerfile.bridge for the bridge-enabled image.
FROM --platform=${BUILDPLATFORM} ${BUILD_IMAGE} AS builder
WORKDIR /workspace

# Download dependencies. This runs once for a multi-platform build: it sits
# in the part of the stage that precedes TARGETARCH, so both targets share it.
ENV GOPATH=/go
COPY go.mod go.sum ./
RUN go mod download

# Copy the go source. cmd/manager/bridge_stub.go provides the no-op setupBridge
# for the default (untagged) build; cmd/manager/bridge.go only compiles under the
# `bridge` build tag, which is not set here, so the bridge is absent from this image.
COPY cmd/manager/ cmd/manager/
COPY api/ api/
COPY helm/ helm/
COPY pkg/ pkg/
COPY controllers/ controllers/
COPY internal/ internal/

# Build
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o manager ./cmd/manager

# ubi9-micro carries no CA certificates; borrow them from ubi9-minimal.
FROM registry.access.redhat.com/ubi9-minimal:9.8@sha256:8ebe2ad8fdf3cab3e5a53c1edc69194c98209cfadab24b884f4ad9ebcf7bbbfc AS certs

# Use ubi micro as base image to package the manager binary
# Refer to https://www.redhat.com/en/blog/introduction-ubi-micro for more details
FROM registry.access.redhat.com/ubi9-micro:9.8@sha256:7a0454cbd9bd847e8f6a63b6f0254a6efbeb6e0ed71a5d824a4f6cccbe626650

LABEL name=gitlab-operator \
      vendor='GitLab, Inc.' \
      description='Operator to deploy GitLab instances' \
      summary='GitLab is a DevOps lifecycle tool that provides Git repositories' \
      maintainer='GitLab Self-Managed'

# Allow the chart directory to be overwritten with --build-arg
ARG CHART_DIR="/charts"

ENV USER_UID=1001 \
    HELM_CHARTS=${CHART_DIR}

# ADD GITLAB LICENSE
COPY LICENSE /licenses/GITLAB

COPY --from=certs /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem /etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem
COPY --from=certs /etc/pki/tls/certs/ca-bundle.crt /etc/pki/tls/certs/ca-bundle.crt
# Add pre-packaged charts for the operator to deploy
COPY charts ${CHART_DIR}

WORKDIR /
COPY --from=builder /workspace/manager .
USER ${USER_UID}

ENTRYPOINT ["/manager"]
