# syntax = docker/dockerfile:1.15
########################################

FROM --platform=${BUILDPLATFORM} golang:1.25.0-alpine AS builder
RUN apk update && apk add --no-cache make
ENV GO111MODULE=on
WORKDIR /src

COPY go.mod go.sum /src/
RUN go mod download && go mod verify

COPY . .
ARG VERSION
ARG TAG
ARG SHA
RUN make build-all-archs

########################################

FROM --platform=${TARGETARCH} scratch AS release
LABEL org.opencontainers.image.source="https://github.com/vatesfr/xenorchestra-cloud-controller-manager" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.description="Xen Orchestra CCM for Kubernetes"

COPY --from=gcr.io/distroless/static-debian12:nonroot . .
ARG TARGETARCH
COPY --from=builder /src/bin/xenorchestra-cloud-controller-manager-${TARGETARCH} /bin/xenorchestra-cloud-controller-manager

ENTRYPOINT ["/bin/xenorchestra-cloud-controller-manager"]
