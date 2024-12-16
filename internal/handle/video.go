package handle

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/cosineblast/pumpsync/internal/video_store"
)

func HandleVideoDownloadRequest(store *video_store.VideoStore, c echo.Context) error {

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
}
