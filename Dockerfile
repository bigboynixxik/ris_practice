FROM golang:1.26.7-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
COPY pkg ./pkg
RUN CGO_ENABLED=0 go build -trimpath -o /out/ris ./cmd/ris

FROM alpine:3.23

RUN apk add --no-cache ca-certificates
COPY --from=build /out/ris /usr/local/bin/ris

WORKDIR /app
ENTRYPOINT ["/usr/local/bin/ris"]
