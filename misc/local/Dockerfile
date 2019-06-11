FROM grpc/go

WORKDIR /go/src
COPY entrypoint.sh /go/entrypoint.sh
RUN chmod +x /go/entrypoint.sh

ENTRYPOINT /go/entrypoint.sh
