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

// Create and configure the chart
const createChart = () => {
  if (!chartContainer.value || !props.data || props.data.length === 0) return;

  // Transform data for G2Plot with performance colors
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
    performance: getPerformanceStatus(point.value),
    color: point.value >= 40 ? '#059669' : point.value >= 25 ? '#d97706' : '#dc2626',
  }));

  // Calculate average TPS for color determination
  const avgTps = props.data.reduce((sum, d) => sum + d.value, 0) / props.data.length;
  const lineColor = getLineColor(avgTps);

  chart = new Line(chartContainer.value, {
    data: chartData,
    xField: 'timestamp',
    yField: 'value',
    smooth: true,
    color: lineColor,
    point: {
      size: 4,
      shape: 'circle',
      style: {
        fill: lineColor,
        stroke: '#fff',
        lineWidth: 2,
      },
    },
    lineStyle: {
      lineWidth: 3,
    },
    annotations: [
      // Background zones for performance levels
      {
        type: 'region',
        start: ['min', 0],
        end: ['max', 25],
        style: {
          fill: '#fecaca', // More vibrant red background for danger zone
          fillOpacity: 0.5,
        },
      },
      {
        type: 'region',
        start: ['min', 25],
        end: ['max', 40],
        style: {
          fill: '#fed7aa', // More vibrant orange background for warning zone
          fillOpacity: 0.5,
        },
      },
      {
        type: 'region',
        start: ['min', 40],
        end: ['max', 100],
        style: {
          fill: '#bbf7d0', // More vibrant green background for good zone
          fillOpacity: 0.5,
        },
      },
      // Threshold lines
      {
        type: 'line',
        start: ['min', 25],
        end: ['max', 25],
        style: {
          stroke: '#dc2626',
          lineWidth: 2,
          lineDash: [4, 4],
          opacity: 0.8,
        },
        text: {
          content: 'Danger Threshold (25 TPS)',
          position: 'start',
          offsetY: -10,
          style: {
            fontSize: 11,
            fill: '#dc2626',
            fontWeight: 'bold',
          },
        },
      },
      {
        type: 'line',
        start: ['min', 40],
        end: ['max', 40],
        style: {
          stroke: '#d97706',
          lineWidth: 2,
          lineDash: [4, 4],
          opacity: 0.8,
        },
        text: {
          content: 'Warning Threshold (40 TPS)',
          position: 'start',
          offsetY: -10,
          style: {
            fontSize: 11,
            fill: '#d97706',
            fontWeight: 'bold',
          },
        },
      },
      {
        type: 'line',
        start: ['min', 60],
        end: ['max', 60],
        style: {
          stroke: '#059669',
          lineWidth: 3,
          lineDash: [5, 5],
          opacity: 0.9,
        },
        text: {
          content: 'Target (60 TPS)',
          position: 'end',
          style: {
            fontSize: 12,
            fill: '#059669',
            fontWeight: 'bold',
          },
        },
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
      },
    },
    yAxis: {
      title: {
        text: 'TPS (Ticks Per Second)',
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
      max: Math.max(65, Math.max(...props.data.map(d => d.value)) + 5),
    },
    tooltip: {
      title: 'Time',
      formatter: (datum) => {
        const performance = getPerformanceStatus(datum.value);
        const color = datum.value >= 40 ? '#059669' : datum.value >= 25 ? '#d97706' : '#dc2626';
        return {
          name: 'Server TPS',
          value: `${datum.value.toFixed(2)} TPS`,
          marker: {
            color: color,
          },
        };
      },
      customContent: (title, items) => {
        if (!items || items.length === 0) return '';
        const item = items[0];
        const performance = getPerformanceStatus(item.data.value);
        const color = item.data.value >= 40 ? '#059669' : item.data.value >= 25 ? '#d97706' : '#dc2626';
        
        return `
          <div class="g2-tooltip">
            <div class="g2-tooltip-title">${title}</div>
            <div class="g2-tooltip-list">
              <div class="g2-tooltip-list-item">
                <span class="g2-tooltip-marker" style="background-color: ${color}"></span>
                <span class="g2-tooltip-name">Server TPS:</span>
                <span class="g2-tooltip-value">${item.data.value.toFixed(2)} TPS</span>
              </div>
              <div class="g2-tooltip-list-item">
                <span class="g2-tooltip-marker" style="background-color: ${color}"></span>
                <span class="g2-tooltip-name">Status:</span>
                <span class="g2-tooltip-value" style="color: ${color}; font-weight: bold;">${performance}</span>
              </div>
            </div>
          </div>
        `;
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
                stroke: lineColor,
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
    performance: getPerformanceStatus(point.value),
    color: point.value >= 40 ? '#059669' : point.value >= 25 ? '#d97706' : '#dc2626',
  }));

  const avgTps = props.data.reduce((sum, d) => sum + d.value, 0) / props.data.length;
  const lineColor = getLineColor(avgTps);

  chart.update({
    data: chartData,
    color: lineColor,
    point: {
      style: {
        fill: lineColor,
      },
    },
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
