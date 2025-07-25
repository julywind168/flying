package login

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/julywind168/flying/demo/proto"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func Start() {
	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.POST("/login", login)
	e.POST("/register", register)

	// Start server
	if err := e.Start(":9999"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}

// Handler
func login(c echo.Context) error {
	var payload proto.LoginPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, proto.Result{
			Code:    proto.ErrCodeBadRequest,
			Message: err.Error(),
		})
	}
	return c.JSON(http.StatusOK, proto.Result{
		Code:    proto.ErrCodeSuccess,
		Message: "Login successful",
	})
}

func register(c echo.Context) error {
	var payload proto.RegisterPayload
	if err := c.Bind(&payload); err != nil {
		return c.JSON(http.StatusBadRequest, proto.Result{
			Code:    proto.ErrCodeBadRequest,
			Message: err.Error(),
		})
	}
	return c.JSON(http.StatusOK, proto.Result{
		Code:    proto.ErrCodeSuccess,
		Message: "Register successful",
	})
}
