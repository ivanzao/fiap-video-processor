FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.22 AS runtime
RUN apk add --no-cache ffmpeg ca-certificates && adduser -D -u 10001 worker
COPY --from=build /out/worker /worker
USER worker
ENV WORK_DIR=/tmp
EXPOSE 9090
ENTRYPOINT ["/worker"]
