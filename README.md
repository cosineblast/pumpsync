
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

## Building this

TODO

## Running this

The json API endpoints are documented in (TODO).
The websocket API endpoints are documented in (TODO).

The executable will use the following environment variables:

| Name | Default Value | Description |
|---|---|---|
| PUMPSYNC_HOST | `[::]` | The host this server will listen on |
| PUMPSYNC_PORT | 8000 | The port the server will listen on |
| PUMPSYNC_URL_PREFIX | http://127.0.0.1:8000 | The prefix of the URLs this server will use when generating links to itself to use in request outputs (e.g video download links) |

A `Dockerfile` is also provided for deployment.

## How it works

TODO

## Plans

- Implement video overlay
- Replace python locate implementation with pure go implementations (or FFTw)
