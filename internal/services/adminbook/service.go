package adminbook

import (
	"bytes"
	"errors"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/internal/repositories/book"
	"codebase/models"
	"codebase/models/dto"
	"codebase/pkg/shared"

	"gorm.io/gorm"
)

const (
	MaxCoverBytes = 5 << 20  // 5 MB
	MaxEpubBytes  = 50 << 20 // 50 MB
)

type (
	Service interface {
		List(search, status string) ([]dto.AdminBook, error)
		Get(id string) (*dto.AdminBook, error)
		Create(in dto.BookInput) (*dto.AdminBook, error)
		Update(id string, in dto.BookInput) (*dto.AdminBook, error)
		Delete(id string) error
		Cover(id string) (*models.BookFile, error)
	}

	ServiceImpl struct {
		books  book.Repository
		errors helpers.Error
		now    shared.Now
	}
)

func NewService(holder *models.SharedHolder) Service {
	return &ServiceImpl{books: book.NewRepository(holder), errors: helpers.NewError(holder), now: time.Now}
}

func (s *ServiceImpl) List(search, status string) ([]dto.AdminBook, error) {
	books, err := s.books.AdminList(strings.TrimSpace(search), status)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	out := make([]dto.AdminBook, 0, len(books))
	if len(books) == 0 {
		return out, nil
	}
	ids := make([]string, len(books))
	for i, b := range books {
		ids[i] = b.ID
	}
	counts, err := s.books.PurchaseCounts(ids)
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	for _, b := range books {
		v := toAdminBook(b)
		v.PurchaseCount = counts[b.ID]
		out = append(out, v)
	}
	return out, nil
}

func (s *ServiceImpl) Get(id string) (*dto.AdminBook, error) {
	b, err := s.live(id)
	if err != nil {
		return nil, err
	}
	counts, err := s.books.PurchaseCounts([]string{id})
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	v := toAdminBook(*b)
	v.PurchaseCount = counts[id]
	return &v, nil
}

// Create returns the new book with PurchaseCount 0: nobody can own it yet.
func (s *ServiceImpl) Create(in dto.BookInput) (*dto.AdminBook, error) {
	if in.Cover == nil || in.Epub == nil {
		return nil, s.errors.Wrap(respConst.VALIDATION_ERROR, "cover and epub files are required.")
	}
	b, cover, err := s.parse(in)
	if err != nil {
		return nil, err
	}
	b.CreatedAt = b.UpdatedAt
	if err := s.books.Create(b, cover); err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	v := toAdminBook(*b)
	return &v, nil
}

// Update replaces the metadata. Files are replaced only when sent. A new price applies to new orders only;
// existing orders and purchases keep the price they stored (PRD US-16, E7).
func (s *ServiceImpl) Update(id string, in dto.BookInput) (*dto.AdminBook, error) {
	b, cover, err := s.parse(in)
	if err != nil {
		return nil, err
	}
	if _, err := s.live(id); err != nil {
		return nil, err
	}
	b.ID = id
	if err := s.books.Update(b, cover); errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.BOOK_NOT_FOUND, "deleted during update")
	} else if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return s.Get(id)
}

func (s *ServiceImpl) Delete(id string) error {
	if err := s.books.SoftDelete(id, s.now()); errors.Is(err, gorm.ErrRecordNotFound) {
		return s.errors.Wrap(respConst.BOOK_NOT_FOUND, "book "+id)
	} else if err != nil {
		return s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return nil
}

// Cover returns the cover of any book the admin can see, drafts included.
func (s *ServiceImpl) Cover(id string) (*models.BookFile, error) {
	f, err := s.books.GetCover(id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, s.errors.Wrap(respConst.BOOK_NOT_FOUND, "cover "+id)
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return f, nil
}

// live returns a book that exists and is not soft-deleted.
func (s *ServiceImpl) live(id string) (*models.Book, error) {
	b, err := s.books.GetByID(id)
	if errors.Is(err, gorm.ErrRecordNotFound) || (err == nil && b.DeletedAt != nil) {
		return nil, s.errors.Wrap(respConst.BOOK_NOT_FOUND, "book "+id)
	}
	if err != nil {
		return nil, s.errors.Wrap(respConst.GENERAL_ERROR, err.Error())
	}
	return b, nil
}

// parse validates the form (PRD US-16, E15, E16) and builds the rows to store.
func (s *ServiceImpl) parse(in dto.BookInput) (*models.Book, *models.BookFile, error) {
	invalid := func(msg string) (*models.Book, *models.BookFile, error) {
		return nil, nil, s.errors.Wrap(respConst.VALIDATION_ERROR, msg)
	}
	b := &models.Book{
		Title: strings.TrimSpace(in.Title), Author: strings.TrimSpace(in.Author), Description: strings.TrimSpace(in.Description),
		AgeType: strings.TrimSpace(in.AgeType), Status: strings.TrimSpace(in.Status), UpdatedAt: s.now(),
	}
	for field, v := range map[string]string{"title": b.Title, "author": b.Author, "description": b.Description, "status": b.Status, "age_type": b.AgeType} {
		if v == "" {
			return invalid(field + " is a required field.")
		}
	}
	if len(b.Title) > 255 || len(b.Author) > 255 {
		return invalid("title and author must be at most 255 characters long.")
	}
	if b.Status != models.BookDraft && b.Status != models.BookPublished {
		return invalid("status must be one of: draft published.")
	}
	if b.AgeType != models.AgeRange && b.AgeType != models.AgePlus {
		return invalid("age_type must be one of: range plus.")
	}
	price, ok := wholeNumber(in.Price)
	if !ok {
		return invalid("price must be a whole number of at least 0.")
	}
	b.Price = price

	if b.AgeMin, ok = wholeNumber(in.AgeMin); !ok {
		return nil, nil, s.errors.Wrap(respConst.INVALID_AGE_RANGE, "age_min "+in.AgeMin)
	}
	maxRaw := strings.TrimSpace(in.AgeMax)
	switch {
	case b.AgeType == models.AgePlus && maxRaw != "":
		return nil, nil, s.errors.Wrap(respConst.INVALID_AGE_RANGE, "plus age has no max")
	case b.AgeType == models.AgeRange:
		max, ok := wholeNumber(maxRaw)
		if !ok || b.AgeMin >= max {
			return nil, nil, s.errors.Wrap(respConst.INVALID_AGE_RANGE, "age range "+in.AgeMin+"-"+in.AgeMax)
		}
		b.AgeMax = &max
	}

	var cover *models.BookFile
	if in.Cover != nil {
		if len(in.Cover.Content) > MaxCoverBytes {
			return nil, nil, s.errors.Wrap(respConst.PAYLOAD_TOO_LARGE, "cover over 5 MB")
		}
		mime, ok := coverMime(*in.Cover)
		if !ok {
			return nil, nil, s.errors.Wrap(respConst.INVALID_FILE_FORMAT, "cover "+in.Cover.FileName)
		}
		cover = &models.BookFile{MimeType: mime, FileName: filepath.Base(in.Cover.FileName), SizeBytes: len(in.Cover.Content),
			Content: in.Cover.Content, CreatedAt: b.UpdatedAt, UpdatedAt: b.UpdatedAt}
	}
	if in.Epub != nil {
		if len(in.Epub.Content) > MaxEpubBytes {
			return nil, nil, s.errors.Wrap(respConst.PAYLOAD_TOO_LARGE, "epub over 50 MB")
		}
		if !isEpub(*in.Epub) {
			return nil, nil, s.errors.Wrap(respConst.INVALID_FILE_FORMAT, "epub "+in.Epub.FileName)
		}
		b.EpubFileName, b.EpubSizeBytes, b.EpubContent = filepath.Base(in.Epub.FileName), len(in.Epub.Content), in.Epub.Content
	}
	return b, cover, nil
}

func wholeNumber(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil && n >= 0
}

// coverMime accepts .jpg/.jpeg/.png whose bytes really are JPEG or PNG.
func coverMime(f dto.Upload) (string, bool) {
	switch strings.ToLower(filepath.Ext(f.FileName)) {
	case ".jpg", ".jpeg", ".png":
	default:
		return "", false
	}
	if len(f.Content) == 0 {
		return "", false
	}
	mime := http.DetectContentType(f.Content)
	return mime, mime == "image/jpeg" || mime == "image/png"
}

// isEpub accepts .epub files that are ZIP archives declaring the EPUB mimetype up front (OCF spec).
// ponytail: header sniff only, no full ZIP/OPF parse; add one if malformed EPUBs reach readers.
func isEpub(f dto.Upload) bool {
	if strings.ToLower(filepath.Ext(f.FileName)) != ".epub" || !bytes.HasPrefix(f.Content, []byte("PK\x03\x04")) {
		return false
	}
	head := f.Content[:min(len(f.Content), 200)]
	return bytes.Contains(head, []byte("application/epub+zip"))
}

func toAdminBook(b models.Book) dto.AdminBook {
	return dto.AdminBook{
		ID: b.ID, Title: b.Title, Author: b.Author, Description: b.Description, AgeType: b.AgeType, AgeMin: b.AgeMin,
		AgeMax: b.AgeMax, AgeLabel: b.AgeLabel(), Price: b.Price, Status: b.Status, CoverURL: b.CoverURL(),
		EpubFileName: b.EpubFileName, EpubSizeBytes: b.EpubSizeBytes, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt,
	}
}
