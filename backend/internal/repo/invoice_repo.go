package repo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"

	"finSight/backend/internal/model"
	"finSight/backend/internal/logger"
)

// InvoiceRepo provides data‑access methods for invoices.
type InvoiceRepo struct {
	db     *sqlx.DB
	logger *zap.Logger
}

// NewInvoiceRepo creates a new InvoiceRepo.
func NewInvoiceRepo(db *sqlx.DB) *InvoiceRepo {
	return &InvoiceRepo{
		db:     db,
		logger: logger.New(),
	}
}

// CreateInvoice inserts a new invoice record and writes an audit entry.
// It returns the created invoice with its generated ID.
func (r *InvoiceRepo) CreateInvoice(ctx context.Context, inv *model.Invoice) (*model.Invoice, error) {
	const query = `
		INSERT INTO invoices (user_id, file_url, raw_text, extracted_json, status, created_at, updated_at)
		VALUES (:user_id, :file_url, :raw_text, :extracted_json, :status, :created_at, :updated_at)
		RETURNING id;
	`

	// Set timestamps
	now := time.Now().UTC()
	inv.CreatedAt = now
	inv.UpdatedAt = now

	// Ensure JSON column is stored as []byte
	if inv.ExtractedJSON == nil {
		inv.ExtractedJSON = json.RawMessage(`{}`)
	}

	// Use NamedQuery to get the generated ID
	var id int64
	if err := r.db.QueryRowxContext(ctx, query, inv).Scan(&id); err != nil {
		r.logger.Error("failed to insert invoice", zap.Error(err), zap.Int64("user_id", inv.UserID))
		return nil, fmt.Errorf("could not create invoice: %w", err)
	}
	inv.ID = id

	// Audit log
	if err := r.insertAuditLog(ctx, inv.UserID, "invoice_created", map[string]any{
		"invoice_id": inv.ID,
		"file_url":   inv.FileURL,
	}); err != nil {
		// Non‑fatal – log and continue
		r.logger.Warn("failed to write audit log for invoice creation", zap.Error(err), zap.Int64("invoice_id", inv.ID))
	}

	return inv, nil
}

// UpdateStatus changes the status of an invoice and records an audit entry.
func (r *InvoiceRepo) UpdateStatus(ctx context.Context, invoiceID int64, userID int64, newStatus model.InvoiceStatus) error {
	const query = `
		UPDATE invoices
		SET status = $1, updated_at = $2
		WHERE id = $3 AND user_id = $4;
	`

	res, err := r.db.ExecContext(ctx, query, newStatus, time.Now().UTC(), invoiceID, userID)
	if err != nil {
		r.logger.Error("failed to update invoice status", zap.Error(err), zap.Int64("invoice_id", invoiceID))
		return fmt.Errorf("could not update invoice status: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	if err := r.insertAuditLog(ctx, userID, "invoice_status_updated", map[string]any{
		"invoice_id": invoiceID,
		"new_status": newStatus,
	}); err != nil {
		r.logger.Warn("failed to write audit log for status update", zap.Error(err), zap.Int64("invoice_id", invoiceID))
	}
	return nil
}

// GetByID retrieves a single invoice belonging to the given user.
func (r *InvoiceRepo) GetByID(ctx context.Context, invoiceID int64, userID int64) (*model.Invoice, error) {
	const query = `
		SELECT id, user_id, file_url, raw_text, extracted_json, status, created_at, updated_at
		FROM invoices
		WHERE id = $1 AND user_id = $2;
	`

	var inv model.Invoice
	if err := r.db.GetContext(ctx, &inv, query, invoiceID, userID); err != nil {
		if err == sql.ErrNoRows {
			return nil, err
		}
		r.logger.Error("failed to fetch invoice", zap.Error(err), zap.Int64("invoice_id", invoiceID))
		return nil, fmt.Errorf("could not fetch invoice: %w", err)
	}
	return &inv, nil
}

// ListByUser returns a paginated list of invoices for a user.
// `page` is 1‑based; `pageSize` defines the number of records per page.
func (r *InvoiceRepo) ListByUser(ctx context.Context, userID int64, page int, pageSize int) ([]*model.Invoice, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	const listQuery = `
		SELECT id, user_id, file_url, raw_text, extracted_json, status, created_at, updated_at
		FROM invoices
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3;
	`

	var invoices []*model.Invoice
	if err := r.db.SelectContext(ctx, &invoices, listQuery, userID, pageSize, offset); err != nil {
		r.logger.Error("failed to list invoices", zap.Error(err), zap.Int64("user_id", userID))
		return nil, 0, fmt.Errorf("could not list invoices: %w", err)
	}

	// Total count for pagination UI
	const countQuery = `SELECT COUNT(*) FROM invoices WHERE user_id = $1;`
	var total int64
	if err := r.db.GetContext(ctx, &total, countQuery, userID); err != nil {
		r.logger.Error("failed to count invoices", zap.Error(err), zap.Int64("user_id", userID))
		// Not fatal for the list itself
		total = int64(len(invoices))
	}

	return invoices, total, nil
}

// Delete removes an invoice owned by the user and logs the action.
func (r *InvoiceRepo) Delete(ctx context.Context, invoiceID int64, userID int64) error {
	const query = `
		DELETE FROM invoices
		WHERE id = $1 AND user_id = $2;
	`

	res, err := r.db.ExecContext(ctx, query, invoiceID, userID)
	if err != nil {
		r.logger.Error("failed to delete invoice", zap.Error(err), zap.Int64("invoice_id", invoiceID))
		return fmt.Errorf("could not delete invoice: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}

	if err := r.insertAuditLog(ctx, userID, "invoice_deleted", map[string]any{
		"invoice_id": invoiceID,
	}); err != nil {
		r.logger.Warn("failed to write audit log for invoice deletion", zap.Error(err), zap.Int64("invoice_id", invoiceID))
	}
	return nil
}

// insertAuditLog writes a generic audit entry.
// The audit table is assumed to have columns: id (serial), user_id, action, payload (jsonb), created_at.
func (r *InvoiceRepo) insertAuditLog(ctx context.Context, userID int64, action string, payload map[string]any) error {
	const query = `
		INSERT INTO audit_logs (user_id, action, payload, created_at)
		VALUES ($1, $2, $3, $4);
	`

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal audit payload: %w", err)
	}

	_, err = r.db.ExecContext(ctx, query, userID, action, payloadBytes, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}
	return nil
}