FROM golang:1.17-alpine3.15
ENV CGO_ENABLED=0
WORKDIR /go/src/
COPY go.mod go.sum ./

RUN export https_proxy=http://147.11.252.42:9090 && apk add ca-certificates openssl
ARG cert_location=/usr/local/share/ca-certificates
RUN openssl s_client -showcerts -connect goproxy.cn:443 </dev/null 2>/dev/null|openssl x509 -outform PEM >  ${cert_location}/goproxy.cn.crt
RUN update-ca-certificates
RUN unset https_proxy && export GOPROXY=https://goproxy.cn,direct && go mod download

#RUN go mod download

COPY . .
RUN go build -o /usr/local/bin/function ./
FROM alpine:3.15
RUN export https_proxy=http://147.11.252.42:9090 && apk add --no-cache git
COPY --from=0 /usr/local/bin/function /usr/local/bin/function
ENTRYPOINT ["function"]