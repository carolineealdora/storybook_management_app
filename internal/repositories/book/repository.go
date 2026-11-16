package book

import (
	"strings"
	"time"

	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// columns excludes epub_content: book lists and details never load the EPUB (PRD §15).
var columns = []string{"id", "title", "author", "description", "age_type", "age_min", "age_max", "price", "status",
	"epub_file_name", "epub_size_bytes", "created_at", "updated_at", "deleted_at"}

type (
	Repository interface {
		GetByID(id string) (*models.Book, error) // includes soft-deleted rows; callers decide visibility
		AdminList(search, status string) ([]models.Book, error)
		Create(book *models.Book, cover *models.BookFile) error
		Update(book *models.Book, cover *models.BookFile) error // nil EpubContent / cover = keep stored file
		SoftDelete(id string, at time.Time) error

		GetCover(bookID string) (*models.BookFile, error)
		PurchaseCounts(bookIDs []string) (map[string]int, error)
	}

	Repo struct {
		db sql.ORM
	}
)

func NewRepository(holder *models.SharedHolder) Repository {
	return &Repo{db: holder.Sql}
}

func (r *Repo) GetByID(id string) (*models.Book, error) {
	var b models.Book
	if err := r.db.Table("books").Select(columns).Where("id = ?", id).First(&b); err != nil {
		return nil, err
	}
	return &b, nil
}

func (r *Repo) AdminList(search, status string) ([]models.Book, error) {
	q := r.db.Table("books").Select(columns).Where("deleted_at IS NULL")
	if search != "" {
		q = q.Where(`title ILIKE ? ESCAPE '\'`, "%"+escapeLike(search)+"%")
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	var books []models.Book
	if _, err := q.Order("created_at DESC").All(&books); err != nil {
		return nil, err
	}
	return books, nil
}

func (r *Repo) Create(book *models.Book, cover *models.BookFile) error {
	if book.ID == "" {
		book.ID = uuid.NewString()
	}
	return sql.WithTx(r.db, func(tx sql.ORM) error {
		if err := tx.Create(book); err != nil {
			return err
		}
		cover.ID, cover.BookID = uuid.NewString(), book.ID
		return tx.Create(cover)
	})
}

func (r *Repo) Update(b *models.Book, cover *models.BookFile) error {
	return sql.WithTx(r.db, func(tx sql.ORM) error {
		q := `UPDATE books SET title = ?, author = ?, description = ?, age_type = ?, age_min = ?, age_max = ?, price = ?, status = ?, updated_at = ?`
		args := []any{b.Title, b.Author, b.Description, b.AgeType, b.AgeMin, b.AgeMax, b.Price, b.Status, b.UpdatedAt}
		if b.EpubContent != nil {
			q += `, epub_file_name = ?, epub_size_bytes = ?, epub_content = ?`
			args = append(args, b.EpubFileName, b.EpubSizeBytes, b.EpubContent)
		}
		q += ` WHERE id = ? AND deleted_at IS NULL RETURNING id`
		args = append(args, b.ID)
		var ids []string
		if err := tx.RawSql(q, &ids, args...); err != nil {
			return err
		}
		if len(ids) == 0 {
			return gorm.ErrRecordNotFound
		}
		if cover == nil {
			return nil
		}
		return tx.Exec(`INSERT INTO book_files (id, book_id, mime_type, file_name, size_bytes, content, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT (book_id) DO UPDATE SET mime_type = EXCLUDED.mime_type, file_name = EXCLUDED.file_name,
				size_bytes = EXCLUDED.size_bytes, content = EXCLUDED.content, updated_at = EXCLUDED.updated_at`,
			uuid.NewString(), b.ID, cover.MimeType, cover.FileName, cover.SizeBytes, cover.Content, b.UpdatedAt, b.UpdatedAt)
	})
}

// SoftDelete hides the book from the showcase; purchases keep pointing at it (PRD E5).
func (r *Repo) SoftDelete(id string, at time.Time) error {
	var ids []string
	if err := r.db.RawSql(`UPDATE books SET deleted_at = ?, updated_at = ? WHERE id = ? AND deleted_at IS NULL RETURNING id`, &ids, at, at, id); err != nil {
		return err
	}
	if len(ids) == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func (r *Repo) GetCover(bookID string) (*models.BookFile, error) {
	var f models.BookFile
	if err := r.db.Where("book_id = ?", bookID).First(&f); err != nil {
		return nil, err
	}
	return &f, nil
}

// PurchaseCounts returns how many users own each book, in one grouped query (PRD E5: owners keep
// deleted books). Books nobody bought are absent from the map, i.e. 0.
func (r *Repo) PurchaseCounts(bookIDs []string) (map[string]int, error) {
	counts := make(map[string]int, len(bookIDs))
	if len(bookIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		BookID string
		Owners int
	}
	if err := r.db.RawSql(`SELECT book_id, COUNT(*) AS owners FROM purchases WHERE book_id IN ? GROUP BY book_id`,
		&rows, bookIDs); err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.BookID] = row.Owners
	}
	return counts, nil
}
