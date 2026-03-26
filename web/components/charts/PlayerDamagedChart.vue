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

const buildChartData = () =>
  props.data.map((point) => ({
    time: toChartTime(point.timestamp),
    value: point.value,
  }));

const getChartOptions = (chartData: ReturnType<typeof buildChartData>) => {
  const timestamps = props.data.map((point) => point.timestamp);

  return {
    data: chartData,
    xField: "time",
    yField: "value",
    smooth: false,
    color: "#f59e0b",
    point: {
      size: 3,
      shape: "circle",
      style: {
        fill: "#f59e0b",
        stroke: "#fff",
        lineWidth: 1,
      },
    },
    lineStyle: {
      lineWidth: 2,
    },
    meta: {
      time: buildTimeMeta(timestamps),
    },
    xAxis: buildTimeAxis(timestamps),
    yAxis: {
      title: {
        text: "Player Damage Events",
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
    },
    tooltip: {
      customContent: (title, items) => {
        if (!items || items.length === 0) return "";
        const time = items[0].data.time;

        return `
          <div style="padding: 10px;">
            <div style="font-weight: 600; margin-bottom: 6px;">${formatChartTooltipTime(time)}</div>
            <div style="display: flex; justify-content: space-between; gap: 12px;">
              <span style="color: #f59e0b;">Player Damage Events</span>
              <span>${items[0].value}</span>
            </div>
          </div>
        `;
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
