package handler

import (
	"bytes"
	"context"

	"codebase/handler/controllers/adminbook"
	"codebase/handler/controllers/auth"
	"codebase/handler/controllers/healthcheck"
	"codebase/handler/controllers/subscription"
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
	// SECURITY: manual review required
	// CORS only for explicitly configured origins. Echo treats an empty AllowOrigins as "*",
	// so with none configured the middleware is not registered at all: same-origin only.
	if origins := sharedHolder.Config.CORSAllowOrigins; len(origins) > 0 {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:  origins,
			AllowHeaders:  []string{echo.HeaderAuthorization, echo.HeaderContentType, echo.HeaderXRequestID},
			ExposeHeaders: []string{echo.HeaderXRequestID},
		}))
	}
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
	adminBookCtrl := adminbook.NewController(sharedHolder)
	subscriptionCtrl := subscription.NewController(sharedHolder)

	adminOnly := helpers.RequireAccount(sharedHolder, models.AccountAdmin)

	v1 := e.Group("/api/v1")
	v1.GET("/healthcheck", healthcheckCtrl.GetHealthcheck) // PUBLIC

	// SECURITY: manual review required
	// This backend serves only fe_storybook_management_app (ADR-011). Parent endpoints live in
	// storybook_library_app, which enforces the parent-mode token and Forgot PIN lockout.
	v1.POST("/auth/refresh", authCtrl.Refresh)                 // PUBLIC — admin refresh tokens only
	v1.GET("/books/:id/cover", adminBookCtrl.Cover, adminOnly) // path kept for the frontend

	// --- Book Management app (admin) ---
	admin := v1.Group("/admin")
	admin.POST("/auth/login", authCtrl.AdminLogin) // PUBLIC
	admin.POST("/auth/logout", authCtrl.Logout, adminOnly)

	// Uploads are capped per file in the service; this caps the whole request (cover + EPUB + fields).
	adminBooks := admin.Group("/books", adminOnly, middleware.BodyLimit("60M"))
	adminBooks.GET("", adminBookCtrl.List)
	adminBooks.POST("", adminBookCtrl.Create)
	adminBooks.GET("/:id", adminBookCtrl.Get)
	adminBooks.PUT("/:id", adminBookCtrl.Update)
	adminBooks.DELETE("/:id", adminBookCtrl.Delete)

	admin.GET("/subscription-plan", subscriptionCtrl.GetAdminPlan, adminOnly)
	admin.PUT("/subscription-plan", subscriptionCtrl.PutPlan, adminOnly)
}
