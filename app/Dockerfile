FROM golang:1.27 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /proxy ./cmd/api

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /proxy /proxy
EXPOSE 8080
ENTRYPOINT ["/proxy"]
