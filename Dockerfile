FROM alpine
COPY bin/ /app/
WORKDIR /app
ENTRYPOINT ["/app/bundle-controller"]