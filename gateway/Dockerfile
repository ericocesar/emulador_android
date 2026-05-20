FROM golang:1.22-alpine AS builder
RUN apk add --no-cache gcc musl-dev git
WORKDIR /app
COPY go.mod ./
COPY . .
RUN go mod tidy && CGO_ENABLED=0 go build -o /gateway .

FROM alpine:3.19
RUN apk add --no-cache android-tools ca-certificates tzdata docker-cli
COPY --from=builder /gateway /gateway
EXPOSE 8080
CMD ["/gateway"]
