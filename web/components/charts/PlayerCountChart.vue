<template>
  <div ref="chartContainer" class="w-full h-full"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { Line } from "@antv/g2plot";

interface DataPoint {
  timestamp: string;
  value:
    | {
        player_count: number;
        public_queue: number;
        reserved_queue: number;
        total_queue: number;
      }
    | number; // Support both old and new format
}

interface Props {
  data: DataPoint[];
  period: string;
}

const props = defineProps<Props>();

const chartContainer = ref<HTMLDivElement>();
let chart: Line | null = null;

// Create and configure the chart
const createChart = () => {
  if (!chartContainer.value || !props.data || props.data.length === 0) return;

  // Transform data for G2Plot with multiple series
  const chartData: any[] = [];

  props.data.forEach((point) => {
    const timestamp = new Date(point.timestamp).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      ...(props.period === "7d" || props.period === "30d"
        ? {
            month: "short",
            day: "numeric",
          }
        : {}),
    });

    if (typeof point.value === "object" && point.value !== null) {
      // New format with queue data
      chartData.push(
        {
          timestamp,
          value: point.value.player_count,
          type: "Players",
          date: new Date(point.timestamp),
        },
        {
          timestamp,
          value: point.value.public_queue,
          type: "Public Queue",
          date: new Date(point.timestamp),
        },
        {
          timestamp,
          value: point.value.reserved_queue,
          type: "Reserved Queue",
          date: new Date(point.timestamp),
        }
      );
    } else {
      // Legacy format
      chartData.push({
        timestamp,
        value: point.value as number,
        type: "Players",
        date: new Date(point.timestamp),
      });
    }
  });

  chart = new Line(chartContainer.value, {
    data: chartData,
    xField: "timestamp",
    yField: "value",
    seriesField: "type",
    smooth: true,
    color: ["#3b82f6", "#f59e0b", "#10b981"],
    point: {
      size: 3,
      shape: "circle",
      style: {
        fill: "#3b82f6",
        stroke: "#fff",
        lineWidth: 1,
      },
    },
    lineStyle: {
      lineWidth: 2,
    },
    xAxis: {
      title: {
        text: "Time",
        style: {
          fontSize: 12,
          fill: "#666",
        },
      },
      label: {
        style: {
          fontSize: 11,
          fill: "#666",
        },
      },
    },
    yAxis: {
      title: {
        text: "Players",
        style: {
          fontSize: 12,
          fill: "#666",
        },
      },
      label: {
        style: {
          fontSize: 11,
          fill: "#666",
        },
      },
      grid: {
        line: {
          style: {
            stroke: "#e5e7eb",
            lineWidth: 1,
            lineDash: [4, 5],
          },
        },
      },
    },
    tooltip: {
      title: "Time",
      customContent: (title, items) => {
        if (!items || items.length === 0) return "";
        const date = items[0].data.date;
        let content = `<div style="padding: 10px;"><strong>${date.toLocaleString()}</strong></div>`;
        items.forEach((item) => {
          content += `
            <div style="padding: 5px 10px; display: flex; justify-content: space-between;">
              <span style="color: ${item.color};">${item.name}:</span>
              <span>${item.value} players</span>
            </div>
          `;
        });
        return content;
      },
    },
    theme: {
      geometries: {
        point: {
          circle: {
            active: {
              style: {
                r: 4,
                fillOpacity: 0.85,
                stroke: "#3b82f6",
                lineWidth: 2,
              },
            },
          },
        },
      },
    },
  });

  chart.render();
};

// Update chart when data changes
const updateChart = () => {
  if (!chart) return;

  const chartData = props.data.map((point) => ({
    timestamp: new Date(point.timestamp).toLocaleTimeString([], {
      hour: "2-digit",
      minute: "2-digit",
      ...(props.period === "7d" || props.period === "30d"
        ? {
            month: "short",
            day: "numeric",
          }
        : {}),
    }),
    value: point.value,
    date: new Date(point.timestamp),
  }));

  chart.update({
    data: chartData,
  });
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
