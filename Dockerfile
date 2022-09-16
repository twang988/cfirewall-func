FROM golang:1.17-alpine3.15
ENV CGO_ENABLED=0
WORKDIR /go/src/
COPY go.mod go.sum ./
#RUN export GOPROXY=https://goproxy.cn,direct && go mod download
RUN go mod download
COPY . .
RUN go build -o /usr/local/bin/function ./
FROM alpine:3.15
RUN apk add git
COPY --from=0 /usr/local/bin/function /usr/local/bin/function
ENTRYPOINT ["function"]