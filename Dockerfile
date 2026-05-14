FROM golang:1.25 AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGET=./cmd/api-server
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ${TARGET}

FROM gcr.io/distroless/base-debian12

WORKDIR /app
COPY --from=builder /out/server /app/server
COPY --from=builder /app/data /app/data

EXPOSE 8080 9000 9001/udp

ENTRYPOINT ["/app/server"]
