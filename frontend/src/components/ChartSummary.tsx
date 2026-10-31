import React, { memo } from "react";
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
  LineChart,
  Line,
  Legend,
} from "recharts";

type VendorSpend = {
  /** Vendor name (displayed on X‑axis) */
  vendor: string;
  /** Total spend in the selected period (currency value) */
  amount: number;
};

type MonthlySpend = {
  /** ISO month string, e.g. "2024-09" */
  month: string;
  /** Total spend for the month */
  amount: number;
};

export interface ChartSummaryProps {
  /** Data for the vendor bar chart */
  vendorData: VendorSpend[];
  /** Data for the monthly line chart */
  monthlyData: MonthlySpend[];
}

/**
 * Custom tooltip for both charts – renders a simple, accessible table.
 */
const CustomTooltip: React.FC<{
  active?: boolean;
  payload?: any[];
  label?: string;
}> = ({ active, payload, label }) => {
  if (!active || !payload || payload.length === 0) {
    return null;
  }

  const value = payload[0].value;
  const name = payload[0].name ?? label;

  return (
    <div className="rounded bg-white p-2 shadow-md border border-gray-200">
      <p className="text-sm font-medium text-gray-700">{name}</p>
      <p className="text-sm text-gray-900">
        {new Intl.NumberFormat(undefined, {
          style: "currency",
          currency: "USD",
        }).format(value)}
      </p>
    </div>
  );
};

/**
 * ChartSummary component – renders a vendor spend bar chart and a monthly spend line chart.
 *
 * The component is fully responsive:
 * - On small screens the charts are stacked vertically.
 * - On medium+ screens they appear side‑by‑side.
 *
 * It follows the project's Tailwind design system and uses Recharts for rendering.
 */
const ChartSummary: React.FC<ChartSummaryProps> = ({
  vendorData,
  monthlyData,
}) => {
  // Guard against empty data sets – show a friendly placeholder.
  const hasVendorData = vendorData && vendorData.length > 0;
  const hasMonthlyData = monthlyData && monthlyData.length > 0;

  return (
    <section
      aria-label="Spend summary charts"
      className="grid grid-cols-1 gap-6 md:grid-cols-2"
    >
      {/* Vendor Bar Chart */}
      <div className="flex flex-col rounded-lg bg-white p-4 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold text-gray-900">
          Spend by Vendor
        </h2>
        {hasVendorData ? (
          <ResponsiveContainer width="100%" height={300}>
            <BarChart
              data={vendorData}
              margin={{ top: 5, right: 20, left: 0, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray="3 3" vertical={false} />
              <XAxis
                dataKey="vendor"
                tick={{ fontSize: 12 }}
                interval={0}
                angle={-30}
                dy={10}
                height={60}
              />
              <YAxis
                tickFormatter={(value) =>
                  new Intl.NumberFormat(undefined, {
                    notation: "compact",
                    compactDisplay: "short",
                  }).format(value)
                }
                tick={{ fontSize: 12 }}
              />
              <Tooltip content={<CustomTooltip />} />
              <Bar
                dataKey="amount"
                name="Spend"
                fill="#0F62FE"
                radius={[4, 4, 0, 0]}
                maxBarSize={80}
              />
            </BarChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-center text-gray-500">
            No vendor spend data available.
          </p>
        )}
      </div>

      {/* Monthly Line Chart */}
      <div className="flex flex-col rounded-lg bg-white p-4 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold text-gray-900">
          Monthly Spend Trend
        </h2>
        {hasMonthlyData ? (
          <ResponsiveContainer width="100%" height={300}>
            <LineChart
              data={monthlyData}
              margin={{ top: 5, right: 20, left: 0, bottom: 5 }}
            >
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis
                dataKey="month"
                tickFormatter={(m) => {
                  const date = new Date(`${m}-01`);
                  return date.toLocaleString(undefined, {
                    month: "short",
                    year: "numeric",
                  });
                }}
                tick={{ fontSize: 12 }}
                interval="preserveStartEnd"
              />
              <YAxis
                tickFormatter={(value) =>
                  new Intl.NumberFormat(undefined, {
                    notation: "compact",
                    compactDisplay: "short",
                  }).format(value)
                }
                tick={{ fontSize: 12 }}
              />
              <Tooltip content={<CustomTooltip />} />
              <Legend verticalAlign="top" height={36} />
              <Line
                type="monotone"
                dataKey="amount"
                name="Spend"
                stroke="#0F62FE"
                strokeWidth={2}
                dot={{ r: 3 }}
                activeDot={{ r: 5 }}
              />
            </LineChart>
          </ResponsiveContainer>
        ) : (
          <p className="text-center text-gray-500">
            No monthly spend data available.
          </p>
        )}
      </div>
    </section>
  );
};

export default memo(ChartSummary);