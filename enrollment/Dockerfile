FROM golang:1.27.4-alpine3.24@sha256:ea30e25680a5470bac02b08bb0e0a735fb75bd0ab7d431b9b95ee0abef362586 AS build
WORKDIR /src
RUN apk add --no-cache openssl
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN test -z "$(gofmt -l .)" && go test ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -trimpath -ldflags="-s -w -buildid=" -o /out/wifi-enrollment .

FROM alpine:3.24.4@sha256:0f49f331d9212aff11b932863be4eb01cdf962969aa86dab7d1ee21ac6f5f52b
RUN apk add --no-cache ca-certificates openssl \
    && addgroup -g 65532 -S nonroot \
    && adduser -u 65532 -S -D -H -G nonroot nonroot
COPY --from=build --chown=65532:65532 /out/wifi-enrollment /app/wifi-enrollment
USER 65532:65532
ENTRYPOINT ["/app/wifi-enrollment"]
