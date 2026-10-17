# Build the manager binary
FROM --platform=$BUILDPLATFORM golang:1.27@sha256:820153096fda4de6ac1b7194d12e416b7dd8f4e9a33d17e0345b111b945162d8 AS builder

WORKDIR /workspace
RUN go env -w GOMODCACHE=/root/.cache/go-build

# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

# Cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN --mount=type=cache,target=/root/.cache/go-build go mod download

# Copy the go source
COPY cmd/ cmd/
COPY compver/ compver/
COPY helmvalues/ helmvalues/

ARG TARGETOS
ARG TARGETARCH

RUN mkdir bin

FROM builder AS ocm-kit-builder
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH GO111MODULE=on go build -ldflags="-s -w" -a -o bin/ocm-kit ./cmd/ocm-kit

# Use distroless as minimal base image to package the binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot@sha256:e2e927ec666bae08560abb3c55d0659eceabb657f56b6782ab500a9fc7f555e3 AS ocm-kit
WORKDIR /
COPY --from=ocm-kit-builder /workspace/bin/ocm-kit .
USER 65532:65532
ENTRYPOINT ["/ocm-kit"]
