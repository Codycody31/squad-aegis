<template>
  <div ref="chartContainer" class="w-full h-full"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { DualAxes } from "@antv/g2plot";
import {
  buildTimeAxis,
  buildTimeMeta,
  chartAxisLabelStyle,
  chartAxisTitleStyle,
  chartGridLineStyle,
  formatChartTooltipTime,
  toChartTime,
} from "./time";

interface ConnectionValue {
  connections: number;
  disconnections: number;
  total_activity: number;
}

interface DataPoint {
  timestamp: string;
  value: ConnectionValue | number;
}

interface Props {
  data: DataPoint[];
  period: string;
}

const props = defineProps<Props>();

const chartContainer = ref<HTMLDivElement>();
let chart: DualAxes | null = null;

const getConnectionValues = (value: ConnectionValue | number) => {
  if (typeof value === "object" && value !== null) {
    return value;
  }

  return {
    connections: Number(value) || 0,
    disconnections: 0,
    total_activity: Number(value) || 0,
  };
};

const buildChartData = () =>
  props.data.map((point) => {
    const value = getConnectionValues(point.value);

    return {
      time: toChartTime(point.timestamp),
      connections: value.connections,
      disconnections: value.disconnections,
      totalActivity: value.total_activity,
    };
  });

const getChartOptions = (chartData: ReturnType<typeof buildChartData>) => {
  const timestamps = props.data.map((point) => point.timestamp);

  return {
    data: [chartData, chartData],
    xField: "time",
    yField: ["connections", "disconnections"],
    meta: {
      time: buildTimeMeta(timestamps),
    },
    geometryOptions: [
      {
        geometry: "line",
        color: "#10b981",
        lineStyle: {
          lineWidth: 2,
        },
        point: {
          size: 3,
          shape: "circle",
          style: {
            fill: "#10b981",
            stroke: "#fff",
            lineWidth: 1,
          },
        },
        smooth: false,
      },
      {
        geometry: "line",
        color: "#ef4444",
        lineStyle: {
          lineWidth: 2,
        },
        point: {
          size: 3,
          shape: "circle",
          style: {
            fill: "#ef4444",
            stroke: "#fff",
            lineWidth: 1,
          },
        },
        smooth: false,
      },
    ],
    xAxis: buildTimeAxis(timestamps),
    yAxis: {
      connections: {
        title: {
          text: "Connections",
          style: {
            ...chartAxisTitleStyle,
            fill: "#10b981",
          },
        },
        label: {
          style: {
            ...chartAxisLabelStyle,
            fill: "#10b981",
          },
        },
        grid: {
          line: {
            style: chartGridLineStyle,
          },
        },
        min: 0,
      },
      disconnections: {
        title: {
          text: "Disconnections",
          style: {
            ...chartAxisTitleStyle,
            fill: "#ef4444",
          },
        },
        label: {
          style: {
            ...chartAxisLabelStyle,
            fill: "#ef4444",
          },
        },
        min: 0,
      },
    },
    legend: {
      position: "top-right",
      itemName: {
        formatter: (text) => {
          return text === "connections" ? "Connections" : "Disconnections";
        },
      },
    },
    tooltip: {
      customContent: (title, items) => {
        if (!items || items.length === 0) return "";
        const time = items[0].data.time;

        let content = `<div style="padding: 10px;"><div style="font-weight: 600; margin-bottom: 6px;">${formatChartTooltipTime(time)}</div>`;
        items.forEach((item) => {
          const label = item.name === "connections" ? "Connections" : "Disconnections";
          content += `
            <div style="display: flex; justify-content: space-between; gap: 12px; padding-top: 4px;">
              <span style="color: ${item.color};">${label}</span>
              <span>${item.value}</span>
            </div>
          `;
        });
        content += "</div>";

        return content;
      },
    },
  };
};

// Create and configure the chart
const createChart = () => {
  if (!chartContainer.value || !props.data || props.data.length === 0) return;

  chart = new DualAxes(chartContainer.value, getChartOptions(buildChartData()));

  chart.render();
};

// Update chart when data changes
const updateChart = () => {
  if (!props.data?.length) {
    if (chart) {
      chart.destroy();
      chart = null;
    }
    return;
  }

  const chartData = buildChartData();

  if (!chart) {
    createChart();
    return;
  }

  chart.update(getChartOptions(chartData));
};

// Watch for data changes
watch(() => props.data, updateChart, { deep: true });
watch(() => props.period, () => {
  if (chart) {
    chart.destroy();
    chart = null;
  }
  createChart();
});

// Lifecycle
onMounted(() => {
  createChart();
});

onUnmounted(() => {
  if (chart) {
    chart.destroy();
  }
});
</script>
