
FROM golang:1.23.4-alpine

# at the current moment we use a simple python script for audio location
# if we don't change our audio detection heursitic (least correlation point) 
# it will probably be replaced with a pure go implementation, as we only need it 
# for the scipy fft convolve

RUN apk add python3

RUN python3 -m venv /opt/the_venv
ENV VIRTUAL_ENV=/opt/the_venv
ENV PATH="$VIRTUAL_ENV/bin:$PATH"

RUN apk add yt-dlp

RUN apk add ffmpeg

WORKDIR /app

COPY ./requirements.txt ./

RUN pip install -r requirements.txt

COPY ./go.mod ./go.sum ./

RUN go mod download

# TODO: only copy relevant files (such as main.go, internal and res/)
COPY . ./

# remember, this is a noop and is just here for documentation
EXPOSE 8000

RUN CGO_ENABLED=0 GOOS=linux go build -o /app/pumsync_backend

CMD ["/app/pumsync_backend"]
