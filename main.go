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
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"

	"os"

	"the_thing/internal/mediasync"
	"the_thing/internal/video_store"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/joho/godotenv"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// TODO: use proper CORS
			return true
		},
	}
)

type ProcessingRequest struct {
	Kind      string `json:"type"` // overwrite_video || overwrite_audio, currently ignored
	VideoLink string `json:"link"`
	FileSize  int    `json:"file_size"`
}

type StatusMessage struct {
	Status    string  `json:"status"` // ok || error | done
	ErrorName *string `json:"error"`
	ResultId  *string `json:"result_id"`
}

func okStatus() StatusMessage {
	return StatusMessage{Status: "ok", ErrorName: nil, ResultId: nil}
}

func doneStatus(id string) StatusMessage {
	return StatusMessage{Status: "done", ErrorName: nil, ResultId: &id}
}

func errorStatus(errorName string) StatusMessage {
	return StatusMessage{Status: "error", ErrorName: &errorName}
}

const maxFileSize = 1024 * 1024 * 500

var fileTooBig = errors.New("file_too_big")

var parseError = errors.New("parse_error")

var negativeFileSize = errors.New("negative_size")

var protocolViolation = errors.New("protocol_violation")

var editFailed = errors.New("edit_failed")

// TODO: simplify this function
func handleRun(store *video_store.VideoStore, c echo.Context) error {

	c.Logger().Info("got request!")

	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)

	if err != nil {
		c.Logger().Error("failed to upgrade to websocket", err)
		return err
	}

	defer ws.Close()

	var request ProcessingRequest

	if err = ws.ReadJSON(&request); err != nil {
		c.Logger().Error("failed to read json info from websocket", err)
		ws.WriteJSON(errorStatus(parseError.Error()))
		return nil
	}

	err = validateRequest(&request)

	if err != nil {
		ws.WriteJSON(errorStatus(err.Error()))
		return nil
	}

	messageType, reader, err := ws.NextReader()

	if err != nil {
		c.Logger().Error("failed to read file from websocket", err)
		return err
	}

	if messageType != websocket.BinaryMessage {
		c.Logger().Error("expected binary message")
		ws.WriteJSON(errorStatus(protocolViolation.Error()))
		return nil
	}

	savedFile, err := saveInputVideoToDisk(reader, request.FileSize)
	defer os.Remove(savedFile)

	c.Logger().Debug("alright! file", savedFile, "saved to disk with size", request.FileSize)

	if err = ws.WriteJSON(okStatus()); err != nil {
		c.Logger().Error("failed write ok status message", err)
		return nil
	}

	result, err := mediasync.ImproveAudio(savedFile, request.VideoLink)

	if err != nil {
		c.Logger().Error("video edit failed", err)
		ws.WriteJSON(errorStatus(editFailed.Error()))
		return nil
	}

	c.Logger().Info("video edited with sucess")

	notifySuccess(store, ws, result)

	return nil
}

func validateRequest(request *ProcessingRequest) error {

	if request.FileSize < 0 {
		slog.Error("request had negative file size")
		return negativeFileSize
	}

	if request.FileSize > maxFileSize {
		slog.Error("request size was too big")
		return fileTooBig
	}

	return nil
}

func notifySuccess(store *video_store.VideoStore, ws *websocket.Conn, resultPath string) error {

	uuid, err := store.AddVideo(resultPath)

	if err != nil {
		return err
	}

	url := fmt.Sprintf("http://127.0.0.1:1323/video/%s", uuid.String())

	err = ws.WriteJSON(doneStatus(url))

	if err != nil {
		return err
	}

	return nil
}

func saveInputVideoToDisk(reader io.Reader, expectedSize int) (string, error) {

	file, err := os.CreateTemp("", "pumpsync_server_*_input")

	if err != nil {
		return "", err
	}

	defer func() {
		if err != nil {
			os.Remove(file.Name())
		}
	}()

	path := file.Name()

	defer file.Close()

	_, err = io.CopyN(file, reader, int64(expectedSize))

	if err != nil {
		return "", nil
	}

	return path, nil
}

func main() {

	// First thing we have to do is setup .env
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading .env file")
		os.Exit(1)
	}

	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	store := video_store.NewVideoStore()

	e.GET("/run", func(c echo.Context) error { return handleRun(&store, c) })

	e.GET("/video/:id", func(c echo.Context) error {

		id := c.Param("id")

		uid, err := uuid.Parse(id)

		if err != nil {
			c.Logger().Error("invalid uid", uid)
			return c.String(http.StatusNotFound, "")
		}

		result := store.FetchVideo(uid)

		if result == nil {
			return c.String(http.StatusNotFound, "")
		}

		return c.Attachment(*result, "result.mp4")
	})

	e.Logger.Fatal(e.Start(":1323"))
}
