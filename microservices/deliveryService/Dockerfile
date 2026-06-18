FROM golang:1.26-alpine AS builder
WORKDIR /src
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	go mod download

COPY rpc ./rpc
COPY deliveryService ./deliveryService
RUN --mount=type=cache,target=/go/pkg/mod \
	--mount=type=cache,target=/root/.cache/go-build \
	go build -trimpath -ldflags "-s -w" -o /out/app ./deliveryService/cmd/delivery

FROM alpine:3.20
WORKDIR /app
RUN adduser -D -H -u 10001 appuser
COPY --from=builder /out/app /app/app
COPY --from=builder /src/deliveryService/cmd/delivery/mcp-config.yaml /app/
USER appuser

EXPOSE 18085 8005
ENTRYPOINT ["/app/app"]