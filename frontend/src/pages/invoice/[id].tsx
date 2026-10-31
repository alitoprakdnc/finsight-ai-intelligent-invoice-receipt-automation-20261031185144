import React, { useCallback } from "react";
import { useRouter } from "next/router";
import useSWR from "swr";
import Layout from "../../components/Layout";
import InvoiceDetail from "../../components/InvoiceDetail";
import { api } from "../../utils/api";

interface Invoice {
  id: string;
  file_url: string;
  extracted_json: Record<string, unknown>;
  raw_text?: string;
  status: string;
  created_at: string;
  updated_at: string;
}

/**
 * Generic fetcher that uses the project's `api` wrapper.
 * It throws an error when the response is not ok so that SWR can
 * surface the failure.
 */
const fetcher = async (url: string) => {
  const res = await api(url);
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.message || "Failed to load data");
  }
  return res.json();
};

const InvoicePage: React.FC = () => {
  const router = useRouter();
  const { id } = router.query;

  const {
    data: invoice,
    error,
    mutate,
    isValidating,
  } = useSWR<Invoice>(id ? `/api/invoices/${id}` : null, fetcher);

  const handleSave = useCallback(
    async (updated: Partial<Invoice>) => {
      if (!id) return;
      try {
        const res = await api(`/api/invoices/${id}`, {
          method: "PUT",
          body: JSON.stringify(updated),
        });
        if (!res.ok) {
          const err = await res.json().catch(() => ({}));
          throw new Error(err.message || "Failed to save changes");
        }
        // Re‑validate the invoice data after a successful update
        await mutate();
        // Optional: show a success toast/alert
        // eslint-disable-next-line no-alert
        alert("Invoice updated successfully");
      } catch (e) {
        // eslint-disable-next-line no-console
        console.error(e);
        // eslint-disable-next-line no-alert
        alert(`Error: ${(e as Error).message}`);
      }
    },
    [id, mutate]
  );

  if (error) {
    return (
      <Layout title="Invoice – Error">
        <div className="flex h-full items-center justify-center">
          <p className="text-red-600">
            Unable to load invoice: {(error as Error).message}
          </p>
        </div>
      </Layout>
    );
  }

  if (!invoice) {
    return (
      <Layout title="Invoice – Loading">
        <div className="flex h-full items-center justify-center">
          <svg
            className="animate-spin h-8 w-8 text-primary-600"
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
      </Layout>
    );
  }

  return (
    <Layout title={`Invoice #${invoice.id}`}>
      <div className="max-w-7xl mx-auto p-4 space-y-6">
        {/* Header */}
        <div className="flex items-center justify-between">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
            Invoice Details
          </h1>
          <button
            type="button"
            onClick={() => router.back()}
            className="inline-flex items-center rounded-md border border-gray-300 bg-white px-3 py-2 text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:bg-gray-800 dark:border-gray-600 dark:text-gray-200 dark:hover:bg-gray-700"
          >
            ← Back
          </button>
        </div>

        {/* Image preview */}
        <section className="rounded-lg bg-white p-4 shadow dark:bg-gray-800">
          <h2 className="mb-2 text-lg font-medium text-gray-800 dark:text-gray-200">
            Original Document
          </h2>
          <div className="flex justify-center rounded border border-gray-200 bg-gray-50 p-2 dark:border-gray-600 dark:bg-gray-900">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={invoice.file_url}
              alt={`Invoice ${invoice.id}`}
              className="max-w-full max-h-[500px] object-contain rounded"
            />
          </div>
        </section>

        {/* Extracted data & manual correction */}
        <section className="rounded-lg bg-white p-4 shadow dark:bg-gray-800">
          <h2 className="mb-4 text-lg font-medium text-gray-800 dark:text-gray-200">
            Extracted Data
          </h2>
          <InvoiceDetail
            invoice={invoice}
            onSave={handleSave}
            isSaving={isValidating}
          />
        </section>
      </div>
    </Layout>
  );
};

export default InvoicePage;