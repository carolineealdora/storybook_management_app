package order

import (
	"codebase/models"
	sql "codebase/pkg/orm"
)

type (
	Repository interface {
		PurchasedBookIDs(userID string) (map[string]bool, error)
		HasPurchase(userID, bookID string) (bool, error)
		ListMyBooks(userID string) ([]models.OwnedBook, error)
	}

	Repo struct {
		db sql.ORM
	}
)

func NewRepository(holder *models.SharedHolder) Repository {
	return &Repo{db: holder.Sql}
}

func (r *Repo) PurchasedBookIDs(userID string) (map[string]bool, error) {
	var ids []string
	if err := r.db.RawSql(`SELECT book_id::text FROM purchases WHERE user_id = ?`, &ids, userID); err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

func (r *Repo) HasPurchase(userID, bookID string) (bool, error) {
	var found []int
	if err := r.db.RawSql(`SELECT 1 FROM purchases WHERE user_id = ? AND book_id = ?`, &found, userID, bookID); err != nil {
		return false, err
	}
	return len(found) > 0, nil
}

// ListMyBooks returns purchased books newest purchase first, including books the admin later deleted (E5).
func (r *Repo) ListMyBooks(userID string) ([]models.OwnedBook, error) {
	var rows []models.OwnedBook
	err := r.db.RawSql(`SELECT b.id, b.title, b.author, b.description, b.age_type, b.age_min, b.age_max, b.price, b.status,
			b.epub_file_name, b.epub_size_bytes, b.created_at, b.updated_at, b.deleted_at, p.price_paid, p.purchased_at
		FROM purchases p JOIN books b ON b.id = p.book_id
		WHERE p.user_id = ? ORDER BY p.purchased_at DESC`, &rows, userID)
	return rows, err
}
