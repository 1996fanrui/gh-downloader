FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /out/gh-downloader ./cmd/gh-downloader

FROM alpine:3.22

RUN apk add --no-cache ca-certificates

COPY --from=build /out/gh-downloader /usr/local/bin/gh-downloader

EXPOSE 8080
ENTRYPOINT ["gh-downloader"]
