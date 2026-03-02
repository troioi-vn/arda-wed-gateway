FROM golang:1.22-alpine AS build
WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/gateway ./cmd/gateway

FROM alpine:3.20
WORKDIR /srv
COPY --from=build /out/gateway /usr/local/bin/gateway

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/gateway"]
