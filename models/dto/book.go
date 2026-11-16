package dto

import "time"

// Upload is a file received from a multipart form.
type Upload struct {
	FileName string
	Content  []byte
}

// BookInput is the admin create/update form. Numbers stay strings so the service can report
// INVALID_AGE_RANGE vs VALIDATION_ERROR precisely (e.g. "1.5" is not a whole number).
type BookInput struct {
	Title, Author, Description string
	AgeType, AgeMin, AgeMax    string
	Price, Status              string
	Cover, Epub                *Upload // nil on update = keep the stored file
}

type AdminBook struct {
	ID            string    `json:"id"`
	Title         string    `json:"title"`
	Author        string    `json:"author"`
	Description   string    `json:"description"`
	AgeType       string    `json:"age_type"`
	AgeMin        int       `json:"age_min"`
	AgeMax        *int      `json:"age_max"`
	AgeLabel      string    `json:"age_label"`
	Price         int       `json:"price"`
	Status        string    `json:"status"`
	CoverURL      string    `json:"cover_url"`
	EpubFileName  string    `json:"epub_file_name"`
	EpubSizeBytes int       `json:"epub_size_bytes"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type AdminBookListQuery struct {
	Search string `query:"search" validate:"max=255"`
	Status string `query:"status" validate:"omitempty,oneof=draft published"`
}

func (q AdminBookListQuery) Validate() interface{} { return q }
