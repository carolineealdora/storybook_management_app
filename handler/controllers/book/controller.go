package book

import (
	"net/http"

	helpers "codebase/internal/helper"
	"codebase/internal/services/book"
	"codebase/models"

	"github.com/labstack/echo/v4"
)

type Controller struct {
	Response helpers.Response
	Service  book.Service
}

func NewController(holder *models.SharedHolder) *Controller {
	return &Controller{Response: helpers.NewResponse(holder), Service: book.NewService(holder)}
}

func (c *Controller) List(ec echo.Context) error {
	cards, err := c.Service.List(helpers.ClaimsFrom(ec).UserID)
	if err != nil {
		return c.fail(ec, err)
	}
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: cards, Meta: map[string]int{"total": len(cards)}})
}

func (c *Controller) Get(ec echo.Context) error {
	id, err := helpers.PathUUID(ec, "id")
	if err != nil {
		return c.fail(ec, err)
	}
	d, err := c.Service.Get(helpers.ClaimsFrom(ec).UserID, id)
	if err != nil {
		return c.fail(ec, err)
	}
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: d})
}

// Cover serves the cover image to parent and admin apps (covers are not paywalled).
func (c *Controller) Cover(ec echo.Context) error {
	id, err := helpers.PathUUID(ec, "id")
	if err != nil {
		return c.fail(ec, err)
	}
	f, err := c.Service.Cover(id)
	if err != nil {
		return c.fail(ec, err)
	}
	ec.Response().Header().Set("Cache-Control", "private, max-age=3600")
	return ec.Blob(http.StatusOK, f.MimeType, f.Content)
}

// Content streams the EPUB after the access check. no-store: access can be revoked by cancelling (E4).
func (c *Controller) Content(ec echo.Context) error {
	id, err := helpers.PathUUID(ec, "id")
	if err != nil {
		return c.fail(ec, err)
	}
	b, err := c.Service.Content(helpers.ClaimsFrom(ec).UserID, id)
	if err != nil {
		return c.fail(ec, err)
	}
	h := ec.Response().Header()
	h.Set("Cache-Control", "no-store")
	h.Set("Content-Disposition", "inline")
	return ec.Blob(http.StatusOK, "application/epub+zip", b.EpubContent)
}

func (c *Controller) MyBooks(ec echo.Context) error {
	books, err := c.Service.MyBooks(helpers.ClaimsFrom(ec).UserID)
	if err != nil {
		return c.fail(ec, err)
	}
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: books, Meta: map[string]int{"total": len(books)}})
}

func (c *Controller) fail(ec echo.Context, err error) error {
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, ErrorData: err})
}
