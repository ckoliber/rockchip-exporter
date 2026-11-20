FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build
ARG TARGETOS TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build -trimpath -ldflags="-s -w" -o /rockchip-exporter .

FROM gcr.io/distroless/static-debian12
COPY --from=build /rockchip-exporter /rockchip-exporter
EXPOSE 9988
ENTRYPOINT ["/rockchip-exporter"]
