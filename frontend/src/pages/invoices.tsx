import React, { useState, useEffect, useMemo, useCallback } from 'react';
import { NextPage } from 'next';
import Layout from '@/components/Layout';
import InvoiceTable from '@/components/InvoiceTable';
import InvoiceUpload from '@/components/InvoiceUpload';
import Modal from '@/components/Modal';
import { useInvoices } from '@/hooks/useInvoices';
import { useAuth } from '@/hooks/useAuth';
import { api } from '@/utils/api';
import { toast } from '@/components/Toast';
import { format } from 'date-fns';
import { FiUpload, FiTrash2, FiRefreshCw } from 'react-icons/fi';

type StatusOption = {
  label: string;
  value: string;
};

const statusOptions: StatusOption[] = [
  { label: 'All', value: '' },
  { label: 'Pending', value: 'pending' },
  { label: 'Processed', value: 'processed' },
  { label: 'Failed', value: 'failed' },
];

const InvoicesPage: NextPage = () => {
  const { user } = useAuth();
  const [search, setSearch] = useState('');
  const [status, setStatus] = useState<string>('');
  const [dateFrom, setDateFrom] = useState<string>('');
  const [dateTo, setDateTo] = useState<string>('');
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [showUpload, setShowUpload] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false);

  // Build query params for the hook
  const query = useMemo(
    () => ({
      search: search.trim(),
      status,
      date_from: dateFrom,
      date_to: dateTo,
      page,
      page_size: pageSize,
    }),
    [search, status, dateFrom, dateTo, page, pageSize],
  );

  const {
    invoices,
    total,
    loading,
    error,
    mutate: refreshInvoices,
  } = useInvoices(query, { revalidateOnFocus: false });

  // Reset pagination when filters change
  useEffect(() => {
    setPage(1);
  }, [search, status, dateFrom, dateTo]);

  const handleSelectChange = useCallback((ids: string[]) => {
    setSelectedIds(ids);
  }, []);

  const handleBulkDelete = async () => {
    if (selectedIds.length === 0) return;
    setShowDeleteConfirm(false);
    try {
      await api.delete('/invoices', { data: { ids: selectedIds } });
      toast.success(`Deleted ${selectedIds.length} invoice(s)`);
      setSelectedIds([]);
      refreshInvoices();
    } catch (err) {
      console.error(err);
      toast.error('Failed to delete invoices. Please try again.');
    }
  };

  const handleRefresh = () => {
    refreshInvoices();
  };

  const columns = useMemo(
    () => [
      {
        Header: 'File',
        accessor: 'file_name',
        Cell: ({ value }: { value: string }) => (
          <div className="truncate" title={value}>
            {value}
          </div>
        ),
      },
      {
        Header: 'Vendor',
        accessor: 'extracted.vendor',
        Cell: ({ row }: any) => {
          const vendor = row.original.extracted?.vendor ?? '-';
          return <span>{vendor}</span>;
        },
      },
      {
        Header: 'Amount',
        accessor: 'extracted.total_amount',
        Cell: ({ row }: any) => {
          const amount = row.original.extracted?.total_amount;
          return amount ? (
            <span className="font-medium">
              {new Intl.NumberFormat('en-US', {
                style: 'currency',
                currency: row.original.extracted?.currency || 'USD',
              }).format(Number(amount))}
            </span>
          ) : (
            '-'
          );
        },
      },
      {
        Header: 'Status',
        accessor: 'status',
        Cell: ({ value }: { value: string }) => {
          const statusMap: Record<string, { label: string; color: string }> = {
            pending: { label: 'Pending', color: 'bg-yellow-100 text-yellow-800' },
            processed: { label: 'Processed', color: 'bg-green-100 text-green-800' },
            failed: { label: 'Failed', color: 'bg-red-100 text-red-800' },
          };
          const cfg = statusMap[value] || {
            label: value,
            color: 'bg-gray-100 text-gray-800',
          };
          return (
            <span
              className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium ${cfg.color}`}
            >
              {cfg.label}
            </span>
          );
        },
      },
      {
        Header: 'Uploaded',
        accessor: 'created_at',
        Cell: ({ value }: { value: string }) => {
          const date = new Date(value);
          return (
            <time dateTime={value} className="text-gray-500">
              {date.toLocaleDateString()} {date.toLocaleTimeString()}
            </time>
          );
        },
      },
    ],
    [],
  );

  // Pagination helpers
  const totalPages = Math.ceil(total / pageSize);
  const canPrev = page > 1;
  const canNext = page < totalPages;

  return (
    <Layout title="Invoices">
      <div className="flex flex-col h-full">
        {/* Header actions */}
        <div className="flex items-center justify-between mb-4">
          <h1 className="text-2xl font-semibold text-gray-900">Invoices</h1>
          <div className="flex items-center space-x-2">
            <button
              onClick={() => setShowUpload(true)}
              className="inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary hover:bg-primary/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary"
            >
              <FiUpload className="mr-2 h-4 w-4" />
              Upload
            </button>
            <button
              onClick={handleRefresh}
              className="p-2 rounded-md text-gray-600 hover:bg-gray-100 focus:outline-none focus:ring-2 focus:ring-primary"
              title="Refresh"
            >
              <FiRefreshCw className="h-5 w-5" />
            </button>
          </div>
        </div>

        {/* Filters */}
        <div className="grid grid-cols-1 md:grid-cols-4 gap-4 mb-6">
          <div>
            <label htmlFor="search" className="sr-only">
              Search invoices
            </label>
            <input
              id="search"
              type="text"
              placeholder="Search by vendor, file name..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            />
          </div>

          <div>
            <label htmlFor="status" className="sr-only">
              Status
            </label>
            <select
              id="status"
              value={status}
              onChange={(e) => setStatus(e.target.value)}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            >
              {statusOptions.map((opt) => (
                <option key={opt.value} value={opt.value}>
                  {opt.label}
                </option>
              ))}
            </select>
          </div>

          <div>
            <label htmlFor="dateFrom" className="sr-only">
              From date
            </label>
            <input
              id="dateFrom"
              type="date"
              value={dateFrom}
              onChange={(e) => setDateFrom(e.target.value)}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            />
          </div>

          <div>
            <label htmlFor="dateTo" className="sr-only">
              To date
            </label>
            <input
              id="dateTo"
              type="date"
              value={dateTo}
              onChange={(e) => setDateTo(e.target.value)}
              className="block w-full rounded-md border-gray-300 shadow-sm focus:border-primary focus:ring-primary sm:text-sm"
            />
          </div>
        </div>

        {/* Bulk actions toolbar */}
        {selectedIds.length > 0 && (
          <div className="flex items-center justify-between bg-gray-50 p-3 rounded-md mb-4">
            <span className="text-sm text-gray-700">
              {selectedIds.length} selected
            </span>
            <div className="flex space-x-2">
              <button
                onClick={() => setShowDeleteConfirm(true)}
                className="inline-flex items-center px-3 py-1.5 border border-transparent text-sm font-medium rounded-md text-white bg-danger hover:bg-danger/90 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-danger"
              >
                <FiTrash2 className="mr-1 h-4 w-4" />
                Delete
              </button>
            </div>
          </div>
        )}

        {/* Table */}
        <div className="flex-1 overflow-auto">
          <InvoiceTable
            columns={columns}
            data={invoices}
            loading={loading}
            error={error}
            pageSize={pageSize}
            selectedRowIds={selectedIds}
            onSelectionChange={handleSelectChange}
            emptyState={
              <div className="text-center py-12 text-gray-500">
                No invoices found. Try adjusting your filters or upload a new
                invoice.
              </div>
            }
          />
        </div>

        {/* Pagination */}
        <nav
          className="flex items-center justify-between py-4"
          aria-label="Pagination"
        >
          <div className="text-sm text-gray-700">
            Showing{' '}
            <span className="font-medium">
              {total === 0 ? 0 : (page - 1) * pageSize + 1}
            </span>{' '}
            to{' '}
            <span className="font-medium">
              {Math.min(page * pageSize, total)}
            </span>{' '}
            of <span className="font-medium">{total}</span> invoices
          </div>
          <div className="flex space-x-2">
            <button
              onClick={() => setPage(1)}
              disabled={!canPrev}
              className={`px-3 py-1 rounded-md border ${
                canPrev
                  ? 'border-gray-300 text-gray-700 hover:bg-gray-100'
                  : 'border-gray-200 text-gray-400 cursor-not-allowed'
              }`}
            >
              First
            </button>
            <button
              onClick={() => setPage((p) => Math.max(p - 1, 1))}
              disabled={!canPrev}
              className={`px-3 py-1 rounded-md border ${
                canPrev
                  ? 'border-gray-300 text-gray-700 hover:bg-gray-100'
                  : 'border-gray-200 text-gray-400 cursor-not-allowed'
              }`}
            >
              Previous
            </button>
            <span className="px-3 py-1 text-gray-700">
              Page {page} of {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(p + 1, totalPages))}
              disabled={!canNext}
              className={`px-3 py-1 rounded-md border ${
                canNext
                  ? 'border-gray-300 text-gray-700 hover:bg-gray-100'
                  : 'border-gray-200 text-gray-400 cursor-not-allowed'
              }`}
            >
              Next
            </button>
            <button
              onClick={() => setPage(totalPages)}
              disabled={!canNext}
              className={`px-3 py-1 rounded-md border ${
                canNext
                  ? 'border-gray-300 text-gray-700 hover:bg-gray-100'
                  : 'border-gray-200 text-gray-400 cursor-not-allowed'
              }`}
            >
              Last
            </button>
          </div>
        </nav>

        {/* Upload Modal */}
        {showUpload && (
          <Modal
            title="Upload Invoices"
            onClose={() => setShowUpload(false)}
            size="lg"
          >
            <InvoiceUpload
              onSuccess={() => {
                setShowUpload(false);
                toast.success('Invoice uploaded successfully');
                refreshInvoices();
              }}
            />
          </Modal>
        )}

        {/* Delete Confirmation Modal */}
        {showDeleteConfirm && (
          <Modal
            title="Confirm Deletion"
            onClose={() => setShowDeleteConfirm(false)}
            size="sm"
          >
            <div className="p-4">
              <p className="text-gray-800">
                Are you sure you want to delete{' '}
                <span className="font-medium">{selectedIds.length}</span>{' '}
                invoice(s)? This action cannot be undone.
              </p>
              <div className="mt-6 flex justify-end space-x-3">
                <button
                  onClick={() => setShowDeleteConfirm(false)}
                  className="px-4 py-2 rounded-md border border-gray-300 text-gray-700 hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-primary"
                >
                  Cancel
                </button>
                <button
                  onClick={handleBulkDelete}
                  className="px-4 py-2 rounded-md bg-danger text-white hover:bg-danger/90 focus:outline-none focus:ring-2 focus:ring-danger"
                >
                  Delete
                </button>
              </div>
            </div>
          </Modal>
        )}
      </div>
    </Layout>
  );
};

export default InvoicesPage;