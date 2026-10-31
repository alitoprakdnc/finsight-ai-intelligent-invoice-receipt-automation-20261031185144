package model

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InvoiceStatus represents the processing state of an invoice.
type InvoiceStatus string

const (
	InvoiceStatusPending    InvoiceStatus = "pending"
	InvoiceStatusProcessing InvoiceStatus = "processing"
	InvoiceStatusCompleted  InvoiceStatus = "completed"
	InvoiceStatusFailed     InvoiceStatus = "failed"
)

// JSONB is a helper type that stores arbitrary JSON data in a PostgreSQL jsonb column.
// It implements the sql.Scanner and driver.Valuer interfaces so GORM can read/write it.
type JSONB json.RawMessage

// Value implements the driver.Valuer interface.
// It returns the JSON as a []byte to be stored in the database.
func (j JSONB) Value() (driver.Value, error) {
	if len(j) == 0 {
		// Store empty JSON object instead of NULL for consistency.
		return []byte("{}"), nil
	}
	return []byte(j), nil
}

// Scan implements the sql.Scanner interface.
// It converts a database value into a JSONB.
func (j *JSONB) Scan(src interface{}) error {
	if src == nil {
		*j = JSONB("{}")
		return nil
	}
	switch v := src.(type) {
	case []byte:
		*j = JSONB(v)
	case string:
		*j = JSONB(v)
	default:
		return json.Unmarshal([]byte{}, j)
	}
	return nil
}

// Invoice represents an uploaded invoice/receipt and its processing metadata.
type Invoice struct {
	// Primary key. Use UUID for global uniqueness across tenants.
	ID uuid.UUID `gorm:"type:uuid;default:uuid_generate_v4();primaryKey" json:"id"`

	// The user who uploaded the invoice.
	UserID uuid.UUID `gorm:"type:uuid;index;not null" json:"user_id"`

	// URL of the original file stored in the object store (e.g., MinIO/S3).
	FileURL string `gorm:"type:text;not null" json:"file_url"`

	// Raw OCR text extracted from the document.
	RawText string `gorm:"type:text" json:"raw_text,omitempty"`

	// Structured data extracted from the OCR (vendor, line items, totals, etc.).
	ExtractedJSON JSONB `gorm:"type:jsonb;default:'{}'" json:"extracted_json,omitempty"`

	// Current processing status.
	Status InvoiceStatus `gorm:"type:varchar(20);index;not null;default:'pending'" json:"status"`

	// Optional error message when status == failed.
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`

	// Timestamps.
	CreatedAt time.Time      `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"` // soft delete, omitted from JSON responses
}

// TableName overrides the default table name used by GORM.
func (Invoice) TableName() string {
	return "invoices"
}

// BeforeCreate is a GORM hook that ensures a UUID is set before inserting a new record.
func (i *Invoice) BeforeCreate(tx *gorm.DB) (err error) {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}

// SetExtractedData marshals a Go value into the ExtractedJSON column.
// It can be used by services that build a structured representation of the invoice.
func (i *Invoice) SetExtractedData(v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	i.ExtractedJSON = JSONB(b)
	return nil
}

// GetExtractedData unmarshals the ExtractedJSON column into the provided destination.
// The destination should be a pointer to the expected struct or map.
func (i *Invoice) GetExtractedData(dst interface{}) error {
	if len(i.ExtractedJSON) == 0 {
		return json.Unmarshal([]byte("{}"), dst)
	}
	return json.Unmarshal(i.ExtractedJSON, dst)
}