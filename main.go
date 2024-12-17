package main

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
