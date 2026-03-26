<template>
  <div ref="chartContainer" class="w-full h-full"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { Line } from "@antv/g2plot";
import {
  buildTimeAxis,
  buildTimeMeta,
  chartAxisLabelStyle,
  chartAxisTitleStyle,
  chartGridLineStyle,
  formatChartTooltipTime,
  toChartTime,
} from "./time";

interface DataPoint {
  timestamp: string;
  value: number;
}

interface Props {
  data: DataPoint[];
  period: string;
}

const props = defineProps<Props>();

const chartContainer = ref<HTMLDivElement>();
let chart: Line | null = null;

// Get line color based on TPS performance
const getLineColor = (avgTps: number) => {
  if (avgTps >= 40) return "#059669"; // Darker green for good performance (40+)
  if (avgTps >= 25) return "#d97706"; // Darker orange for warning (25-40)
  return "#dc2626"; // Darker red for danger (sub 25)
};

// Get performance status text
const getPerformanceStatus = (tps: number) => {
  if (tps >= 40) return "Good";
  if (tps >= 25) return "Warning";
  return "Danger";
};

const buildChartData = () =>
  props.data.map((point) => ({
    time: toChartTime(point.timestamp),
    value: point.value,
    performance: getPerformanceStatus(point.value),
    color:
      point.value >= 40 ? "#059669" : point.value >= 25 ? "#d97706" : "#dc2626",
  }));

const getChartOptions = (chartData: ReturnType<typeof buildChartData>) => {
  const timestamps = props.data.map((point) => point.timestamp);

  const avgTps =
    props.data.reduce((sum, d) => sum + d.value, 0) / props.data.length;
  const lineColor = getLineColor(avgTps);

  return {
    data: chartData,
    xField: "time",
    yField: "value",
    smooth: false,
    color: lineColor,
    point: {
      size: 4,
      shape: "circle",
      style: {
        fill: lineColor,
        stroke: "#fff",
        lineWidth: 2,
      },
    },
    lineStyle: {
      lineWidth: 3,
    },
    meta: {
      time: buildTimeMeta(timestamps),
    },
    annotations: [
      {
        type: "region",
        start: ["min", 0],
        end: ["max", 25],
        style: {
          fill: "#fecaca", // More vibrant red background for danger zone
          fillOpacity: 0.5,
        },
      },
      {
        type: "region",
        start: ["min", 25],
        end: ["max", 40],
        style: {
          fill: "#fed7aa", // More vibrant orange background for warning zone
          fillOpacity: 0.5,
        },
      },
      {
        type: "region",
        start: ["min", 40],
        end: ["max", 100],
        style: {
          fill: "#bbf7d0", // More vibrant green background for good zone
          fillOpacity: 0.5,
        },
      },
      // Threshold lines
      {
        type: "line",
        start: ["min", 25],
        end: ["max", 25],
        style: {
          stroke: "#dc2626",
          lineWidth: 2,
          lineDash: [4, 4],
          opacity: 0.8,
        },
        text: {
          content: "Danger Threshold (25 TPS)",
          position: "start",
          offsetY: -10,
          style: {
            fontSize: 11,
            fill: "#dc2626",
            fontWeight: "bold",
          },
        },
      },
      {
        type: "line",
        start: ["min", 40],
        end: ["max", 40],
        style: {
          stroke: "#d97706",
          lineWidth: 2,
          lineDash: [4, 4],
          opacity: 0.8,
        },
        text: {
          content: "Warning Threshold (40 TPS)",
          position: "start",
          offsetY: -10,
          style: {
            fontSize: 11,
            fill: "#d97706",
            fontWeight: "bold",
          },
        },
      },
      {
        type: "line",
        start: ["min", 60],
        end: ["max", 60],
        style: {
          stroke: "#059669",
          lineWidth: 3,
          lineDash: [5, 5],
          opacity: 0.9,
        },
        text: {
          content: "Target (60 TPS)",
          position: "end",
          style: {
            fontSize: 12,
            fill: "#059669",
            fontWeight: "bold",
          },
        },
      },
    ],
    xAxis: buildTimeAxis(timestamps),
    yAxis: {
      title: {
        text: "TPS (Ticks Per Second)",
        style: chartAxisTitleStyle,
      },
      label: {
        style: chartAxisLabelStyle,
      },
      grid: {
        line: {
          style: chartGridLineStyle,
        },
      },
      min: 0,
      max: Math.max(65, Math.max(...props.data.map((d) => d.value)) + 5),
    },
    tooltip: {
      customContent: (title, items) => {
        if (!items || items.length === 0) return "";
        const time = items[0].data.time;
        let content = `<div style="padding: 10px;"><strong>${formatChartTooltipTime(time)}</strong></div>`;
        items.forEach((item) => {
          const performance = getPerformanceStatus(Number(item.value));
          content += `
            <div style="padding: 5px 10px; display: flex; justify-content: space-between;">
              <span style="color: ${item.color};">TPS:</span>
              <span>${item.value} (${performance})</span>
            </div>
          `;
        });
        return content;
      },
    },
  };
};

// Create and configure the chart
const createChart = () => {
  if (!chartContainer.value || !props.data || props.data.length === 0) return;

  chart = new Line(chartContainer.value, getChartOptions(buildChartData()));

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
watch(
  () => props.period,
  () => {
    if (chart) {
      chart.destroy();
      chart = null;
    }
    createChart();
  }
);

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
