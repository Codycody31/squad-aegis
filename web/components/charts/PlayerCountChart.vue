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

interface PlayerCountValue {
  player_count: number;
  public_queue: number;
  reserved_queue: number;
  total_queue: number;
}

interface DataPoint {
  timestamp: string;
  value: PlayerCountValue | number;
}

interface Props {
  data: DataPoint[];
  period: string;
}

const props = defineProps<Props>();

const chartContainer = ref<HTMLDivElement>();
let chart: Line | null = null;

const buildChartData = () => {
  const chartData: any[] = [];

  props.data.forEach((point) => {
    const time = toChartTime(point.timestamp);

    if (typeof point.value === "object" && point.value !== null) {
      chartData.push(
        {
          time,
          value: point.value.player_count,
          type: "Players",
        },
        {
          time,
          value: point.value.public_queue,
          type: "Public Queue",
        },
        {
          time,
          value: point.value.reserved_queue,
          type: "Reserved Queue",
        }
      );
    } else {
      chartData.push({
        time,
        value: point.value as number,
        type: "Players",
      });
    }
  });

  return chartData;
};

const getChartOptions = (chartData: any[]) => {
  const timestamps = props.data.map((point) => point.timestamp);

  return {
    data: chartData,
    xField: "time",
    yField: "value",
    seriesField: "type",
    smooth: false,
    color: ["#3b82f6", "#f59e0b", "#10b981"],
    point: {
      size: 3,
      shape: "circle",
      style: {
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
        text: "Players",
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
        let content = `<div style="padding: 10px;"><strong>${formatChartTooltipTime(time)}</strong></div>`;
        items.forEach((item) => {
          content += `
            <div style="padding: 5px 10px; display: flex; justify-content: space-between;">
              <span style="color: ${item.color};">${item.name}:</span>
              <span>${item.value}</span>
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
  if (!props.data?.length) return;

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
