//go:build integration

package book

import (
	"testing"
	"time"

	"codebase/internal/repositories/testdb"
	"codebase/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newBook(title, status string) (*models.Book, *models.BookFile) {
	now := time.Now().UTC().Truncate(time.Microsecond)
	max := 3
	return &models.Book{Title: title, Author: "A", Description: "D", AgeType: models.AgeRange, AgeMin: 1, AgeMax: &max,
			Price: 15000, Status: status, EpubFileName: "b.epub", EpubSizeBytes: 4, EpubContent: []byte("EPUB"), CreatedAt: now, UpdatedAt: now},
		&models.BookFile{MimeType: "image/png", FileName: "c.png", SizeBytes: 3, Content: []byte("PNG"), CreatedAt: now, UpdatedAt: now}
}

func TestCreateGet_RoundTrip_NeverLoadsEpubContent(t *testing.T) {
	r := &Repo{db: testdb.Open(t)}
	b, c := newBook("Cat", models.BookPublished)
	require.NoError(t, r.Create(b, c))
	require.NotEmpty(t, b.ID)
	assert.Equal(t, b.ID, c.BookID)

	got, err := r.GetByID(b.ID)
	require.NoError(t, err)
	assert.Equal(t, "Cat", got.Title)
	assert.Equal(t, 3, *got.AgeMax)
	assert.Nil(t, got.EpubContent, "detail/list queries must not select epub_content")
	assert.Equal(t, 4, got.EpubSizeBytes)
}

func TestAdminList_SearchStatusAndSoftDelete(t *testing.T) {
	r := &Repo{db: testdb.Open(t)}
	for _, x := range []struct{ title, status string }{{"The Cat", "published"}, {"Big Cat 100%", "draft"}, {"Dog", "published"}} {
		b, c := newBook(x.title, x.status)
		require.NoError(t, r.Create(b, c))
	}
	all, err := r.AdminList("", "")
	require.NoError(t, err)
	assert.Len(t, all, 3)

	cats, err := r.AdminList("cat", "")
	require.NoError(t, err)
	assert.Len(t, cats, 2, "case-insensitive title search")

	pct, err := r.AdminList("100%", "")
	require.NoError(t, err)
	assert.Len(t, pct, 1, "%% is matched literally")

	drafts, err := r.AdminList("", "draft")
	require.NoError(t, err)
	require.Len(t, drafts, 1)

	require.NoError(t, r.SoftDelete(drafts[0].ID, time.Now()))
	all, err = r.AdminList("", "")
	require.NoError(t, err)
	assert.Len(t, all, 2, "soft-deleted books are hidden from the admin list")
	assert.ErrorIs(t, r.SoftDelete(drafts[0].ID, time.Now()), gorm.ErrRecordNotFound, "already deleted")

	kept, err := r.GetByID(drafts[0].ID)
	require.NoError(t, err)
	assert.NotNil(t, kept.DeletedAt, "row is kept for owners")
}

func TestUpdate_WithoutFiles_KeepsEpubAndCover(t *testing.T) {
	db := testdb.Open(t)
	r := &Repo{db: db}
	b, c := newBook("Cat", models.BookDraft)
	require.NoError(t, r.Create(b, c))

	upd := *b
	upd.Title, upd.Price, upd.AgeType, upd.AgeMax, upd.EpubContent = "Cat 2", 20000, models.AgePlus, nil, nil
	require.NoError(t, r.Update(&upd, nil))

	got, err := r.GetByID(b.ID)
	require.NoError(t, err)
	assert.Equal(t, "Cat 2", got.Title)
	assert.Equal(t, 20000, got.Price)
	assert.Nil(t, got.AgeMax)
	var epub [][]byte
	require.NoError(t, db.RawSql(`SELECT epub_content FROM books WHERE id = ?`, &epub, b.ID))
	assert.Equal(t, []byte("EPUB"), epub[0])
}

func TestUpdate_WithFiles_ReplacesEpubAndCover(t *testing.T) {
	db := testdb.Open(t)
	r := &Repo{db: db}
	b, c := newBook("Cat", models.BookDraft)
	require.NoError(t, r.Create(b, c))

	upd := *b
	upd.EpubFileName, upd.EpubSizeBytes, upd.EpubContent = "new.epub", 3, []byte("NEW")
	require.NoError(t, r.Update(&upd, &models.BookFile{MimeType: "image/jpeg", FileName: "n.jpg", SizeBytes: 1, Content: []byte("J")}))

	var mimes []string
	require.NoError(t, db.RawSql(`SELECT mime_type FROM book_files WHERE book_id = ?`, &mimes, b.ID))
	assert.Equal(t, []string{"image/jpeg"}, mimes, "one cover per book, replaced")
	var epub [][]byte
	require.NoError(t, db.RawSql(`SELECT epub_content FROM books WHERE id = ?`, &epub, b.ID))
	assert.Equal(t, []byte("NEW"), epub[0])
}

func TestUpdate_DeletedBook_ReturnsNotFound(t *testing.T) {
	r := &Repo{db: testdb.Open(t)}
	b, c := newBook("Cat", models.BookDraft)
	require.NoError(t, r.Create(b, c))
	require.NoError(t, r.SoftDelete(b.ID, time.Now()))
	assert.ErrorIs(t, r.Update(b, nil), gorm.ErrRecordNotFound)
}
