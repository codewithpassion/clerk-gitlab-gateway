FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gateway .

FROM alpine:3.21
RUN adduser -D -u 65532 nonroot
COPY --from=builder /build/gateway /gateway
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/gateway"]
