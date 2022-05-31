FROM alpine
COPY bin/bundle /app/
WORKDIR /app
ENTRYPOINT ["/app/bundle"]