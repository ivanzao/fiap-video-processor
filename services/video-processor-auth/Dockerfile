ARG LAMBDA_NAME=login

FROM golang:1.27-alpine AS deps
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .

FROM deps AS build-local
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/local ./cmd/local

FROM gcr.io/distroless/static-debian12:nonroot AS local
COPY --from=build-local /out/local /local
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/local"]

FROM deps AS build-migrate
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot AS migrate
COPY --from=build-migrate /out/migrate /migrate
USER nonroot:nonroot
ENTRYPOINT ["/migrate"]

FROM deps AS build-lambda
ARG LAMBDA_NAME
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags="-s -w" -o /out/bootstrap ./cmd/${LAMBDA_NAME}

FROM public.ecr.aws/lambda/provided:al2023-arm64 AS lambda
COPY --from=build-lambda /out/bootstrap /var/runtime/bootstrap
CMD ["bootstrap"]
