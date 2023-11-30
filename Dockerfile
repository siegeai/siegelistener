# syntax=docker/dockerfile:1
FROM golang:1.21-alpine3.18 as builder
RUN apk add --no-cache alpine-sdk bash ca-certificates libpcap-dev

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build

# Deploy the application binary into a lean image
FROM alpine:3.18 AS runner
RUN apk add --no-cache ca-certificates libpcap

WORKDIR /root
USER root

COPY --from=builder /src/siegelistener .

ENTRYPOINT ["/root/siegelistener"]
