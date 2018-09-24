# build stage
FROM golang:alpine AS build-env
WORKDIR  /go/src/github.com/ecadlabs/rosdump
ADD . .
RUN go build

# final stage
FROM alpine
WORKDIR /app
COPY --from=build-env /go/src/github.com/ecadlabs/rosdump /usr/bin/rosdump
ENTRYPOINT ["/app/customerd"]
CMD ["-c", "/etc/rosdump.yaml"]
