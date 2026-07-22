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
import {
  chartTooltipDomStyles,
  readToken,
  statusColors,
  useChartTheme,
} from "./theme";

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
  const colors = statusColors();

  if (avgTps >= 40) return colors.good; // Good performance (40+)
  if (avgTps >= 25) return colors.warn; // Warning (25-40)
  return colors.danger; // Danger (sub 25)
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
    color: getLineColor(point.value),
  }));

const getChartOptions = (chartData: ReturnType<typeof buildChartData>) => {
  const timestamps = props.data.map((point) => point.timestamp);
  const colors = statusColors();

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
        stroke: readToken("--background"),
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
          fill: colors.danger, // Red background for danger zone
          fillOpacity: 0.2,
        },
      },
      {
        type: "region",
        start: ["min", 25],
        end: ["max", 40],
        style: {
          fill: colors.warn, // Orange background for warning zone
          fillOpacity: 0.2,
        },
      },
      {
        type: "region",
        start: ["min", 40],
        end: ["max", 100],
        style: {
          fill: colors.good, // Green background for good zone
          fillOpacity: 0.2,
        },
      },
      // Threshold lines
      {
        type: "line",
        start: ["min", 25],
        end: ["max", 25],
        style: {
          stroke: colors.danger,
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
            fill: colors.danger,
            fontWeight: "bold",
          },
        },
      },
      {
        type: "line",
        start: ["min", 40],
        end: ["max", 40],
        style: {
          stroke: colors.warn,
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
            fill: colors.warn,
            fontWeight: "bold",
          },
        },
      },
      {
        type: "line",
        start: ["min", 60],
        end: ["max", 60],
        style: {
          stroke: colors.good,
          lineWidth: 3,
          lineDash: [5, 5],
          opacity: 0.9,
        },
        text: {
          content: "Target (60 TPS)",
          position: "end",
          style: {
            fontSize: 12,
            fill: colors.good,
            fontWeight: "bold",
          },
        },
      },
    ],
    xAxis: buildTimeAxis(timestamps),
    yAxis: {
      title: {
        text: "TPS (Ticks Per Second)",
        style: chartAxisTitleStyle(),
      },
      label: {
        style: chartAxisLabelStyle(),
      },
      grid: {
        line: {
          style: chartGridLineStyle(),
        },
      },
      min: 0,
      max: Math.max(65, Math.max(...props.data.map((d) => d.value)) + 5),
    },
    tooltip: {
      domStyles: chartTooltipDomStyles(),
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

// Destroy and recreate so options (and theme colors) are rebuilt
const rebuildChart = () => {
  if (chart) {
    chart.destroy();
    chart = null;
  }
  createChart();
};

// Watch for data changes
watch(() => props.data, updateChart, { deep: true });
watch(() => props.period, rebuildChart);

// Re-render with fresh colors when the theme changes
useChartTheme(rebuildChart);

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
