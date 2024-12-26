
#
# GO BUILD
#

FROM golang:1.23.4-alpine

WORKDIR /src

COPY ./go.mod ./go.sum ./

RUN go mod download

COPY ./main.go ./
COPY ./internal ./internal/
COPY ./res ./res/

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/pumpsync_backend

#
# RUST BUILD
#

FROM rust:1.83-alpine3.20

COPY ./locate /src/

WORKDIR /src

RUN cargo build --release && mkdir -p /app && cp target/release/ps_locate /app/locate_audio

#
# RUNTIME
#

FROM alpine:3.21

RUN apk add yt-dlp

RUN apk add ffmpeg

WORKDIR /app

COPY --from=0 /app/pumpsync_backend /app/
COPY --from=0 /src/res /app/res
COPY --from=1 /app/locate_audio /app/locate_audio

CMD ["/app/pumpsync_backend"]
