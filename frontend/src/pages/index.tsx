import React, { useState, useMemo } from "react";
import Layout from "@/components/Layout";
import InvoiceUpload from "@/components/InvoiceUpload";
import InvoiceTable from "@/components/InvoiceTable";
import ChartSummary from "@/components/ChartSummary";
import { useInvoices } from "@/hooks/useInvoices";
import { useAuth } from "@/hooks/useAuth";
import { Invoice } from "@/types/invoice";
import { Spinner } from "@/components/Spinner";

const DashboardPage: React.FC = () => {
  const { user } = useAuth();
  const { invoices, isLoading, isError, mutate } = useInvoices();
  const [isUploadOpen, setUploadOpen] = useState(false);

  // Compute aggregate stats from invoices
  const stats = useMemo(() => {
    if (!invoices) {
      return { total: 0, count: 0, pending: 0, processed: 0 };
    }
    const total = invoices.reduce((sum, inv) => {
      const amount = Number(inv.extracted?.totalAmount ?? 0);
      return sum + (isNaN(amount) ? 0 : amount);
    }, 0);
    const count = invoices.length;
    const pending = invoices.filter((i) => i.status === "pending").length;
    const processed = invoices.filter((i) => i.status === "processed").length;
    return { total, count, pending, processed };
  }, [invoices]);

  const handleUploadSuccess = async () => {
    setUploadOpen(false);
    // Refresh invoice list after a successful upload
    await mutate();
  };

  if (isLoading) {
    return (
      <Layout>
        <div className="flex items-center justify-center h-96">
          <Spinner />
        </div>
      </Layout>
    );
  }

  if (isError) {
    return (
      <Layout>
        <div className="p-6 text-center text-red-600">
          <p className="font-medium">Failed to load invoices.</p>
          <button
            onClick={() => mutate()}
            className="mt-4 px-4 py-2 bg-primary-600 text-white rounded hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-400"
          >
            Retry
          </button>
        </div>
      </Layout>
    );
  }

  return (
    <Layout>
      <div className="px-4 py-6 sm:px-6 lg:px-8">
        {/* Header */}
        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between mb-8">
          <h1 className="text-2xl font-semibold text-gray-900 dark:text-gray-100">
            Welcome, {user?.email ?? "User"}
          </h1>
          <button
            onClick={() => setUploadOpen(true)}
            className="mt-4 sm:mt-0 inline-flex items-center px-4 py-2 border border-transparent text-sm font-medium rounded-md shadow-sm text-white bg-primary-600 hover:bg-primary-700 focus:outline-none focus:ring-2 focus:ring-primary-400"
          >
            Upload Invoice
          </button>
        </div>

        {/* Stats */}
        <dl className="grid grid-cols-1 gap-5 sm:grid-cols-3 lg:grid-cols-4 mb-8">
          <div className="px-4 py-5 bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden sm:p-6">
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              Total Invoices
            </dt>
            <dd className="mt-1 text-3xl font-semibold text-gray-900 dark:text-gray-100">
              {stats.count}
            </dd>
          </div>

          <div className="px-4 py-5 bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden sm:p-6">
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              Processed
            </dt>
            <dd className="mt-1 text-3xl font-semibold text-green-600 dark:text-green-400">
              {stats.processed}
            </dd>
          </div>

          <div className="px-4 py-5 bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden sm:p-6">
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              Pending
            </dt>
            <dd className="mt-1 text-3xl font-semibold text-yellow-600 dark:text-yellow-400">
              {stats.pending}
            </dd>
          </div>

          <div className="px-4 py-5 bg-white dark:bg-gray-800 shadow rounded-lg overflow-hidden sm:p-6">
            <dt className="text-sm font-medium text-gray-500 dark:text-gray-400 truncate">
              Total Spend
            </dt>
            <dd className="mt-1 text-3xl font-semibold text-primary-600 dark:text-primary-400">
              ${stats.total.toFixed(2)}
            </dd>
          </div>
        </dl>

        {/* Charts & Recent Invoices */}
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Chart */}
          <div className="lg:col-span-2 bg-white dark:bg-gray-800 shadow rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
              Spend Overview
            </h2>
            <ChartSummary invoices={invoices as Invoice[]} />
          </div>

          {/* Recent Invoices */}
          <div className="bg-white dark:bg-gray-800 shadow rounded-lg p-6">
            <h2 className="text-lg font-medium text-gray-900 dark:text-gray-100 mb-4">
              Recent Invoices
            </h2>
            <InvoiceTable invoices={invoices?.slice(0, 5) ?? []} />
            <div className="mt-4 text-right">
              <a
                href="/invoices"
                className="text-primary-600 dark:text-primary-400 hover:underline font-medium"
              >
                View all &rarr;
              </a>
            </div>
          </div>
        </div>

        {/* Upload Modal */}
        {isUploadOpen && (
          <InvoiceUpload
            onClose={() => setUploadOpen(false)}
            onSuccess={handleUploadSuccess}
          />
        )}
      </div>
    </Layout>
  );
};

export default DashboardPage;