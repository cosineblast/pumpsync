package main

import (
	"log/slog"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/cosineblast/pumpsync/internal/handle"
	"github.com/cosineblast/pumpsync/internal/video_store"

	"github.com/joho/godotenv"
)

func main() {
    err := godotenv.Load()
    if err != nil {
        slog.Error("Error loading .env file")
        return
    }

    e := setupServer()

    startServer(e)
}

func setupServer() *echo.Echo{
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	store := video_store.NewVideoStore()

	e.GET("/api/edit", func(c echo.Context) error { return handle.HandleEditRequest(&store, c) })

	e.GET("/api/video/:id", func(c echo.Context) error { return handle.HandleVideoDownloadRequest(&store, c) })

    startServer(e)

    return e
}

func startServer(e *echo.Echo) {
    host := os.Getenv("PUMPSYNC_HOST")

    port := os.Getenv("PUMPSYNC_PORT")

    if port == "" {
        port = "8000"
    }

    address := host + ":" + port

    useTLS := os.Getenv("PUMPSYNC_USE_TLS")

    var err error

    if useTLS == "1" {
        certificate := os.Getenv("PUMPSYNC_TLS_CERT")
        key := os.Getenv("PUMPSYNC_TLS_KEY")

        err = e.StartTLS(address, certificate, key)
    } else {
        err = e.Start(address)
    }

    e.Logger.Fatal(err)
}
