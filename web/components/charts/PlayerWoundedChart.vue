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
  chartPalette,
  chartTooltipDomStyles,
  readToken,
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

const buildChartData = () =>
  props.data.map((point) => ({
    time: toChartTime(point.timestamp),
    value: point.value,
  }));

const getChartOptions = (chartData: ReturnType<typeof buildChartData>) => {
  const timestamps = props.data.map((point) => point.timestamp);
  const [seriesColor] = chartPalette();

  return {
    data: chartData,
    xField: "time",
    yField: "value",
    smooth: false,
    color: seriesColor,
    point: {
      size: 3,
      shape: "circle",
      style: {
        fill: seriesColor,
        stroke: readToken("--background"),
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
        text: "Wounded Players",
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
    },
    tooltip: {
      domStyles: chartTooltipDomStyles(),
      customContent: (title, items) => {
        if (!items || items.length === 0) return "";
        const time = items[0].data.time;

        return `
          <div style="padding: 10px;">
            <div style="font-weight: 600; margin-bottom: 6px;">${formatChartTooltipTime(time)}</div>
            <div style="display: flex; justify-content: space-between; gap: 12px;">
              <span style="color: ${seriesColor};">Wounded Players</span>
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
