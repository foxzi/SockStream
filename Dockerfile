FROM --platform=$BUILDPLATFORM golang:1.24 AS builder

ARG TARGETOS
ARG TARGETARCH

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags="-s -w" -o /out/sockstream ./cmd/sockstream

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /out/sockstream /sockstream

EXPOSE 8080

ENTRYPOINT ["/sockstream"]
