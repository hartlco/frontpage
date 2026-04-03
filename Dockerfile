FROM golang:1.24-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /frontpage-server ./cmd/frontpage-server
RUN CGO_ENABLED=0 go build -o /frontpage-cli ./cmd/frontpage-cli

FROM alpine:3.19
RUN apk add --no-cache git git-lfs ca-certificates tzdata
WORKDIR /app
COPY --from=builder /frontpage-server .
COPY --from=builder /frontpage-cli /usr/local/bin/frontpage-cli
COPY templates/ ./templates/
COPY static/ ./static/
EXPOSE 8080
CMD ["./frontpage-server"]
