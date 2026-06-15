# GoReleaser-driven multi-arch build via `dockers_v2`. The binary is
# built outside this Dockerfile (GoReleaser cross-compiles every
# target before invoking docker buildx), and the per-arch artifact is
# organized under $TARGETPLATFORM by GoReleaser so a single Dockerfile
# can produce one manifest list covering linux/amd64 + linux/arm64.
#
# Distroless `static:nonroot` is the minimal base that still ships a
# usable /etc/passwd, ca-certificates, and tzdata — everything ota's
# net/http client needs to talk to Okta over TLS.
FROM gcr.io/distroless/static:nonroot

LABEL org.opencontainers.image.title="ota"
LABEL org.opencontainers.image.description="A k9s-style terminal UI for Okta Workforce Identity"
LABEL org.opencontainers.image.url="https://github.com/tedilabs/ota"
LABEL org.opencontainers.image.source="https://github.com/tedilabs/ota"
LABEL org.opencontainers.image.licenses="Apache-2.0"

ARG TARGETPLATFORM
COPY $TARGETPLATFORM/ota /usr/local/bin/ota

USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/ota"]
