package order

import (
	"time"

	"codebase/models"
	sql "codebase/pkg/orm"

	"github.com/google/uuid"
)

type (
	Repository interface {
		PurchasedBookIDs(userID string) (map[string]bool, error)
		HasPurchase(userID, bookID string) (bool, error)
		ListMyBooks(userID string) ([]models.OwnedBook, error)

		CreateBookOrder(o *models.BookOrder) error
		GetBookOrder(id, userID string) (*models.BookOrder, error)
		MarkBookOrderPaid(o *models.BookOrder, at time.Time) error
		MarkBookOrderFailed(id string, at time.Time) error
		ListBookOrders(userID string) ([]models.BookOrderRow, error)
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

// CreateBookOrder returns sql.ErrDuplicate when the user already used this idempotency key (E8).
func (r *Repo) CreateBookOrder(o *models.BookOrder) error {
	if o.ID == "" {
		o.ID = uuid.NewString()
	}
	if err := r.db.Create(o); err != nil {
		if sql.IsUniqueViolation(err) {
			return sql.ErrDuplicate
		}
		return err
	}
	return nil
}

// GetBookOrder only finds the caller's own orders; another user's order is "not found".
func (r *Repo) GetBookOrder(id, userID string) (*models.BookOrder, error) {
	var o models.BookOrder
	if err := r.db.Where("id = ? AND user_id = ?", id, userID).First(&o); err != nil {
		return nil, err
	}
	return &o, nil
}

// MarkBookOrderPaid flips pending → paid and records the purchase in one transaction.
// sql.ErrStateChanged: the order was no longer pending (double tap). sql.ErrDuplicate: the book is already owned.
func (r *Repo) MarkBookOrderPaid(o *models.BookOrder, at time.Time) error {
	return sql.WithTx(r.db, func(tx sql.ORM) error {
		var ids []string
		if err := tx.RawSql(`UPDATE book_orders SET status = 'paid', paid_at = ?, updated_at = ? WHERE id = ? AND status = 'pending' RETURNING id`,
			&ids, at, at, o.ID); err != nil {
			return err
		}
		if len(ids) == 0 {
			return sql.ErrStateChanged
		}
		err := tx.Create(&models.Purchase{ID: uuid.NewString(), UserID: o.UserID, BookID: o.BookID, BookOrderID: o.ID, PricePaid: o.Price, PurchasedAt: at})
		if sql.IsUniqueViolation(err) {
			return sql.ErrDuplicate
		}
		return err
	})
}

func (r *Repo) MarkBookOrderFailed(id string, at time.Time) error {
	var ids []string
	if err := r.db.RawSql(`UPDATE book_orders SET status = 'failed', updated_at = ? WHERE id = ? AND status = 'pending' RETURNING id`,
		&ids, at, id); err != nil {
		return err
	}
	if len(ids) == 0 {
		return sql.ErrStateChanged
	}
	return nil
}

// ListBookOrders is Order History: book orders only, newest first, with the price stored on the order.
func (r *Repo) ListBookOrders(userID string) ([]models.BookOrderRow, error) {
	var rows []models.BookOrderRow
	err := r.db.RawSql(`SELECT o.*, b.title AS book_title FROM book_orders o JOIN books b ON b.id = o.book_id
		WHERE o.user_id = ? ORDER BY o.created_at DESC`, &rows, userID)
	return rows, err
}
