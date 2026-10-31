import { useState, useCallback } from 'react';
import useSWR, { mutate as globalMutate } from 'swr';
import api from '@/utils/api';

/**
 * Types
 */
export type InvoiceStatus = 'pending' | 'processing' | 'completed' | 'failed';

export interface Invoice {
  id: string;
  userId: string;
  fileUrl: string;
  rawText?: string;
  extractedJson?: Record<string, unknown>;
  status: InvoiceStatus;
  createdAt: string; // ISO string
  updatedAt: string; // ISO string
}

export interface InvoiceListResponse {
  invoices: Invoice[];
  total: number;
  page: number;
  pageSize: number;
}

/**
 * Hook options – used to build the query string.
 */
export interface UseInvoicesOptions {
  /** Initial page number (1‑based) */
  initialPage?: number;
  /** Number of items per page */
  pageSize?: number;
  /** Filter by invoice status */
  status?: InvoiceStatus;
  /** ISO date string – start of created‑at range */
  startDate?: string;
  /** ISO date string – end of created‑at range */
  endDate?: string;
  /** Full‑text search query */
  query?: string;
}

/**
 * Return type of the hook.
 */
export interface UseInvoicesResult {
  invoices: Invoice[];
  total: number;
  page: number;
  pageSize: number;
  isLoading: boolean;
  isError: boolean;
  error?: Error;
  isValidating: boolean;
  /** Refresh the current list (re‑fetch) */
  refresh: () => Promise<void>;
  /** Change page – triggers a new fetch */
  setPage: (page: number) => void;
  /** Change page size – triggers a new fetch */
  setPageSize: (size: number) => void;
  /** Apply or clear filters – resets to page 1 */
  setFilters: (filters: Partial<UseInvoicesOptions>) => void;
  /** Upload a new invoice file */
  addInvoice: (file: File) => Promise<Invoice | null>;
  /** Delete an invoice by id */
  deleteInvoice: (id: string) => Promise<boolean>;
  /** Patch an invoice (e.g. manual correction) */
  updateInvoice: (id: string, updates: Partial<Invoice>) => Promise<Invoice | null>;
}

/**
 * SWR fetcher – thin wrapper around the central API helper.
 */
const fetcher = (url: string) => api.get(url).then((res) => res.data);

/**
 * useInvoices – fetches a paginated list of invoices for the current user.
 *
 * The hook internally tracks pagination & filter state, builds a query string,
 * and uses SWR for caching, revalidation and optimistic UI updates.
 *
 * All mutation helpers (`addInvoice`, `deleteInvoice`, `updateInvoice`) perform
 * optimistic updates on the cached list to keep the UI snappy while still
 * handling server errors gracefully.
 */
export default function useInvoices(
  opts: UseInvoicesOptions = {}
): UseInvoicesResult {
  // -------------------------------------------------------------------------
  // State: pagination + filters
  // -------------------------------------------------------------------------
  const [params, setParams] = useState<{
    page: number;
    pageSize: number;
    status?: InvoiceStatus;
    startDate?: string;
    endDate?: string;
    query?: string;
  }>({
    page: opts.initialPage ?? 1,
    pageSize: opts.pageSize ?? 20,
    status: opts.status,
    startDate: opts.startDate,
    endDate: opts.endDate,
    query: opts.query,
  });

  // -------------------------------------------------------------------------
  // Build query string – only include defined params
  // -------------------------------------------------------------------------
  const queryString = new URLSearchParams({
    page: params.page.toString(),
    pageSize: params.pageSize.toString(),
    ...(params.status ? { status: params.status } : {}),
    ...(params.startDate ? { startDate: params.startDate } : {}),
    ...(params.endDate ? { endDate: params.endDate } : {}),
    ...(params.query ? { q: params.query } : {}),
  }).toString();

  // -------------------------------------------------------------------------
  // SWR data fetching
  // -------------------------------------------------------------------------
  const {
    data,
    error,
    mutate,
    isValidating,
  } = useSWR<InvoiceListResponse>(`/api/invoices?${queryString}`, fetcher, {
    revalidateOnFocus: false,
    dedupingInterval: 60_000, // 1 min
  });

  const isLoading = !data && !error;

  // -------------------------------------------------------------------------
  // Pagination helpers
  // -------------------------------------------------------------------------
  const setPage = useCallback((page: number) => {
    setParams((p) => ({ ...p, page }));
  }, []);

  const setPageSize = useCallback((size: number) => {
    setParams((p) => ({ ...p, pageSize: size, page: 1 }));
  }, []);

  const setFilters = useCallback(
    (filters: Partial<UseInvoicesOptions>) => {
      setParams((p) => ({
        ...p,
        ...filters,
        page: 1, // reset to first page when filters change
      }));
    },
    []
  );

  // -------------------------------------------------------------------------
  // Mutations – optimistic updates + error handling
  // -------------------------------------------------------------------------
  const addInvoice = useCallback(async (file: File): Promise<Invoice | null> => {
    const formData = new FormData();
    formData.append('file', file);

    try {
      const res = await api.post<Invoice>('/api/invoices', formData, {
        headers: { 'Content-Type': 'multipart/form-data' },
      });

      // Optimistically prepend the new invoice to the cached list
      mutate(
        (prev) => {
          if (!prev) return prev;
          const updated = {
            ...prev,
            invoices: [res.data, ...prev.invoices],
            total: prev.total + 1,
          };
          return updated;
        },
        false // do not revalidate immediately
      );

      return res.data;
    } catch (e) {
      console.error('Failed to upload invoice:', e);
      return null;
    }
  }, [mutate]);

  const deleteInvoice = useCallback(
    async (id: string): Promise<boolean> => {
      try {
        await api.delete(`/api/invoices/${id}`);

        // Optimistically remove from cache
        mutate(
          (prev) => {
            if (!prev) return prev;
            const filtered = prev.invoices.filter((inv) => inv.id !== id);
            return {
              ...prev,
              invoices: filtered,
              total: prev.total - 1,
            };
          },
          false
        );

        return true;
      } catch (e) {
        console.error('Failed to delete invoice:', e);
        return false;
      }
    },
    [mutate]
  );

  const updateInvoice = useCallback(
    async (
      id: string,
      updates: Partial<Invoice>
    ): Promise<Invoice | null> => {
      try {
        const res = await api.patch<Invoice>(`/api/invoices/${id}`, updates);

        // Optimistically merge updates into cached list
        mutate(
          (prev) => {
            if (!prev) return prev;
            const merged = prev.invoices.map((inv) =>
              inv.id === id ? { ...inv, ...res.data } : inv
            );
            return { ...prev, invoices: merged };
          },
          false
        );

        return res.data;
      } catch (e) {
        console.error('Failed to update invoice:', e);
        return null;
      }
    },
    [mutate]
  );

  const refresh = useCallback(async () => {
    await mutate();
  }, [mutate]);

  // -------------------------------------------------------------------------
  // Return shape
  // -------------------------------------------------------------------------
  return {
    invoices: data?.invoices ?? [],
    total: data?.total ?? 0,
    page: data?.page ?? params.page,
    pageSize: data?.pageSize ?? params.pageSize,
    isLoading,
    isError: !!error,
    error,
    isValidating,
    refresh,
    setPage,
    setPageSize,
    setFilters,
    addInvoice,
    deleteInvoice,
    updateInvoice,
  };
}