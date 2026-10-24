# Build the AxLang command-line tools for the target platform.
FROM --platform=$BUILDPLATFORM golang:1.26-alpine AS build

ARG TARGETOS
ARG TARGETARCH

WORKDIR /src
COPY . .

RUN mkdir -p /out \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/axc ./cmd/axc \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w" -o /out/axls ./cmd/axls

# The image includes both tools. axc is the default command; use --entrypoint
# /usr/local/bin/axls when running the language server over stdio.
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.source="https://github.com/vivek5200/axlang" \
      org.opencontainers.image.description="AxLang compiler, formatter, and language server" \
      org.opencontainers.image.licenses="Apache-2.0"

COPY --from=build /out/axc /usr/local/bin/axc
COPY --from=build /out/axls /usr/local/bin/axls

ENTRYPOINT ["/usr/local/bin/axc"]
