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
RUN yum -y install notary
COPY ./notary /usr/local/notary-utils
COPY --from=build /build/${APP_NAME} /usr/bin
RUN chmod +x /usr/bin/${APP_NAME}
