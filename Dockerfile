FROM golang:1.21 AS builder

WORKDIR /app

COPY go.mod ./
# Download deps early for better caching. go.sum may be created during build.
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/sockstream ./cmd/sockstream

FROM gcr.io/distroless/static:nonroot

COPY --from=builder /out/sockstream /sockstream

EXPOSE 8080

ENTRYPOINT ["/sockstream"]
