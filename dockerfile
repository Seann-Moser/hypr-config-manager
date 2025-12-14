# STAGE 1
# Build the executable(s).
FROM --platform=$BUILDPLATFORM  golang:alpine AS stage1

WORKDIR /var/build/go
ARG TARGETARCH
ENV GOARCH=$TARGETARCH

ADD go.mod .
ADD go.sum .
RUN go env -w GOMODCACHE=/gomod-cache

RUN --mount=type=cache,target=/gomod-cache \
  go mod vendor


ARG VERSION=dev
ADD ./ ./
ENV GOCACHE=/root/.cache/go-build
RUN --mount=type=cache,target="/root/.cache/go-build" go build -o /var/build/bin/api ./

#STAGE 2
#Prepare the base image.
FROM alpine:latest AS stage2

RUN apk --no-cache add --no-check-certificate ca-certificates \
    && update-ca-certificates

FROM stage2 AS stage3

#COPY  templates/ templates/
COPY --from=stage1 /var/build/bin/* /usr/local/bin/
EXPOSE 8080

ENTRYPOINT [ "/usr/local/bin/api" ]
