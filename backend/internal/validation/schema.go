package validation

import (
	"fmt"
	"strings"
	"sync"

	"github.com/xeipuuv/gojsonschema"
)

// ValidationError aggregates all schema validation errors.
type ValidationError struct {
	Errors []string
}

// Error implements the error interface.
func (ve *ValidationError) Error() string {
	if len(ve.Errors) == 0 {
		return "validation error: no details"
	}
	return fmt.Sprintf("validation error: %s", strings.Join(ve.Errors, "; "))
}

// schemaCache holds compiled JSON schemas to avoid re‑parsing on every request.
type schemaCache struct {
	invoice *gojsonschema.Schema
	webhook *gojsonschema.Schema
	once    sync.Once
	err     error
}

var cache schemaCache

// initSchemas parses the JSON schema definitions and compiles them.
// It is executed once lazily the first time a validator is called.
func initSchemas() {
	cache.once.Do(func() {
		invoiceLoader := gojsonschema.NewStringLoader(invoiceSchema)
		webhookLoader := gojsonschema.NewStringLoader(webhookSchema)

		var invoiceSchemaCompiled, webhookSchemaCompiled *gojsonschema.Schema
		var err error

		invoiceSchemaCompiled, err = gojsonschema.NewSchema(invoiceLoader)
		if err != nil {
			cache.err = fmt.Errorf("failed to compile invoice schema: %w", err)
			return
		}
		webhookSchemaCompiled, err = gojsonschema.NewSchema(webhookLoader)
		if err != nil {
			cache.err = fmt.Errorf("failed to compile webhook schema: %w", err)
			return
		}
		cache.invoice = invoiceSchemaCompiled
		cache.webhook = webhookSchemaCompiled
	})
}

// ValidateInvoicePayload validates the supplied payload against the invoice JSON schema.
// The payload can be any Go value that is JSON‑marshallable (e.g., map[string]interface{} or struct).
func ValidateInvoicePayload(payload interface{}) error {
	initSchemas()
	if cache.err != nil {
		return fmt.Errorf("schema initialization error: %w", cache.err)
	}
	return validate(cache.invoice, payload)
}

// ValidateWebhookConfig validates the supplied payload against the webhook configuration JSON schema.
func ValidateWebhookConfig(payload interface{}) error {
	initSchemas()
	if cache.err != nil {
		return fmt.Errorf("schema initialization error: %w", cache.err)
	}
	return validate(cache.webhook, payload)
}

// validate runs the actual validation using a compiled schema.
func validate(compiled *gojsonschema.Schema, payload interface{}) error {
	documentLoader := gojsonschema.NewGoLoader(payload)

	result, err := compiled.Validate(documentLoader)
	if err != nil {
		return fmt.Errorf("failed to validate payload: %w", err)
	}
	if result.Valid() {
		return nil
	}

	ve := &ValidationError{
		Errors: make([]string, 0, len(result.Errors())),
	}
	for _, desc := range result.Errors() {
		ve.Errors = append(ve.Errors, desc.String())
	}
	return ve
}

// ---------------------------------------------------------------------------
// JSON schema definitions
// ---------------------------------------------------------------------------

// invoiceSchema defines the expected shape of an invoice upload request.
// The API expects a multipart upload, but the JSON payload that accompanies the
// upload (e.g., metadata) must conform to this schema.
const invoiceSchema = `
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "InvoiceMetadata",
  "type": "object",
  "required": ["file_name", "metadata"],
  "additionalProperties": false,
  "properties": {
    "file_name": {
      "type": "string",
      "minLength": 1,
      "description": "Original filename of the uploaded invoice."
    },
    "metadata": {
      "type": "object",
      "required": ["vendor", "invoice_number", "date", "total_amount", "currency"],
      "additionalProperties": false,
      "properties": {
        "vendor": {
          "type": "string",
          "minLength": 1,
          "description": "Name of the vendor issuing the invoice."
        },
        "invoice_number": {
          "type": "string",
          "minLength": 1,
          "description": "Vendor‑provided invoice identifier."
        },
        "date": {
          "type": "string",
          "format": "date",
          "description": "Invoice issue date (ISO 8601)."
        },
        "total_amount": {
          "type": "number",
          "minimum": 0,
          "description": "Total amount payable."
        },
        "currency": {
          "type": "string",
          "pattern": "^[A-Z]{3}$",
          "description": "Three‑letter ISO 4217 currency code."
        },
        "tax_id": {
          "type": "string",
          "minLength": 1,
          "description": "Optional tax identifier for the vendor."
        },
        "line_items": {
          "type": "array",
          "items": {
            "type": "object",
            "required": ["description", "quantity", "unit_price"],
            "additionalProperties": false,
            "properties": {
              "description": { "type": "string", "minLength": 1 },
              "quantity": { "type": "number", "minimum": 0 },
              "unit_price": { "type": "number", "minimum": 0 },
              "tax_rate": { "type": "number", "minimum": 0, "maximum": 100 }
            }
          }
        }
      }
    }
  }
}
`

// webhookSchema defines the shape of a webhook subscription configuration.
const webhookSchema = `
{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "title": "WebhookSubscription",
  "type": "object",
  "required": ["url", "secret", "events"],
  "additionalProperties": false,
  "properties": {
    "url": {
      "type": "string",
      "format": "uri",
      "description": "Destination endpoint that will receive POST requests."
    },
    "secret": {
      "type": "string",
      "minLength": 32,
      "description": "HMAC secret used to sign webhook payloads (minimum 32 characters)."
    },
    "events": {
      "type": "array",
      "minItems": 1,
      "uniqueItems": true,
      "items": {
        "type": "string",
        "enum": [
          "invoice.created",
          "invoice.updated",
          "invoice.deleted",
          "webhook.failed"
        ]
      },
      "description": "List of events that trigger the webhook."
    },
    "active": {
      "type": "boolean",
      "default": true,
      "description": "Whether the subscription is currently active."
    },
    "description": {
      "type": "string",
      "maxLength": 200,
      "description": "Optional human‑readable description for the subscription."
    }
  }
}
`