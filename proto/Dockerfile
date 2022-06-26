FROM golang:1.18-bullseye as build

ARG PROTOC_GEN_GO_VERSION="v1.28.0"
ARG PROTOC_GEN_GO_GRPC_VERSION="v1.2.0"
ARG PROTOC_VERSION="3.20.1"
ARG PROTOC_GEN_BUF_VERSION="v1.4.0"

ENV GOBIN=/usr/local/bin
ENV GO111MODULE=on
ENV GOPATH=/tmp/gopath

RUN set -eux \
  && DEBIAN_FRONTEND=noninteractive \
  && apt-get update \
  && apt-get upgrade -y \
  && apt-get install --yes --no-install-recommends \
  curl \
  jq \
  unzip \
  && rm -rf /var/lib/apt/lists/*

# Install protoc-gen-go
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION}
# Install protoc-gen-go-grpc
RUN go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}
# Install protoc
RUN curl -sSL -o /tmp/protoc.zip https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-x86_64.zip \
    && unzip /tmp/protoc.zip -d /usr/local/ -x "*.txt"
# Install buf
RUN curl -sSL -o /usr/local/bin/buf https://github.com/bufbuild/buf/releases/download/${PROTOC_GEN_BUF_VERSION}/buf-Linux-x86_64 \
  && chmod +x /usr/local/bin/buf
# Install protoc-gen-buf-lint
RUN curl -sSL -o /usr/local/bin/protoc-gen-buf-lint https://github.com/bufbuild/buf/releases/download/${PROTOC_GEN_BUF_VERSION}/protoc-gen-buf-lint-Linux-x86_64 \
  && chmod +x /usr/local/bin/protoc-gen-buf-lint

WORKDIR /workdir
CMD [ "bash" ]
