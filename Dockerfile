### code build stage
FROM golang:latest AS build-env

RUN go get -u github.com/golang/dep/cmd/dep

WORKDIR /go/src/github.com/bluehoodie/crypt-controller

# resolve dependencies
ADD Gopkg.toml .
ADD Gopkg.lock .
RUN dep ensure -vendor-only

# copy code
ADD . .

# build
RUN CGO_ENABLED=0 GOOS=linux go build -o crypt-controller && mv crypt-controller /

### final container build stage
FROM alpine:latest
WORKDIR /app
COPY --from=build-env /crypt-controller /app/
ENTRYPOINT ["./crypt-controller"]