FROM golang:1.27.1-alpine@sha256:4cb7ac979db5fcc41cae44b2227ba5ab8a51e8807f40d9ba4dee20a0ad960b5b AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=2.0.0
ARG COMMIT=unknown
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o /out/api-engine ./cmd/api-engine \
    && mkdir -p /out/data /out/tmp \
    && chown 65532:65532 /out/data /out/tmp

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /out/api-engine /api-engine
COPY --from=build --chown=65532:65532 /out/data /data
COPY --from=build --chown=65532:65532 /out/tmp /tmp
USER 65532:65532
ENV GOMEMLIMIT=768MiB
EXPOSE 8080
ENTRYPOINT ["/api-engine"]
CMD ["serve", "--config", "/etc/api-engine/config.yml"]
