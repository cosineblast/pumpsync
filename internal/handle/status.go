package handle

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

func HandleStatusRequest(c echo.Context) error {
    return c.JSON(http.StatusOK, "hi");
}
    
