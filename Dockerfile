ARG APP_NAME="grumpy"

FROM box-registry.jfrog.io/jenkins/box-centos7-build-golang:1.14.1 as build
ARG APP_NAME
WORKDIR /build
COPY . .
RUN go build -o ${APP_NAME} -mod=vendor main.go

FROM box-registry.jfrog.io/jenkins/box-centos7
ARG APP_NAME
LABEL com.box.name=${APP_NAME}
LABEL maintainer="pass-team@box.com"
COPY --from=build /build/${APP_NAME} /usr/bin
RUN chmod +x /usr/bin/${APP_NAME}
RUN yum -y install notary
COPY ./notary /tmp/notary

## 
## 
## 
## # build stage
## FROM box-registry.jfrog.io/jenkins/box-centos7-build-golang:1.14.1 as build
## 
## 
## FROM golang:1.10-stretch AS build-env
## RUN mkdir -p /go/src/github.com/pipo02mix/grumpy
## WORKDIR /go/src/github.com/pipo02mix/grumpy
## COPY  . .
## RUN useradd -u 10001 webhook
## RUN CGO_ENABLED=0 GOOS=linux go build -a -ldflags '-extldflags "-static"' -o grumpywebhook -mod=vendor
## 
## FROM scratch
## COPY --from=build-env /go/src/github.com/pipo02mix/grumpy/grumpywebhook .
## COPY --from=build-env /etc/passwd /etc/passwd
## USER webhook
## ENTRYPOINT ["/grumpywebhook"]