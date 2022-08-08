# BUILD stage
FROM devopsworks/golang-upx:1.18 as BUILDER

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

WORKDIR /build

COPY . .
RUN go mod download

RUN make build && \
    /usr/local/bin/upx -9 ./out/bin/cfcr

# RUN stage
FROM gcr.io/distroless/base-debian11

WORKDIR /app

COPY --from=builder /build/out/bin/cfcr .

COPY --from=builder /build/config.sample.yaml config.yaml

EXPOSE 2112

ENTRYPOINT [ "/app/cfcr" ]