FROM golang:1.22-alpine AS builder

WORKDIR /workspace

ARG BUILD_DATE
ARG VCS_REF
ARG VERSION=dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o exporter \
    -ldflags="-s -w -X main.version=${VERSION}" \
    ./cmd/exporter

FROM alpine:3.19

ARG BUILD_DATE
ARG VCS_REF
ARG VERSION=dev

LABEL org.label-schema.build-date=$BUILD_DATE \
      org.label-schema.name="argo-workflows-metrics" \
      org.label-schema.description="Argo Workflows Prometheus Metrics Exporter" \
      org.label-schema.url="https://github.com/conti/argo-workflows-metrics" \
      org.label-schema.vcs-ref=$VCS_REF \
      org.label-schema.vcs-url="https://github.com/conti/argo-workflows-metrics" \
      org.label-schema.vendor="conti" \
      org.label-schema.version=$VERSION \
      org.label-schema.schema-version="1.0"

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /workspace/exporter .

USER 65534:65534

ENTRYPOINT ["/app/exporter"]
