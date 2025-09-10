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
    color: '#f59e0b',
    lineStyle: {
      lineWidth: 2,
    },
    point: {
      size: 3,
      shape: 'circle',
      style: {
        fill: '#f59e0b',
        stroke: '#f59e0b',
        strokeWidth: 1,
      },
    },
    smooth: true,
    label: false,
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
        autoRotate: true,
        autoHide: true,
      },
    },
    yAxis: {
      title: {
        text: 'Chat Messages',
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
      min: 0,
    },
    tooltip: {
      title: 'Time',
      formatter: (datum) => {
        return {
          name: 'Chat Messages',
          value: `${datum.value} messages`,
        };
      },
    },
    interactions: [{ type: 'element-active' }],
    state: {
      active: {
        style: {
          opacity: 0.8,
          stroke: '#f59e0b',
          lineWidth: 2,
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
