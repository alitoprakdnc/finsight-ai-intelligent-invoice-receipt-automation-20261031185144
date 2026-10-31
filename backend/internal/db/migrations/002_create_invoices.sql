-- +goose Up
-- Create enum type for invoice processing status
CREATE TYPE invoice_status AS ENUM ('pending', 'processing', 'completed', 'failed');

-- Create the invoices table
CREATE TABLE invoices (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    file_url TEXT NOT NULL,
    raw_text TEXT,
    extracted_data JSONB,
    status invoice_status NOT NULL DEFAULT 'pending',
    error_message TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at TIMESTAMPTZ
);

-- Foreign key to users table
ALTER TABLE invoices
    ADD CONSTRAINT fk_invoices_user
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- Indexes for common query patterns
CREATE INDEX idx_invoices_user_id ON invoices(user_id);
CREATE INDEX idx_invoices_status ON invoices(status);
CREATE INDEX idx_invoices_created_at ON invoices(created_at);

-- +goose Down
DROP TABLE IF EXISTS invoices;
DROP TYPE IF EXISTS invoice_status;