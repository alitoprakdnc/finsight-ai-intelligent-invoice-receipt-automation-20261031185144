import React, { useMemo, useState, useEffect } from "react";
import {
  ColumnDef,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  getPaginationRowModel,
  SortingState,
  useReactTable,
} from "@tanstack/react-table";
import { format } from "date-fns";
import { HiOutlineEye, HiOutlinePencilAlt, HiOutlineTrash } from "react-icons/hi";

/* -------------------------------------------------------------------------- */
/* Types                                                                      */
/* -------------------------------------------------------------------------- */

export interface Invoice {
  /** UUID */
  id: string;
  /** Vendor name */
  vendor: string;
  /** Total amount (in the invoice currency) */
  amount: number;
  /** ISO‑8601 date string */
  date: string;
  /** Processing status */
  status: "pending" | "processed" | "failed";
}

/* -------------------------------------------------------------------------- */
/* Props                                                                      */
/* -------------------------------------------------------------------------- */

export interface InvoiceTableProps {
  /** Array of invoices to display */
  invoices: Invoice[];
  /** Show a loading indicator when true */
  loading?: boolean;
  /** Optional error message */
  error?: string | null;
  /** Callback when the user clicks the “view” action */
  onView?: (invoice: Invoice) => void;
  /** Callback when the user clicks the “edit” action */
  onEdit?: (invoice: Invoice) => void;
  /** Callback when the user clicks the “delete” action */
  onDelete?: (invoice: Invoice) => void;
  /** Page size options for the pagination selector */
  pageSizeOptions?: number[];
  /** Default page size */
  defaultPageSize?: number;
}

/* -------------------------------------------------------------------------- */
/* Helper components                                                          */
/* -------------------------------------------------------------------------- */

const StatusBadge: React.FC<{ status: Invoice["status"] }> = ({ status }) => {
  const colors = {
    pending: "bg-yellow-100 text-yellow-800",
    processed: "bg-green-100 text-green-800",
    failed: "bg-red-100 text-red-800",
  };
  const labels = {
    pending: "Pending",
    processed: "Processed",
    failed: "Failed",
  };
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ${colors[status]}`}
    >
      {labels[status]}
    </span>
  );
};

const ActionButtons: React.FC<{
  invoice: Invoice;
  onView?: (i: Invoice) => void;
  onEdit?: (i: Invoice) => void;
  onDelete?: (i: Invoice) => void;
}> = ({ invoice, onView, onEdit, onDelete }) => {
  return (
    <div className="flex items-center space-x-2">
      {onView && (
        <button
          onClick={() => onView(invoice)}
          aria-label="View invoice"
          className="p-1 text-gray-600 hover:text-gray-900"
        >
          <HiOutlineEye className="h-5 w-5" />
        </button>
      )}
      {onEdit && (
        <button
          onClick={() => onEdit(invoice)}
          aria-label="Edit invoice"
          className="p-1 text-gray-600 hover:text-gray-900"
        >
          <HiOutlinePencilAlt className="h-5 w-5" />
        </button>
      )}
      {onDelete && (
        <button
          onClick={() => onDelete(invoice)}
          aria-label="Delete invoice"
          className="p-1 text-gray-600 hover:text-red-600"
        >
          <HiOutlineTrash className="h-5 w-5" />
        </button>
      )}
    </div>
  );
};

/* -------------------------------------------------------------------------- */
/* Main component                                                             */
/* -------------------------------------------------------------------------- */

export const InvoiceTable: React.FC<InvoiceTableProps> = ({
  invoices,
  loading = false,
  error = null,
  onView,
  onEdit,
  onDelete,
  pageSizeOptions = [10, 20, 50],
  defaultPageSize = 10,
}) => {
  const [sorting, setSorting] = useState<SortingState>([]);
  const [pageSize, setPageSize] = useState<number>(defaultPageSize);
  const [pageIndex, setPageIndex] = useState<number>(0);

  // Reset pagination when the data set changes
  useEffect(() => {
    setPageIndex(0);
  }, [invoices]);

  const columns = useMemo<ColumnDef<Invoice, any>[]>(() => {
    const currencyFormatter = new Intl.NumberFormat(undefined, {
      style: "currency",
      currency: "USD",
    });

    return [
      {
        accessorKey: "vendor",
        header: () => <span className="font-medium">Vendor</span>,
        cell: (info) => info.getValue(),
        enableSorting: true,
      },
      {
        accessorKey: "amount",
        header: () => <span className="font-medium">Amount</span>,
        cell: (info) => currencyFormatter.format(info.getValue() as number),
        enableSorting: true,
      },
      {
        accessorKey: "date",
        header: () => <span className="font-medium">Date</span>,
        cell: (info) => format(new Date(info.getValue() as string), "PPP"),
        enableSorting: true,
      },
      {
        accessorKey: "status",
        header: () => <span className="font-medium">Status</span>,
        cell: (info) => <StatusBadge status={info.getValue() as Invoice["status"]} />,
        enableSorting: true,
      },
      {
        id: "actions",
        header: () => null,
        cell: (info) => (
          <ActionButtons
            invoice={info.row.original}
            onView={onView}
            onEdit={onEdit}
            onDelete={onDelete}
          />
        ),
        enableSorting: false,
        meta: {
          className: "text-right",
        },
      },
    ];
  }, [onView, onEdit, onDelete]);

  const table = useReactTable({
    data: invoices,
    columns,
    state: {
      sorting,
      pagination: { pageIndex, pageSize },
    },
    onSortingChange: setSorting,
    onPaginationChange: (updater) => {
      const newState = typeof updater === "function" ? updater(table.getState().pagination) : updater;
      setPageIndex(newState.pageIndex);
      setPageSize(newState.pageSize);
    },
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    debugTable: false,
  });

  // ------------------------------------------------------------------------
  // Render helpers
  // ------------------------------------------------------------------------

  const renderTable = () => (
    <div className="overflow-x-auto rounded-lg border border-gray-200 bg-white shadow-sm">
      <table className="min-w-full divide-y divide-gray-200">
        <thead className="bg-gray-50">
          {table.getHeaderGroups().map((headerGroup) => (
            <tr key={headerGroup.id}>
              {headerGroup.headers.map((header) => {
                const isSortable = header.column.getCanSort();
                const sortDirection = header.column.getIsSorted();
                return (
                  <th
                    key={header.id}
                    colSpan={header.colSpan}
                    className={`px-4 py-2 text-left text-xs font-medium text-gray-500 uppercase tracking-wider ${
                      isSortable ? "cursor-pointer select-none" : ""
                    } ${header.column.columnDef.meta?.className ?? ""}`}
                    onClick={isSortable ? header.column.getToggleSortingHandler() : undefined}
                  >
                    <div className="flex items-center space-x-1">
                      {flexRender(header.column.columnDef.header, header.getContext())}
                      {isSortable && (
                        <svg
                          className={`h-3 w-3 transition-transform ${
                            sortDirection === "asc"
                              ? "transform rotate-180"
                              : sortDirection === "desc"
                              ? "transform rotate-0"
                              : "opacity-0"
                          }`}
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        >
                          <polyline points="6 9 12 15 18 9" />
                        </svg>
                      )}
                    </div>
                  </th>
                );
              })}
            </tr>
          ))}
        </thead>
        <tbody className="bg-white divide-y divide-gray-200">
          {table.getRowModel().rows.map((row) => (
            <tr key={row.id} className="hover:bg-gray-50">
              {row.getVisibleCells().map((cell) => (
                <td
                  key={cell.id}
                  className={`px-4 py-2 text-sm text-gray-700 ${
                    cell.column.columnDef.meta?.className ?? ""
                  }`}
                >
                  {flexRender(cell.column.columnDef.cell, cell.getContext())}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  const renderPagination = () => {
    const { pageIndex, pageSize, pageCount, totalRows } = {
      pageIndex: table.getState().pagination.pageIndex,
      pageSize: table.getState().pagination.pageSize,
      pageCount: table.getPageCount(),
      totalRows: table.getRowCount(),
    };

    const start = pageIndex * pageSize + 1;
    const end = Math.min(start + pageSize - 1, totalRows);

    return (
      <div className="flex items-center justify-between py-3 px-4 bg-gray-50 border-t border-gray-200">
        {/* Page info */}
        <div className="text-sm text-gray-600">
          Showing <span className="font-medium">{start}</span> to{" "}
          <span className="font-medium">{end}</span> of{" "}
          <span className="font-medium">{totalRows}</span> invoices
        </div>

        {/* Controls */}
        <div className="flex items-center space-x-4">
          {/* Page size selector */}
          <label className="flex items-center space-x-2 text-sm text-gray-600">
            <span>Rows per page</span>
            <select
              value={pageSize}
              onChange={(e) => {
                setPageSize(Number(e.target.value));
                setPageIndex(0);
              }}
              className="rounded border-gray-300 text-sm focus:ring-2 focus:ring-primary-500 focus:border-primary-500"
            >
              {pageSizeOptions.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </label>

          {/* Navigation buttons */}
          <nav className="inline-flex -space-x-px rounded-md shadow-sm" aria-label="Pagination">
            <button
              onClick={() => setPageIndex(0)}
              disabled={pageIndex === 0}
              className={`relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-300 bg-white text-sm font-medium ${
                pageIndex === 0 ? "text-gray-300 cursor-not-allowed" : "text-gray-700 hover:bg-gray-50"
              }`}
            >
              <span className="sr-only">First</span>
              <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                <path d="M10.707 5.293a1 1 0 00-1.414 0L5.586 9l3.707 3.707a1 1 0 001.414-1.414L8.414 10H15a1 1 0 100-2H8.414l2.293-2.293a1 1 0 000-1.414z" />
              </svg>
            </button>

            <button
              onClick={() => setPageIndex((old) => Math.max(old - 1, 0))}
              disabled={pageIndex === 0}
              className={`relative inline-flex items-center px-2 py-2 border border-gray-300 bg-white text-sm font-medium ${
                pageIndex === 0 ? "text-gray-300 cursor-not-allowed" : "text-gray-700 hover:bg-gray-50"
              }`}
            >
              <span className="sr-only">Previous</span>
              <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                <path d="M12.707 14.707a1 1 0 01-1.414 0L7.586 11l3.707-3.707a1 1 0 011.414 1.414L10.414 11l2.293 2.293a1 1 0 010 1.414z" />
              </svg>
            </button>

            <button
              onClick={() => setPageIndex((old) => Math.min(old + 1, pageCount - 1))}
              disabled={pageIndex >= pageCount - 1}
              className={`relative inline-flex items-center px-2 py-2 border border-gray-300 bg-white text-sm font-medium ${
                pageIndex >= pageCount - 1
                  ? "text-gray-300 cursor-not-allowed"
                  : "text-gray-700 hover:bg-gray-50"
              }`}
            >
              <span className="sr-only">Next</span>
              <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                <path d="M7.293 5.293a1 1 0 011.414 0L12.414 9l-3.707 3.707a1 1 0 01-1.414-1.414L9.586 10 7.293 7.707a1 1 0 010-1.414z" />
              </svg>
            </button>

            <button
              onClick={() => setPageIndex(pageCount - 1)}
              disabled={pageIndex >= pageCount - 1}
              className={`relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-300 bg-white text-sm font-medium ${
                pageIndex >= pageCount - 1
                  ? "text-gray-300 cursor-not-allowed"
                  : "text-gray-700 hover:bg-gray-50"
              }`}
            >
              <span className="sr-only">Last</span>
              <svg className="h-5 w-5" viewBox="0 0 20 20" fill="currentColor">
                <path d="M9.293 14.707a1 1 0 001.414 0L14.414 11l-3.707-3.707a1 1 0 00-1.414 1.414L11.586 11l-2.293 2.293a1 1 0 000 1.414z" />
              </svg>
            </button>
          </nav>
        </div>
      </div>
    );
  };

  // ------------------------------------------------------------------------
  // Main render
  // ------------------------------------------------------------------------

  if (loading) {
    return (
      <div className="flex items-center justify-center py-12">
        <svg
          className="animate-spin h-8 w-8 text-primary-600"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8v8H4z"
          />
        </svg>
      </div>
    );
  }

  if (error) {
    return (
      <div className="rounded-md bg-red-50 p-4 text-sm text-red-700">
        <p>{error}</p>
      </div>
    );
  }

  if (invoices.length === 0) {
    return (
      <div className="rounded-md bg-gray-50 p-4 text-center text-sm text-gray-600">
        No invoices found.
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4">
      {renderTable()}
      {renderPagination()}
    </div>
  );
};

export default InvoiceTable;