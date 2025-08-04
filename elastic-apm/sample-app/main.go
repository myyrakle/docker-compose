package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/labstack/echo/v4"
	"go.elastic.co/apm/module/apmechov4/v2"
	"go.elastic.co/apm/v2"
)

func main() {
	e := echo.New()
	e.Use(apmechov4.Middleware())

	e.GET("/", func(c echo.Context) error {
		tx := apm.TransactionFromContext(c.Request().Context())
		tx.Context.SetLabel("endpoint", "home")
		return c.JSON(http.StatusOK, map[string]interface{}{
			"message":   "Hello from Elastic APM monitored Go app!",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]interface{}{
			"status":    "healthy",
			"timestamp": time.Now().Format(time.RFC3339),
		})
	})

	e.GET("/error", func(c echo.Context) error {
		err := fmt.Errorf("This is a test error for APM monitoring")
		apm.CaptureError(c.Request().Context(), err).Send()
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error":   "Something went wrong!",
			"message": err.Error(),
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	e.Logger.Infof("Go Echo server running on port %s", port)
	e.Logger.Fatal(e.Start(":" + port))
}
