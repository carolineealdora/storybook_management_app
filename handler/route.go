package handler

import (
	"bytes"
	"context"

	"codebase/handler/controllers/auth"
	"codebase/handler/controllers/healthcheck"
	"codebase/internal/constant/key"
	helpers "codebase/internal/helper"
	"codebase/models"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.elastic.co/apm/module/apmechov4/v2"
)

// InitRoute registers middleware and routes on sharedHolder.Echo. It does not start the server.
func InitRoute(sharedHolder *models.SharedHolder) {
	e := sharedHolder.Echo
	e.HTTPErrorHandler = helpers.NewHTTPErrorHandler(sharedHolder)

	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			requestID := c.Response().Header().Get(echo.HeaderXRequestID)
			ctx := context.WithValue(c.Request().Context(), key.RequestId, requestID)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	})
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins:  sharedHolder.Config.CORSAllowOrigins,
		AllowHeaders:  []string{echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderXRequestID},
		ExposeHeaders: []string{echo.HeaderXRequestID},
	}))
	e.Use(apmechov4.Middleware())
	// Access log only: method, path, status, latency. Bodies are never logged (passwords, PINs, tokens, EPUBs).
	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "[${time_rfc3339}] | ${status} | ${method} ${path} | ${latency_human} | ip=${remote_ip} | request_id=${id}\n",
		Output: e.Logger.Output(),
		CustomTagFunc: func(c echo.Context, buf *bytes.Buffer) (int, error) {
			return buf.WriteString(c.Response().Header().Get(echo.HeaderXRequestID))
		},
	}))

	healthcheckCtrl := healthcheck.NewController(sharedHolder)
	authCtrl := auth.NewController(sharedHolder)

	parentOnly := helpers.RequireAccount(sharedHolder, models.AccountParent)
	adminOnly := helpers.RequireAccount(sharedHolder, models.AccountAdmin)

	v1 := e.Group("/api/v1")
	v1.GET("/healthcheck", healthcheckCtrl.GetHealthcheck) // PUBLIC

	// --- Digital Storybook Library app (parent) ---
	v1.POST("/auth/login", authCtrl.Login)     // PUBLIC
	v1.POST("/auth/refresh", authCtrl.Refresh) // PUBLIC — parent and admin
	v1.POST("/auth/logout", authCtrl.Logout, parentOnly)

	// --- Book Management app (admin) ---
	admin := v1.Group("/admin")
	admin.POST("/auth/login", authCtrl.AdminLogin) // PUBLIC
	admin.POST("/auth/logout", authCtrl.Logout, adminOnly)
}
