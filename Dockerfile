# Build the manager binary
ARG BUILD_IMAGE=docker.io/golang:1.26
FROM --platform=${BUILDPLATFORM} ${BUILD_IMAGE} AS builder
WORKDIR /workspace

# Copy in any existing Go cache, and download
# any missing dependencies.
ENV GOPATH=/go
COPY .go/pkg/mod/ /go/pkg/mod/
COPY go.mod go.sum ./
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY helm/ helm/
COPY pkg/ pkg/
COPY controllers/ controllers/

# Build
ARG TARGETOS
ARG TARGETARCH

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} GO111MODULE=on go build -a -o manager main.go

# Use ubi micro as base image to package the manager binary
# Refer to https://www.redhat.com/en/blog/introduction-ubi-micro for more details
FROM registry.access.redhat.com/ubi9-micro:9.8@sha256:fdf68a4f5f88cca14ae906bbec6e0fbbffe92b5b91e73e0862c961234d63b986

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

# Add pre-packaged charts for the operator to deploy
COPY charts ${CHART_DIR}

WORKDIR /
COPY --from=builder /workspace/manager .
USER ${USER_UID}

ENTRYPOINT ["/manager"]
