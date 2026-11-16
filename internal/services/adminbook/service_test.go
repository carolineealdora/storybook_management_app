package adminbook

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"time"

	respConst "codebase/internal/constant/response"
	helpers "codebase/internal/helper"
	"codebase/models"
	"codebase/models/dto"
	"codebase/test/apitest"
	repository_mock "codebase/test/mock/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var now = time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)

func newSvc() (*ServiceImpl, *repository_mock.BookRepository) {
	repo := new(repository_mock.BookRepository)
	return &ServiceImpl{books: repo, errors: helpers.NewError(nil), now: func() time.Time { return now }}, repo
}

func validInput() dto.BookInput {
	return dto.BookInput{
		Title: "The Hungry Cat", Author: "Ana", Description: "A cat story", AgeType: "range", AgeMin: "1", AgeMax: "3",
		Price: "15000", Status: "published",
		Cover: &dto.Upload{FileName: "cover.PNG", Content: apitest.PNG},
		Epub:  &dto.Upload{FileName: "book.epub", Content: apitest.EPUB},
	}
}

func code(err error) string { return helpers.ErrCode(err) }

// --- Create (US-16) ---

func TestCreate_ValidRange_StoresBookAndCover(t *testing.T) {
	svc, repo := newSvc()
	var book *models.Book
	var cover *models.BookFile
	repo.On("Create", mock.Anything, mock.Anything).Run(func(a mock.Arguments) {
		book, cover = a.Get(0).(*models.Book), a.Get(1).(*models.BookFile)
		book.ID = "book-1"
	}).Return(nil)

	got, err := svc.Create(validInput())
	require.NoError(t, err)
	assert.Equal(t, "book-1", got.ID)
	assert.Equal(t, "1–3", got.AgeLabel)
	assert.Equal(t, 15000, got.Price)
	assert.Equal(t, "/api/v1/books/book-1/cover", got.CoverURL)
	assert.Equal(t, models.AgeRange, book.AgeType)
	require.NotNil(t, book.AgeMax)
	assert.Equal(t, 3, *book.AgeMax)
	assert.True(t, bytes.Equal(apitest.EPUB, book.EpubContent))
	assert.Equal(t, len(apitest.EPUB), book.EpubSizeBytes)
	assert.Equal(t, "image/png", cover.MimeType)
	assert.Equal(t, now, book.CreatedAt)
}

func TestCreate_PlusAgeAndFreeJpeg_Stores(t *testing.T) {
	svc, repo := newSvc()
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	in := validInput()
	in.AgeType, in.AgeMin, in.AgeMax, in.Price = "plus", "3", "", "0"
	in.Cover = &dto.Upload{FileName: "c.jpeg", Content: apitest.JPEG}
	got, err := svc.Create(in)
	require.NoError(t, err)
	assert.Equal(t, "3+", got.AgeLabel)
	assert.Nil(t, got.AgeMax)
	assert.Equal(t, 0, got.Price)
}

func TestCreate_MissingRequiredFields_ReturnsValidationError(t *testing.T) {
	svc, _ := newSvc()
	for _, mutate := range []func(*dto.BookInput){
		func(i *dto.BookInput) { i.Title = "  " },
		func(i *dto.BookInput) { i.Author = "" },
		func(i *dto.BookInput) { i.Description = "" },
		func(i *dto.BookInput) { i.Status = "" },
		func(i *dto.BookInput) { i.Status = "archived" },
		func(i *dto.BookInput) { i.AgeType = "" },
		func(i *dto.BookInput) { i.AgeType = "number" },
		func(i *dto.BookInput) { i.Price = "" },
		func(i *dto.BookInput) { i.Cover = nil },
		func(i *dto.BookInput) { i.Epub = nil },
	} {
		in := validInput()
		mutate(&in)
		_, err := svc.Create(in)
		assert.Equal(t, respConst.VALIDATION_ERROR, code(err), "%+v", in)
	}
}

func TestCreate_InvalidPrice_ReturnsValidationError(t *testing.T) {
	svc, _ := newSvc()
	for _, p := range []string{"-1", "1.5", "abc", "15,000"} {
		in := validInput()
		in.Price = p
		_, err := svc.Create(in)
		assert.Equal(t, respConst.VALIDATION_ERROR, code(err), p)
	}
}

func TestCreate_InvalidAgeRange_ReturnsInvalidAgeRange(t *testing.T) {
	svc, _ := newSvc()
	cases := []struct{ typ, min, max string }{
		{"range", "3", "3"},   // min not lower than max (E16)
		{"range", "4", "3"},   // min > max
		{"range", "1.5", "3"}, // not whole
		{"range", "1", "3.0"}, // not whole
		{"range", "1", ""},    // range needs max
		{"range", "-1", "3"},  // negative
		{"plus", "", ""},      // plus needs number
		{"plus", "2.5", ""},   // not whole
		{"plus", "3", "5"},    // plus has no max
	}
	for _, c := range cases {
		in := validInput()
		in.AgeType, in.AgeMin, in.AgeMax = c.typ, c.min, c.max
		_, err := svc.Create(in)
		assert.Equal(t, respConst.INVALID_AGE_RANGE, code(err), "%+v", c)
	}
}

func TestCreate_InvalidFiles_ReturnsInvalidFileFormat(t *testing.T) {
	svc, _ := newSvc()
	cases := map[string]func(*dto.BookInput){
		"cover gif ext": func(i *dto.BookInput) { i.Cover.FileName = "c.gif" },
		"cover renamed text": func(i *dto.BookInput) {
			i.Cover = &dto.Upload{FileName: "c.png", Content: []byte("hello world, not an image")}
		},
		"cover empty":      func(i *dto.BookInput) { i.Cover = &dto.Upload{FileName: "c.png"} },
		"epub pdf ext":     func(i *dto.BookInput) { i.Epub.FileName = "book.pdf" },
		"epub renamed png": func(i *dto.BookInput) { i.Epub = &dto.Upload{FileName: "book.epub", Content: apitest.PNG} },
		"epub zip but not epub": func(i *dto.BookInput) {
			i.Epub = &dto.Upload{FileName: "book.epub", Content: []byte("PK\x03\x04plainzipfile.txt")}
		},
		"epub without extension": func(i *dto.BookInput) { i.Epub.FileName = "book" },
	}
	for name, mutate := range cases {
		in := validInput()
		mutate(&in)
		_, err := svc.Create(in)
		assert.Equal(t, respConst.INVALID_FILE_FORMAT, code(err), name)
	}
}

func TestCreate_OversizedFiles_ReturnsPayloadTooLarge(t *testing.T) {
	svc, _ := newSvc()
	in := validInput()
	in.Cover.Content = append(append([]byte{}, apitest.PNG...), make([]byte, MaxCoverBytes)...)
	_, err := svc.Create(in)
	assert.Equal(t, respConst.PAYLOAD_TOO_LARGE, code(err))
}

func TestCreate_RepoFails_ReturnsGeneralError(t *testing.T) {
	svc, repo := newSvc()
	repo.On("Create", mock.Anything, mock.Anything).Return(errors.New("db down"))
	_, err := svc.Create(validInput())
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- Update ---

func TestUpdate_WithoutFiles_KeepsExistingFiles(t *testing.T) {
	svc, repo := newSvc()
	existing := &models.Book{ID: "book-1", Title: "Old", Status: models.BookDraft, AgeType: "plus", AgeMin: 3, EpubFileName: "old.epub"}
	repo.On("GetByID", "book-1").Return(existing, nil)
	repo.On("PurchaseCounts", []string{"book-1"}).Return(map[string]int{}, nil)
	var saved *models.Book
	repo.On("Update", mock.Anything, (*models.BookFile)(nil)).Run(func(a mock.Arguments) { saved = a.Get(0).(*models.Book) }).Return(nil)

	in := validInput()
	in.Cover, in.Epub, in.Price = nil, nil, "20000"
	_, err := svc.Update("book-1", in)
	require.NoError(t, err)
	assert.Equal(t, "book-1", saved.ID)
	assert.Equal(t, 20000, saved.Price, "new price affects new purchases only — orders keep their own price")
	assert.Nil(t, saved.EpubContent, "nil content means keep the stored EPUB")
	assert.Equal(t, now, saved.UpdatedAt)
}

func TestUpdate_WithNewFiles_ReplacesThem(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetByID", "book-1").Return(&models.Book{ID: "book-1"}, nil)
	repo.On("PurchaseCounts", []string{"book-1"}).Return(map[string]int{}, nil)
	repo.On("Update", mock.MatchedBy(func(b *models.Book) bool { return len(b.EpubContent) > 0 }),
		mock.MatchedBy(func(f *models.BookFile) bool { return f.MimeType == "image/png" })).Return(nil)
	_, err := svc.Update("book-1", validInput())
	require.NoError(t, err)
	repo.AssertExpectations(t)
}

func TestUpdate_InvalidInput_ReturnsValidationBeforeLookup(t *testing.T) {
	svc, repo := newSvc()
	in := validInput()
	in.Title = ""
	_, err := svc.Update("book-1", in)
	assert.Equal(t, respConst.VALIDATION_ERROR, code(err))
	repo.AssertNotCalled(t, "GetByID", mock.Anything)
}

func TestUpdate_MissingOrDeleted_ReturnsBookNotFound(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetByID", "gone").Return(nil, gorm.ErrRecordNotFound)
	deleted := now
	repo.On("GetByID", "deleted").Return(&models.Book{ID: "deleted", DeletedAt: &deleted}, nil)
	_, err := svc.Update("gone", validInput())
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
	_, err = svc.Update("deleted", validInput())
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
}

func TestUpdate_ConcurrentlyDeleted_ReturnsBookNotFound(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetByID", "book-1").Return(&models.Book{ID: "book-1"}, nil)
	repo.On("Update", mock.Anything, mock.Anything).Return(gorm.ErrRecordNotFound)
	_, err := svc.Update("book-1", validInput())
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
}

// --- Get / List / Delete ---

func TestGet_Deleted_ReturnsBookNotFound(t *testing.T) {
	svc, repo := newSvc()
	deleted := now
	repo.On("GetByID", "b").Return(&models.Book{ID: "b", DeletedAt: &deleted}, nil)
	_, err := svc.Get("b")
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
}

func TestGet_Draft_ReturnedToAdmin(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetByID", "b").Return(&models.Book{ID: "b", Status: models.BookDraft, AgeType: "plus", AgeMin: 2}, nil)
	repo.On("PurchaseCounts", []string{"b"}).Return(map[string]int{}, nil)
	got, err := svc.Get("b")
	require.NoError(t, err)
	assert.Equal(t, models.BookDraft, got.Status)
	assert.Equal(t, "2+", got.AgeLabel)
}

func TestList_PassesSearchAndStatus(t *testing.T) {
	svc, repo := newSvc()
	repo.On("AdminList", "cat", "draft").Return([]models.Book{{ID: "b1", AgeType: "plus"}, {ID: "b2", AgeType: "plus"}}, nil)
	repo.On("PurchaseCounts", []string{"b1", "b2"}).Return(map[string]int{}, nil)
	got, err := svc.List(" cat ", "draft")
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestList_Empty_ReturnsEmptySliceNotNil(t *testing.T) {
	svc, repo := newSvc()
	repo.On("AdminList", "", "").Return(nil, nil)
	got, err := svc.List("", "")
	require.NoError(t, err)
	assert.NotNil(t, got)
}

func TestList_RepoFails_ReturnsGeneralError(t *testing.T) {
	svc, repo := newSvc()
	repo.On("AdminList", "", "").Return(nil, errors.New("db down"))
	_, err := svc.List("", "")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

func TestDelete_Existing_SoftDeletes(t *testing.T) {
	svc, repo := newSvc()
	repo.On("SoftDelete", "b", now).Return(nil)
	require.NoError(t, svc.Delete("b"))
	repo.AssertExpectations(t)
}

func TestDelete_Missing_ReturnsBookNotFound(t *testing.T) {
	svc, repo := newSvc()
	repo.On("SoftDelete", "b", now).Return(gorm.ErrRecordNotFound)
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(svc.Delete("b")))
}

// --- Cover ---

func TestCover_Found_ReturnsFile(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetCover", "b1").Return(&models.BookFile{MimeType: "image/png", Content: []byte("PNG")}, nil)
	got, err := svc.Cover("b1")
	require.NoError(t, err)
	assert.Equal(t, "image/png", got.MimeType)
}

func TestCover_Missing_ReturnsBookNotFound(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetCover", "b1").Return(nil, gorm.ErrRecordNotFound)
	_, err := svc.Cover("b1")
	assert.Equal(t, respConst.BOOK_NOT_FOUND, code(err))
}

func TestCover_DBDown_ReturnsGeneralError(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetCover", "b1").Return(nil, errors.New("conn refused"))
	_, err := svc.Cover("b1")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

// --- purchase_count (STORY-5, design E5) ---

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func TestList_AttachesPurchaseCounts_ZeroWhenUnbought(t *testing.T) {
	svc, repo := newSvc()
	repo.On("AdminList", "", "").Return([]models.Book{{ID: "b1", AgeType: "plus"}, {ID: "b2", AgeType: "plus"}}, nil)
	repo.On("PurchaseCounts", []string{"b1", "b2"}).Return(map[string]int{"b1": 38}, nil)
	got, err := svc.List("", "")
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Contains(t, jsonOf(t, got[0]), `"purchase_count":38`)
	assert.Contains(t, jsonOf(t, got[1]), `"purchase_count":0`)
}

func TestList_PurchaseCountsFail_ReturnsGeneralError(t *testing.T) {
	svc, repo := newSvc()
	repo.On("AdminList", "", "").Return([]models.Book{{ID: "b1", AgeType: "plus"}}, nil)
	repo.On("PurchaseCounts", []string{"b1"}).Return(nil, errors.New("db down"))
	_, err := svc.List("", "")
	assert.Equal(t, respConst.GENERAL_ERROR, code(err))
}

func TestList_NoBooks_SkipsCountQuery(t *testing.T) {
	svc, repo := newSvc()
	repo.On("AdminList", "", "").Return(nil, nil)
	_, err := svc.List("", "")
	require.NoError(t, err)
	repo.AssertNotCalled(t, "PurchaseCounts", mock.Anything)
}

func TestGet_AttachesPurchaseCount(t *testing.T) {
	svc, repo := newSvc()
	repo.On("GetByID", "b").Return(&models.Book{ID: "b", AgeType: "plus"}, nil)
	repo.On("PurchaseCounts", []string{"b"}).Return(map[string]int{"b": 5}, nil)
	got, err := svc.Get("b")
	require.NoError(t, err)
	assert.Contains(t, jsonOf(t, got), `"purchase_count":5`)
}

func TestCreate_NewBook_ZeroPurchasesWithoutQuery(t *testing.T) {
	svc, repo := newSvc()
	repo.On("Create", mock.Anything, mock.Anything).Return(nil)
	got, err := svc.Create(validInput())
	require.NoError(t, err)
	assert.Contains(t, jsonOf(t, got), `"purchase_count":0`)
	repo.AssertNotCalled(t, "PurchaseCounts", mock.Anything)
}
