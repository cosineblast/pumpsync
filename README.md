
# pumpsync

`pumpsync` is a tool in development to automatically edit Pump it Up gameplay videos.

[Pump it Up](https://en.wikipedia.org/wiki/Pump_It_Up_(video_game_series)) players often record their gameplay for various reasons.

However, because the gameplay is often recorded in public arcades, it can be hard to listen to the music
being played in the recordings.

On top of that, it is common for players to overlay a video of charting of the song being played, to
make it easier to follow the movements of the gameplay, this can be a tedious task to do.

These are some of the hurdles `pumpsync` aims to help with.

This repository contains the backend code for the pumpsync website.
The source for the frontend can be found at [pumpsync_front](https://github.com/cosineblast/pumpsync_front).

## What it does

TODO


## Running this

A `Dockerfile` is also provided for deployment.

The json API endpoints are documented in (TODO).
The websocket API endpoints are documented in (TODO).

The executable will use the following environment variables:

| Name | Default Value | Description |
|---|---|---|
| PUMPSYNC_HOST | `[::]` | The host this server will listen on |
| PUMPSYNC_PORT | 8000 | The port the server will listen on |
| PUMPSYNC_URL_PREFIX | http://127.0.0.1:8000 | The prefix of the URLs this server will use when generating links to itself to use in request outputs (e.g video download links) |
| PUMPSYNC_DEBUG | 0 | When equal to 1, the server will show the stderr of the commands it executes to stderr |
| PUMPSYNC_USE_TLS | 0 | When equal to 1, the server will use accept TLS for incoming connections |
| PUMPSYNC_TLS_CERT | - | When `PUMPSYNC_USE_TLS` is defined, this variable represents the path to the file where the TLS certificate to be used is stored |
| PUMPSYNC_TLS_KEY | - | When `PUMPSYNC_USE_TLS` is defined, this variable represents the path to a file where the TLS certificate key to be used is stored |

## Developing this

This project uses [devbox](https://github.com/jetify-com/devbox), so you can load all the development dependencies by running `devbox shell`.
Then, to build and run, `go run .`

The program also reads environment variables from `.env` by default.

## How it works


TODO

## Plans

- Implement video overlay
- Replace python locate implementation with pure go implementations (or FFTw)
