import { CheckCircle, Clock, PlayCircle, XCircle } from "lucide-vue-next";

export interface WorkflowExecution {
  id: string;
  workflow_id: string;
  status: string;
  started_at: string;
  completed_at?: string;
  trigger_data?: any;
  error?: string;
}

export interface WorkflowExecutionLog {
  entry_id: string;
  execution_id: string;
  workflow_id: string;
  server_id: string;
  event_time: string;
  trigger_event_type: string;
  trigger_event_data?: any;
  status: string;
  step_name: string;
  step_type: string;
  step_order: number;
  step_status: string;
  step_input?: any;
  step_output?: any;
  step_duration_ms: number;
  variables?: any;
  metadata?: any;
  error?: string;
}

export interface WorkflowLogMessage {
  entry_id: string;
  execution_id: string;
  workflow_id: string;
  server_id: string;
  step_id: string;
  step_name: string;
  log_time: string;
  log_level: string;
  message: string;
  variables?: any;
  metadata?: any;
}

export interface ApiResponse<T = any> {
  message?: string;
  data: T;
  code: number;
}

export const getStatusIcon = (status: string) => {
  switch (status) {
    case "completed":
      return CheckCircle;
    case "failed":
    case "error":
      return XCircle;
    case "running":
    case "executing":
      return PlayCircle;
    default:
      return Clock;
  }
};

export const getStatusColor = (status: string) => {
  switch (status) {
    case "completed":
      return "text-green-600";
    case "failed":
    case "error":
      return "text-red-600";
    case "running":
    case "executing":
      return "text-blue-600";
    default:
      return "text-gray-600";
  }
};

export const getLogLevelColor = (level: string) => {
  switch (level.toUpperCase()) {
    case "DEBUG":
      return "text-gray-600 bg-gray-100";
    case "INFO":
      return "text-blue-600 bg-blue-100";
    case "WARN":
      return "text-yellow-600 bg-yellow-100";
    case "ERROR":
      return "text-red-600 bg-red-100";
    default:
      return "text-gray-600 bg-gray-100";
  }
};

export const formatDateTime = (dateStr: string) => {
  if (!dateStr) return "N/A";
  try {
    return new Date(dateStr).toLocaleString();
  } catch {
    return dateStr;
  }
};

export const formatDuration = (startStr: string, endStr?: string) => {
  if (!startStr) return "N/A";
  if (!endStr) return "In progress...";

  try {
    const start = new Date(startStr);
    const end = new Date(endStr);
    const diffMs = end.getTime() - start.getTime();

    if (diffMs < 1000) return `${diffMs}ms`;
    if (diffMs < 60000) return `${(diffMs / 1000).toFixed(1)}s`;
    if (diffMs < 3600000) return `${(diffMs / 60000).toFixed(1)}m`;
    return `${(diffMs / 3600000).toFixed(1)}h`;
  } catch {
    return "N/A";
  }
};

export const formatDurationMs = (durationMs: number) => {
  if (durationMs === 0) return "0ms";
  if (durationMs < 1000) return `${durationMs}ms`;
  if (durationMs < 60000) return `${(durationMs / 1000).toFixed(1)}s`;
  if (durationMs < 3600000) return `${(durationMs / 60000).toFixed(1)}m`;
  return `${(durationMs / 3600000).toFixed(1)}h`;
};

// JSON syntax highlighting function
export const syntaxHighlight = (json: string) => {
  if (!json) return "";

  return json
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(
      /("(\\u[a-zA-Z0-9]{4}|\\[^u]|[^\\"])*"(\s*:)?|\b(true|false|null)\b|-?\d+(?:\.\d*)?(?:[eE][+\-]?\d+)?)/g,
      (match) => {
        let cls = "text-red-600"; // number
        if (/^"/.test(match)) {
          if (/:$/.test(match)) {
            cls = "text-blue-600 font-medium"; // key
          } else {
            cls = "text-green-600"; // string
          }
        } else if (/true|false/.test(match)) {
          cls = "text-purple-600"; // boolean
        } else if (/null/.test(match)) {
          cls = "text-gray-500"; // null
        }
        return `<span class="${cls}">${match}</span>`;
      }
    );
};
