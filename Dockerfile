FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod ./
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gateway .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /build/gateway /gateway
EXPOSE 8080
ENTRYPOINT ["/gateway"]
