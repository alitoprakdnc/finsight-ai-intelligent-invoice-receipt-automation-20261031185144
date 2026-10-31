import React, { useEffect, useState, ChangeEvent } from "react";
import useSWR, { mutate } from "swr";
import { useRouter } from "next/router";
import api from "../utils/api";
import Modal from "./Modal";

type Invoice = {
  id: string;
  file_url: string;
  extracted_json: Record<string, any>;
  status: string;
  created_at: string;
  updated_at: string;
};

type InvoiceDetailProps = {
  /** The invoice identifier to display */
  invoiceId: string;
  /** Optional callback after a successful save */
  onSaved?: (updated: Invoice) => void;
};

const fetcher = (url: string) => api.get(url);

const InvoiceDetail: React.FC<InvoiceDetailProps> = ({
  invoiceId,
  onSaved,
}) => {
  const { data, error, isValidating } = useSWR<Invoice>(
    `/api/invoices/${invoiceId}`,
    fetcher
  );
  const [edited, setEdited] = useState<Record<string, any> | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [showSuccess, setShowSuccess] = useState(false);
  const router = useRouter();

  // Initialise editable copy once data is loaded
  useEffect(() => {
    if (data?.extracted_json) {
      // Deep clone to avoid mutating the SWR cache directly
      setEdited(JSON.parse(JSON.stringify(data.extracted_json)));
    }
  }, [data]);

  const handleTopLevelChange = (
    e: ChangeEvent<HTMLInputElement | HTMLSelectElement>
  ) => {
    if (!edited) return;
    const { name, value, type, checked } = e.target;
    const parsedValue =
      type === "number"
        ? Number(value)
        : type === "checkbox"
        ? checked
        : value;
    setEdited((prev) => ({
      ...(prev ?? {}),
      [name]: parsedValue,
    }));
  };

  const handleLineItemChange = (
    index: number,
    field: string,
    value: string
  ) => {
    if (!edited || !Array.isArray(edited.line_items)) return;
    const updatedLineItems = edited.line_items.map((item: any, i: number) =>
      i === index ? { ...item, [field]: value } : item
    );
    setEdited({ ...edited, line_items: updatedLineItems });
  };

  const renderInputForValue = (
    key: string,
    value: any
  ): JSX.Element => {
    const commonProps = {
      name: key,
      id: `field-${key}`,
      className:
        "mt-1 block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm",
      onChange: handleTopLevelChange,
    };

    if (typeof value === "number") {
      return (
        <input
          type="number"
          defaultValue={value}
          {...commonProps}
        />
      );
    }
    if (typeof value === "string") {
      // Detect date strings (ISO format)
      const isDate = /^\d{4}-\d{2}-\d{2}/.test(value);
      return (
        <input
          type={isDate ? "date" : "text"}
          defaultValue={value}
          {...commonProps}
        />
      );
    }
    if (typeof value === "boolean") {
      return (
        <input
          type="checkbox"
          checked={value}
          {...commonProps}
        />
      );
    }
    // Fallback to text input for other types
    return (
      <input
        type="text"
        defaultValue={String(value)}
        {...commonProps}
      />
    );
  };

  const handleSave = async () => {
    if (!edited) return;
    setSaving(true);
    setSaveError(null);
    try {
      const updated = await api.patch(`/api/invoices/${invoiceId}`, {
        extracted_json: edited,
      });
      // Revalidate SWR cache
      mutate(`/api/invoices/${invoiceId}`, updated, false);
      setShowSuccess(true);
      if (onSaved) onSaved(updated as Invoice);
    } catch (err: any) {
      setSaveError(err?.message ?? "Failed to save corrections.");
    } finally {
      setSaving(false);
    }
  };

  if (error) {
    return (
      <div className="p-6 text-center text-danger">
        <p className="font-medium">Failed to load invoice.</p>
        <button
          onClick={() => router.reload()}
          className="mt-4 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-primary"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!data || isValidating || !edited) {
    return (
      <div className="flex justify-center items-center py-12">
        <svg
          className="animate-spin h-8 w-8 text-primary"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
          aria-label="Loading"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8v8H4z"
          />
        </svg>
      </div>
    );
  }

  const { file_url, extracted_json } = data;
  const { line_items, ...topLevel } = edited;

  return (
    <div className="p-6">
      {/* Success Modal */}
      <Modal
        isOpen={showSuccess}
        onClose={() => setShowSuccess(false)}
        title="Corrections Saved"
      >
        <p className="mt-2 text-sm text-gray-600">
          Your changes have been saved successfully.
        </p>
        <div className="mt-4 flex justify-end">
          <button
            onClick={() => setShowSuccess(false)}
            className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary hover:bg-primary-dark focus:outline-none focus:ring-2 focus:ring-primary"
          >
            Close
          </button>
        </div>
      </Modal>

      {/* Main Content */}
      <div className="grid gap-6 md:grid-cols-2">
        {/* Image Preview */}
        <div className="bg-white rounded-lg shadow p-4">
          <h2 className="text-lg font-medium text-gray-900 mb-4">
            Original Document
          </h2>
          {file_url ? (
            <img
              src={file_url}
              alt="Invoice document"
              className="w-full object-contain rounded border border-gray-200"
            />
          ) : (
            <p className="text-sm text-gray-500">No document available.</p>
          )}
        </div>

        {/* Editable Data */}
        <div className="bg-white rounded-lg shadow p-4 flex flex-col">
          <h2 className="text-lg font-medium text-gray-900 mb-4">
            Extracted Data
          </h2>

          {/* Top‑level fields */}
          <div className="grid gap-4 sm:grid-cols-2">
            {Object.entries(topLevel).map(([key, value]) => (
              <div key={key}>
                <label
                  htmlFor={`field-${key}`}
                  className="block text-sm font-medium text-gray-700"
                >
                  {key.replace(/_/g, " ")}
                </label>
                {renderInputForValue(key, value)}
              </div>
            ))}
          </div>

          {/* Line items table */}
          {Array.isArray(line_items) && line_items.length > 0 && (
            <div className="mt-6 overflow-x-auto">
              <h3 className="text-md font-medium text-gray-800 mb-2">
                Line Items
              </h3>
              <table className="min-w-full divide-y divide-gray-200 border">
                <thead className="bg-gray-50">
                  <tr>
                    {Object.keys(line_items[0] ?? {}).map((col) => (
                      <th
                        key={col}
                        scope="col"
                        className="px-3 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
                      >
                        {col.replace(/_/g, " ")}
                      </th>
                    ))}
                  </tr>
                </thead>
                <tbody className="bg-white divide-y divide-gray-200">
                  {line_items.map((item: any, idx: number) => (
                    <tr key={idx}>
                      {Object.entries(item).map(([field, val]) => (
                        <td key={field} className="px-3 py-2 whitespace-nowrap">
                          <input
                            type="text"
                            defaultValue={val}
                            className="w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
                            onChange={(e) =>
                              handleLineItemChange(idx, field, e.target.value)
                            }
                          />
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          {/* Save button & error */}
          {saveError && (
            <p className="mt-3 text-sm text-danger">{saveError}</p>
          )}
          <button
            type="button"
            onClick={handleSave}
            disabled={saving}
            className={`mt-6 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white ${
              saving
                ? "bg-primary/70 cursor-not-allowed"
                : "bg-primary hover:bg-primary-dark"
            } focus:outline-none focus:ring-2 focus:ring-primary`}
          >
            {saving ? (
              <svg
                className="animate-spin -ml-1 mr-2 h-5 w-5 text-white"
                xmlns="http://www.w3.org/2000/svg"
                fill="none"
                viewBox="0 0 24 24"
                aria-hidden="true"
              >
                <circle
                  className="opacity-25"
                  cx="12"
                  cy="12"
                  r="10"
                  stroke="currentColor"
                  strokeWidth="4"
                />
                <path
                  className="opacity-75"
                  fill="currentColor"
                  d="M4 12a8 8 0 018-8v8H4z"
                />
              </svg>
            ) : null}
            {saving ? "Saving…" : "Save Corrections"}
          </button>
        </div>
      </div>
    </div>
  );
};

export default InvoiceDetail;