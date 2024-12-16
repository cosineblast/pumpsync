package main

// workflow(user)

// users logs in
// user fills in youtube link
// user selects file to upload
// user clicks run
// loading bar shows up
// suggestion: information about ETA
// user waits...
// eventually: loading bar finishes
// user clicks download
// updated video is downloaded =]

// workflow (js, backend)

// [browser] --> [static server] http(/)
// <-- html
// ... (js, css, png, etc)

// [js gets data from user filling form]
// --> ws(/run)
// >> string message containing json object with youtube link and size of local video
// >> bytes message containing the video itself
// << string message ok (or error)
// << suggestion: messages with status of what the server is doing, ETA
// << string message finished
// << url with video for download (lasts 5 minutes)

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/cosineblast/pumpsync/internal/handle"
	"github.com/cosineblast/pumpsync/internal/video_store"
)

func main() {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	store := video_store.NewVideoStore()

    e.GET("/api/edit", func(c echo.Context) error { return handle.HandleEditRequest(&store, c) })

	e.GET("/api/video/:id", func(c echo.Context) error { return handle.HandleVideoDownloadRequest(&store, c) })

	e.Logger.Fatal(e.Start(":1323"))
}
