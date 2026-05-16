import type { SetValue, UploadStats } from "@/types";
import { Dropdown } from "@heroui/react";
import { Button } from "@heroui/react";
import type { ApexOptions } from "apexcharts";
import { memo, useMemo } from "react";
import ReactApexChart from "react-apexcharts";

const options: ApexOptions = {
  chart: {
    fontFamily: "Rubik, sans-serif",
    height: 250,
    sparkline: {
      enabled: false,
    },
    toolbar: {
      show: false,
    },
    type: "area",
    zoom: {
      enabled: false,
    },
  },
  colors: ["oklch(var(--color-accent))"],
  dataLabels: {
    enabled: false,
  },
  fill: {
    gradient: {
      opacityFrom: 0.45,
      opacityTo: 0.05,
      shadeIntensity: 1,
      stops: [20, 100, 100, 100],
    },
    type: "gradient",
  },
  grid: {
    borderColor: "var(--color-border)",
    padding: {
      bottom: 0,
      left: 20,
      right: 20,
    },
    show: true,
    strokeDashArray: 4,
  },
  legend: {
    show: false,
  },
  markers: {
    colors: ["oklch(var(--color-accent))"],
    hover: {
      size: 7,
    },
    size: 5,
    strokeColors: "var(--color-surface)",
    strokeWidth: 2,
  },
  stroke: {
    curve: "smooth",
    width: 3,
  },
  tooltip: {
    style: {
      fontFamily: "Rubik, sans-serif",
      fontSize: "12px",
    },
    theme: "dark",
    x: {
      show: true,
    },
    y: {
      formatter: (val) => `${val.toFixed(2)} GB`,
    },
  },
  xaxis: {
    axisBorder: {
      show: false,
    },
    axisTicks: {
      show: false,
    },
    labels: {
      style: {
        colors: "var(--color-muted)",
        fontSize: "12px",
        fontWeight: 500,
      },
    },
    type: "category",
  },
  yaxis: {
    labels: {
      style: {
        colors: "var(--color-muted)",
        fontSize: "12px",
        fontWeight: 500,
      },
    },
  },
};

function getChartData(stats: UploadStats[]): ApexOptions {
  const categories = stats.map((stat) => stat.uploadDate);
  const data = stats.map((stat) => stat.totalUploaded);
  return {
    ...options,
    series: [
      {
        data,
        name: "Uploaded",
      },
    ],
    xaxis: {
      ...options.xaxis,
      categories,
    },
  };
}

interface UploadStatsChartProps {
  stats: UploadStats[];
  days: number;
  setDays: SetValue<number>;
}

const allowedDays = [7, 15, 30, 60];

export const UploadStatsChart = memo(
  ({ stats, days, setDays }: UploadStatsChartProps) => {
    const chartOptions = useMemo(() => getChartData(stats), [stats]);

    return (
      <div className="w-full">
        <div className="flex justify-end mb-2">
          <Dropdown>
            <Button
              variant="secondary"
              className="rounded-xl px-4 py-2 font-medium bg-accent-soft text-accent-soft-foreground"
            >{`${days} Days`}</Button>
            <Dropdown.Popover className="min-w-32">
              <Dropdown.Menu>
                {allowedDays.map((day) => (
                  <Dropdown.Item key={day} onPress={() => setDays(day)}>
                    {`${day} Days`}
                  </Dropdown.Item>
                ))}
              </Dropdown.Menu>
            </Dropdown.Popover>
          </Dropdown>
        </div>
        <div className="min-h-[250px]">
          <ReactApexChart
            options={chartOptions}
            series={chartOptions.series}
            type="area"
            height={250}
          />
        </div>
      </div>
    );
  },
);
