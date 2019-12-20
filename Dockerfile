FROM golang:1.11 as build
ENV CGO_ENABLED=0

WORKDIR /go/src/github.com/alexellis/faas-containerd
COPY vendor vendor

COPY main.go main.go
COPY weave.go weave.go
COPY netns.go netns.go

RUN gofmt -l -d $(find . -type f -name '*.go' -not -path "./vendor/*") \
    && go test ./... \
    && VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') \
    && GIT_COMMIT=$(git rev-list -1 HEAD) \
    && CGO_ENABLED=0 GOOS=linux go build --ldflags "-s -w \
        -X github.com/alexellis/faas-containerd/version.GitCommit=${GIT_COMMIT}\
        -X github.com/alexellis/faas-containerd/version.Version=${VERSION}" \
        -a -installsuffix cgo -o faas-containerd .

FROM alpine:3.10 as ship

LABEL org.label-schema.license="MIT" \
      org.label-schema.vcs-url="https://github.com/alexellis/faas-containerd" \
      org.label-schema.vcs-type="Git" \
      org.label-schema.name="alexellis/faas-containerd" \
      org.label-schema.vendor="alexellis" \
      org.label-schema.docker.schema-version="1.0"

RUN addgroup -S app \
    && adduser -S -g app app \
    && apk --no-cache add \
    ca-certificates

WORKDIR /home/app

EXPOSE 8080

ENV http_proxy      ""
ENV https_proxy     ""

COPY --from=0 /go/src/github.com/alexellis/faas-containerd/faas-containerd    .
RUN chown -R app:app ./

USER root

CMD ["./faas-containerd"]
