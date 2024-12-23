package handle

// workflow (user perspective)

// users logs in
// user fills in youtube link
// user selects file to upload
// user clicks run
// loading icon shows up
// suggestion: information about ETA
// user waits...
// eventually: loading icon finishes
// user clicks download
// updated video is downloaded =]

// workflow (js, backend)

// [browser] --> [static server] http(/)
// <-- html
// ... (js, css, png, etc)

// [js gets data from user filling form]
// --> wss://.../edit
// <<< upgrade to websocket
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
	"log/slog"
	"net/http"
	"regexp"

	"os"

	"github.com/cosineblast/pumpsync/internal/mediasync"
	"github.com/cosineblast/pumpsync/internal/video_store"

	"github.com/gorilla/websocket"

	"github.com/labstack/echo/v4"
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
	Kind     string `json:"type"`      // overwrite_video || overwrite_audio, currently ignored
	VideoId  string `json:"video_id"`  // id of youtube video, base64-esque string
	FileSize int    `json:"file_size"` // size of file, strictly positive and less than the defiend limits
}

type StatusMessage struct {
	Status    string  `json:"status"` // ok || error | done
	ErrorTag *string `json:"error"`
	ResultId  *string `json:"result_id"`
}

func okMessage() StatusMessage {
	return StatusMessage{Status: "ok", ErrorTag: nil, ResultId: nil}
}

func doneMessage(id string) StatusMessage {
	return StatusMessage{Status: "done", ErrorTag: nil, ResultId: &id}
}

func errorMessage(err *responseError) StatusMessage {
	return StatusMessage{Status: "error", ErrorTag: &err.tag}
}

const maxFileSize = 1024 * 1024 * 500

func HandleEditRequest(store *video_store.VideoStore, c echo.Context) error {

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
		ws.WriteJSON(errorMessage(parseError))
		return nil
	}

	if resErr := validateRequest(&request); resErr != nil {
		ws.WriteJSON(errorMessage(resErr))
		return nil
	}

	messageType, reader, err := ws.NextReader()

	if err != nil {
		c.Logger().Error("failed to read file from websocket", err)
		return err
	}

	if messageType != websocket.BinaryMessage {
		c.Logger().Error("expected binary message")
		ws.WriteJSON(errorMessage(protocolViolation))
		return nil
	}

	savedFile, err := saveInputVideoToDisk(reader, request.FileSize)
	defer os.Remove(savedFile)

	c.Logger().Debug("alright! file", savedFile, "saved to disk with size", request.FileSize)

	if err = ws.WriteJSON(okMessage()); err != nil {
		c.Logger().Error("failed write ok status message", err)
		return nil
	}

	youtubeUrl := fmt.Sprintf("http://youtube.com/watch?v=%s", request.VideoId)

	c.Logger().Info("video edited with sucess")

    result, responseErr := tryEditVideo(savedFile, youtubeUrl)

    if responseErr != nil {
        ws.WriteJSON(errorMessage(responseErr))
    }

	notifySuccess(store, ws, result)

	return nil
}

// edits the video with the given request and file, and returns 
// an apropiate response error if it fails
func tryEditVideo(savedFile string, youtubeUrl string) (string, *responseError) {

	result, err := mediasync.ImproveAudio(savedFile, youtubeUrl)

	if err != nil {
		slog.Error("video edit failed", "err", err)

        if errors.Is(err, mediasync.TooLowScoreError) {
            return "", editLocateFailed
        } else if errors.Is(err, mediasync.DownloadError) {
            return "", editDownloadFailed
        } else {
            return "", editFailedGeneric
        }
	}

    return result, nil
}

func validateRequest(request *ProcessingRequest) *responseError {

	if request.FileSize < 0 {
		slog.Error("request had negative file size")
		return negativeFileSize
	}

	if request.FileSize > maxFileSize {
		slog.Error("request size was too big")
		return fileTooBig
	}

	if request.Kind != "overwrite_video" && request.Kind != "overwrite_audio" {
		slog.Error("illegal request kind")
		return protocolViolation
	}

	return validateVideoId(request.VideoId)
}

func validateVideoId(id string) *responseError {

	validIdRegex := "^[a-zA-Z0-9\\-\\_]+$"

	ok, err := regexp.MatchString(validIdRegex, id)

	if err != nil {
		slog.Error("youtube regex compilation failed!")
		return serverError
	}

	if !ok {
		slog.Error("video id did not match regex", "id", id)
		return protocolViolation
	}

	return nil
}

func getUrlPrefix() string {
    prefix := os.Getenv("PUMPSYNC_URL_PREFIX")

    if prefix == "" {
        prefix = "http://127.0.0.1:8000"
    }

    return prefix
}

func notifySuccess(store *video_store.VideoStore, ws *websocket.Conn, resultPath string) error {

	uuid, err := store.AddVideo(resultPath)

	if err != nil {
		return err
	}

    prefix := getUrlPrefix()

	url := fmt.Sprintf("%s/api/video/%s", prefix, uuid.String())

	err = ws.WriteJSON(doneMessage(url))

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
