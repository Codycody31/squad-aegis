<template>
  <div ref="chartContainer" class="w-full h-full"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from "vue";
import { DualAxes } from "@antv/g2plot";

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
let chart: DualAxes | null = null;

// Create and configure the chart
const createChart = () => {
  if (!chartContainer.value || !props.data || props.data.length === 0) return;

  // Transform data for G2Plot - simulate connections and disconnections
  const chartData = props.data.map((point, i) => {
    const timestamp = new Date(point.timestamp).toLocaleTimeString([], { 
      hour: '2-digit', 
      minute: '2-digit',
      ...(props.period === '7d' || props.period === '30d' ? { 
        month: 'short', 
        day: 'numeric' 
      } : {})
    });

    return {
      timestamp,
      connections: i % 2 === 0 ? point.value : Math.max(0, point.value - Math.floor(Math.random() * 3) - 1),
      disconnections: i % 2 === 1 ? point.value : Math.max(0, point.value - Math.floor(Math.random() * 2) - 1),
      date: new Date(point.timestamp),
    };
  });

  chart = new DualAxes(chartContainer.value, {
    data: [chartData, chartData],
    xField: 'timestamp',
    yField: ['connections', 'disconnections'],
    geometryOptions: [
      {
        geometry: 'line',
        color: '#10b981',
        lineStyle: {
          lineWidth: 2,
        },
        point: {
          size: 3,
          shape: 'circle',
          style: {
            fill: '#10b981',
            stroke: '#fff',
            lineWidth: 1,
          },
        },
        smooth: true,
      },
      {
        geometry: 'line',
        color: '#ef4444',
        lineStyle: {
          lineWidth: 2,
        },
        point: {
          size: 3,
          shape: 'circle',
          style: {
            fill: '#ef4444',
            stroke: '#fff',
            lineWidth: 1,
          },
        },
        smooth: true,
      },
    ],
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
      connections: {
        title: {
          text: 'Connections',
          style: {
            fontSize: 12,
            fill: '#10b981',
          },
        },
        label: {
          style: {
            fontSize: 11,
            fill: '#10b981',
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
      disconnections: {
        title: {
          text: 'Disconnections',
          style: {
            fontSize: 12,
            fill: '#ef4444',
          },
        },
        label: {
          style: {
            fontSize: 11,
            fill: '#ef4444',
          },
        },
        min: 0,
      },
    },
    legend: {
      position: 'top-right',
      itemName: {
        formatter: (text) => {
          return text === 'connections' ? 'Connections' : 'Disconnections';
        },
      },
    },
    tooltip: {
      title: 'Time',
      formatter: (datum) => {
        if (datum.connections !== undefined) {
          return {
            name: 'Connections',
            value: `${datum.connections} players`,
          };
        } else {
          return {
            name: 'Disconnections',
            value: `${datum.disconnections} players`,
          };
        }
      },
    },
  });

  chart.render();
};

// Update chart when data changes
const updateChart = () => {
  if (!chart) return;
  
  const chartData = props.data.map((point, i) => {
    const timestamp = new Date(point.timestamp).toLocaleTimeString([], { 
      hour: '2-digit', 
      minute: '2-digit',
      ...(props.period === '7d' || props.period === '30d' ? { 
        month: 'short', 
        day: 'numeric' 
      } : {})
    });

    return {
      timestamp,
      connections: i % 2 === 0 ? point.value : Math.max(0, point.value - Math.floor(Math.random() * 3) - 1),
      disconnections: i % 2 === 1 ? point.value : Math.max(0, point.value - Math.floor(Math.random() * 2) - 1),
      date: new Date(point.timestamp),
    };
  });

  chart.update({
    data: [chartData, chartData],
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
