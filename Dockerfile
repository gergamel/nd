FROM golang:1.10.3
WORKDIR /go/src/github.com/gergamel/nd/
RUN go get -d -v github.com/boltdb/bolt github.com/gorilla/mux github.com/gorilla/context
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o nd-server .

FROM alpine:3.8
EXPOSE 8080
VOLUME ["/var/opt/ndel"]
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=0 /go/src/github.com/gergamel/nd/nd-server .
CMD ["./nd-server"]
