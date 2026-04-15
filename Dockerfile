FROM golang:1.26.2-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o /migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12

COPY --from=builder /server /server
COPY --from=builder /migrate /migrate
COPY db/migrations /migrations

USER nonroot:nonroot
EXPOSE 8090

ENTRYPOINT ["/server"]
