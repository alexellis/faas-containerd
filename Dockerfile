FROM golang:1.12 as build
ENV CGO_ENABLED=0

RUN mkdir -p /go/src/github.com/genuinetools/
WORKDIR /go/src/github.com/genuinetools/
RUN git clone https://github.com/genuinetools/netns \
  && cd netns \
  && go build --ldflags "-s -w" -a -installsuffix cgo -o /go/bin/netns \
  && /go/bin/netns version

WORKDIR /go/src/github.com/alexellis/faas-containerd

COPY vendor vendor

COPY .git .git
COPY main.go main.go
COPY weave.go weave.go
COPY netns.go netns.go

RUN gofmt -l -d $(find . -type f -name '*.go' -not -path "./vendor/*") \
    && go test ./... \
    && VERSION=$(git describe --all --exact-match `git rev-parse HEAD` | grep tags | sed 's/tags\///') \
    && GIT_COMMIT=$(git rev-list -1 HEAD) \
    && CGO_ENABLED=0 GOOS=linux go build --ldflags "-s -w \
        -X github.com/alexellis/faas-containerd/pkg.GitCommit=${GIT_COMMIT}\
        -X github.com/alexellis/faas-containerd/pkg.Version=${VERSION}" \
        -a -installsuffix cgo -o faas-containerd .

FROM alpine:3.11 as ship

LABEL org.label-schema.license="MIT" \
      org.label-schema.vcs-url="https://github.com/alexellis/faas-containerd" \
      org.label-schema.vcs-type="Git" \
      org.label-schema.name="alexellis/faas-containerd" \
      org.label-schema.vendor="alexellis" \
      org.label-schema.docker.schema-version="1.0"

RUN apk --no-cache add \
    ca-certificates \
    iptables

WORKDIR /home/app

EXPOSE 8080

ENV http_proxy      ""
ENV https_proxy     ""

COPY --from=0 /go/src/github.com/alexellis/faas-containerd/faas-containerd    .
COPY --from=0 /go/bin/netns /usr/local/bin/netns

USER root

CMD ["./faas-containerd"]
