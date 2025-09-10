<template>
  <div ref="chartContainer" class="w-full h-full"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { Line } from "@antv/g2plot";

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

// Create and configure the chart
const createChart = () => {
  if (!chartContainer.value || !props.data || props.data.length === 0) return;

  // Transform data for G2Plot
  const chartData = props.data.map(point => ({
    timestamp: new Date(point.timestamp).toLocaleTimeString([], { 
      hour: '2-digit', 
      minute: '2-digit',
      ...(props.period === '7d' || props.period === '30d' ? { 
        month: 'short', 
        day: 'numeric' 
      } : {})
    }),
    value: point.value,
    date: new Date(point.timestamp),
  }));

  chart = new Line(chartContainer.value, {
    data: chartData,
    xField: 'timestamp',
    yField: 'value',
    smooth: true,
    color: '#6366f1',
    point: {
      size: 3,
      shape: 'circle',
      style: {
        fill: '#6366f1',
        stroke: '#fff',
        lineWidth: 1,
      },
    },
    lineStyle: {
      lineWidth: 2,
    },
    xAxis: {
      title: {
        text: 'Time',
        style: {
          fontSize: 12,
          fill: '#666',
        },
      },
      label: {
        style: {
          fontSize: 11,
          fill: '#666',
        },
      },
    },
    yAxis: {
      title: {
        text: 'Admin Broadcasts',
        style: {
          fontSize: 12,
          fill: '#666',
        },
      },
      label: {
        style: {
          fontSize: 11,
          fill: '#666',
        },
      },
      grid: {
        line: {
          style: {
            stroke: '#e5e7eb',
            lineWidth: 1,
            lineDash: [4, 5],
          },
        },
      },
    },
    tooltip: {
      title: 'Time',
      formatter: (datum) => {
        return {
          name: 'Admin Broadcasts',
          value: `${datum.value} broadcasts`,
        };
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
                stroke: '#6366f1',
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
  
  const chartData = props.data.map(point => ({
    timestamp: new Date(point.timestamp).toLocaleTimeString([], { 
      hour: '2-digit', 
      minute: '2-digit',
      ...(props.period === '7d' || props.period === '30d' ? { 
        month: 'short', 
        day: 'numeric' 
      } : {})
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
