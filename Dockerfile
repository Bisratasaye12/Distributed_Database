# go-sqlite3 needs cgo, so build on a full Debian image with gcc
FROM golang:1.23-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=1 go build -o /out/distsql .

FROM debian:bookworm-slim
COPY --from=build /out/distsql /usr/local/bin/distsql
VOLUME /data
EXPOSE 7946 8946
ENTRYPOINT ["distsql"]
