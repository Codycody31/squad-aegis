type ChartTimeValue = string | number | Date;

export const chartAxisLabelStyle = {
  fontSize: 11,
  fill: "#666",
};

export const chartAxisTitleStyle = {
  fontSize: 12,
  fill: "#666",
};

export const chartGridLineStyle = {
  stroke: "#e5e7eb",
  lineWidth: 1,
  lineDash: [4, 5] as number[],
};

const shortTimeFormatter = new Intl.DateTimeFormat(undefined, {
  hour: "numeric",
  minute: "2-digit",
});

const dayTimeFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  hour: "numeric",
});

const dayFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
});

const tooltipFormatter = new Intl.DateTimeFormat(undefined, {
  month: "short",
  day: "numeric",
  year: "numeric",
  hour: "numeric",
  minute: "2-digit",
});

const toDate = (value: ChartTimeValue) => {
  if (value instanceof Date) {
    return value;
  }

  return new Date(value);
};

const getSpanMs = (timestamps: ChartTimeValue[]) => {
  if (timestamps.length < 2) {
    return 0;
  }

  const sortedTimes = timestamps
    .map((value) => toDate(value).getTime())
    .filter((value) => !Number.isNaN(value))
    .sort((a, b) => a - b);

  if (sortedTimes.length < 2) {
    return 0;
  }

  return sortedTimes[sortedTimes.length - 1] - sortedTimes[0];
};

export const toChartTime = (timestamp: string) => toDate(timestamp).toISOString();

export const formatChartAxisTime = (
  value: ChartTimeValue,
  timestamps: ChartTimeValue[]
) => {
  const date = toDate(value);

  if (Number.isNaN(date.getTime())) {
    return String(value);
  }

  const spanMs = getSpanMs(timestamps);

  if (spanMs <= 36 * 60 * 60 * 1000) {
    return shortTimeFormatter.format(date);
  }

  if (spanMs <= 14 * 24 * 60 * 60 * 1000) {
    return dayTimeFormatter.format(date);
  }

  return dayFormatter.format(date);
};

export const formatChartTooltipTime = (value: ChartTimeValue) => {
  const date = toDate(value);

  if (Number.isNaN(date.getTime())) {
    return String(value);
  }

  return tooltipFormatter.format(date);
};

export const getTimeTickCount = (timestamps: ChartTimeValue[]) => {
  const count = timestamps.length;

  if (count <= 1) {
    return 1;
  }

  const spanMs = getSpanMs(timestamps);

  if (spanMs <= 6 * 60 * 60 * 1000) {
    return Math.min(count, 6);
  }

  if (spanMs <= 24 * 60 * 60 * 1000) {
    return Math.min(count, 8);
  }

  if (spanMs <= 7 * 24 * 60 * 60 * 1000) {
    return Math.min(count, 7);
  }

  if (spanMs <= 31 * 24 * 60 * 60 * 1000) {
    return Math.min(count, 8);
  }

  return Math.min(count, 6);
};

export const buildTimeMeta = (timestamps: ChartTimeValue[]) => ({
  type: "timeCat" as const,
  tickCount: getTimeTickCount(timestamps),
});

export const buildTimeAxis = (timestamps: ChartTimeValue[]) => ({
  title: {
    text: "Time",
    style: chartAxisTitleStyle,
  },
  label: {
    style: chartAxisLabelStyle,
    autoRotate: false,
    autoHide: true,
    formatter: (value: string) => formatChartAxisTime(value, timestamps),
  },
});
