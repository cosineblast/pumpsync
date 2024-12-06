
# pumpsync

`pumpsync` is a tool in development to automatically edit Pump it Up gameplay videos.

[Pump it Up](https://en.wikipedia.org/wiki/Pump_It_Up_(video_game_series)) players often record their gameplay for various reasons. 

However, because the gameplay is often recorded in public arcades, it can be hard to listen to the music
being played in the recordings.

On top of that, it is common for players to overlay a video of charting of the song being played, to 
make it easier to follow the movements of the gameplay, this can be a tedious task to do. 

These are some of the hurdles `pumpsync` aims to help with.

## Using it 

Currently, `pumpsync` is just a simple CLI program, which will be expanded to a website in a near future.
It supports overwriting the music audio of a pump it up gameplay video with the audio of an youtube gameplay recording:

``` 
./pumpsync \
    --bg media/papasito_bg.mp4 \
    --link "https://www.youtube.com/watch?v=SqtyWdTB_9k" \
    -o papasito_modified.mp4
```

This command will read a noisy gameplay video from `papasito_bg.mp4`, download the audio from the chart video in the given youtube link, 
and create a new video file named `papasito_modified.mp4`, containing an edit of the original video, the music properly
overwritten at the correct timing. 

`pumpsync` is aware that some youtube videos have an intro and an outro, for things such as music selection and score screen, 
and is able to automatically trim those out for videos which have an XX or Phoenix intro/outro.

### Building it

To run this project, you will need the following dependencies (last tested version in parenthesis):

- `yt-dlp` (2024.11.18)
- `golang` (1.23)
- `python3` (3.12)
- `ffmepg` (7.1)

Additionally, you will need the python dependencies in the `requirements.txt` file, which can be installed with `pip install -r requirements.txt`.

The project can be ran with `go run .`, or a binary can be produced with `go build .` and ran with `./pumpsync`.

This will be expanded in the future, and the python dependencies will probably get replaced by pure Go packages.

In order to run, the project needs the resource files located in the `res` directory.

## How it works

TODO: Document this.

## Plans

- Implement video overlay
- Replace python locate implementation with pure go implementations (or FFTw)
- Package with nix
- Make a website for this
