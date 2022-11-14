# Build the manager binary
FROM golang:1.19 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -tags hazelcastinternal -a -o manager main.go

FROM registry.access.redhat.com/ubi8/ubi-minimal:8.6

RUN microdnf upgrade -y && \
    microdnf clean all

ARG version="5.4"
ARG pardotID="dockerhub"
ENV OPERATOR_VERSION=${version}
ENV PARDOT_ID=${pardotID}

LABEL name="Hazelcast Platform Operator" \
      maintainer="info@hazelcast.com" \
      vendor="Hazelcast, Inc." \
      version=$version \
      release="1" \
      summary="Hazelcast Platform Operator Image" \
      description="Hazelcast Platform Operator Image"

WORKDIR /
COPY --from=builder /workspace/manager .
COPY licenses /licenses
USER 65532:65532

ENTRYPOINT ["/manager"]
