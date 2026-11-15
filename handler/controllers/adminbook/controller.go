package adminbook

import (
	"errors"
	"io"
	"net/http"
	"strings"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/internal/services/adminbook"
	"codebase/models"
	"codebase/models/dto"

	"github.com/labstack/echo/v4"
)

type Controller struct {
	Response helpers.Response
	Binding  helpers.Binding
	Service  adminbook.Service
}

func NewController(holder *models.SharedHolder) *Controller {
	return &Controller{Response: helpers.NewResponse(holder), Binding: helpers.NewBinding(), Service: adminbook.NewService(holder)}
}

func (c *Controller) List(ec echo.Context) error {
	var q dto.AdminBookListQuery
	if err := helpers.BindValid(c.Binding, ec, &q); err != nil {
		return c.respond(ec, 0, nil, err)
	}
	books, err := c.Service.List(q.Search, q.Status)
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, RespData: books, Meta: map[string]int{"total": len(books)}})
}

func (c *Controller) Get(ec echo.Context) error {
	id, err := helpers.PathUUID(ec, "id")
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	b, err := c.Service.Get(id)
	return c.respond(ec, http.StatusOK, b, err)
}

func (c *Controller) Create(ec echo.Context) error {
	in, err := readBookForm(ec)
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	b, err := c.Service.Create(in)
	return c.respond(ec, http.StatusCreated, b, err)
}

func (c *Controller) Update(ec echo.Context) error {
	id, err := helpers.PathUUID(ec, "id")
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	in, err := readBookForm(ec)
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	b, err := c.Service.Update(id, in)
	return c.respond(ec, http.StatusOK, b, err)
}

func (c *Controller) Delete(ec echo.Context) error {
	id, err := helpers.PathUUID(ec, "id")
	if err != nil {
		return c.respond(ec, 0, nil, err)
	}
	if err := c.Service.Delete(id); err != nil {
		return c.respond(ec, 0, nil, err)
	}
	return ec.NoContent(http.StatusNoContent)
}

func (c *Controller) respond(ec echo.Context, status int, data any, err error) error {
	return c.Response.CommonResponse(models.ResponseArgs{Ctx: ec, Status: status, RespData: data, ErrorData: err})
}

// readBookForm reads the multipart form: text fields plus optional "cover" and "epub" files.
func readBookForm(ec echo.Context) (dto.BookInput, error) {
	if !strings.HasPrefix(ec.Request().Header.Get(echo.HeaderContentType), echo.MIMEMultipartForm) {
		return dto.BookInput{}, helpers.NewError(nil).Wrap(respConst.VALIDATION_ERROR, "request must be multipart/form-data.")
	}
	in := dto.BookInput{
		Title: ec.FormValue("title"), Author: ec.FormValue("author"), Description: ec.FormValue("description"),
		AgeType: ec.FormValue("age_type"), AgeMin: ec.FormValue("age_min"), AgeMax: ec.FormValue("age_max"),
		Price: ec.FormValue("price"), Status: ec.FormValue("status"),
	}
	var err error
	if in.Cover, err = readUpload(ec, "cover"); err != nil {
		return in, err
	}
	if in.Epub, err = readUpload(ec, "epub"); err != nil {
		return in, err
	}
	return in, nil
}

func readUpload(ec echo.Context, field string) (*dto.Upload, error) {
	fh, err := ec.FormFile(field)
	if errors.Is(err, http.ErrMissingFile) {
		return nil, nil
	}
	if err != nil {
		return nil, helpers.NewError(nil).Wrap(respConst.VALIDATION_ERROR, field+" could not be read.")
	}
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only multipart part
	// Read one byte past the largest allowed size so the service can still reject oversized files.
	content, err := io.ReadAll(io.LimitReader(f, adminbook.MaxEpubBytes+1))
	if err != nil {
		return nil, err
	}
	return &dto.Upload{FileName: fh.Filename, Content: content}, nil
}
