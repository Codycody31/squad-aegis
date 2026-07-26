export interface AuditLog {
  id: string;
  server_id: string;
  server_name: string;
  user_id: string;
  username: string;
  action: string;
  changes: any;
  timestamp: string;
}

// Get badge color based on action type
export function getActionBadgeColor(actionType: string): string {
  const actionColors: Record<string, string> = {
    "server:ban:create":
      "bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/20 dark:text-red-400",
    "server:ban:delete":
      "bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900/20 dark:text-green-400",
    "server:rcon:execute":
      "bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900/20 dark:text-purple-400",
    "server:rcon:command:kick":
      "bg-orange-50 text-orange-700 ring-orange-600/20 dark:bg-orange-900/20 dark:text-orange-400",
    "server:rcon:command:warn":
      "bg-yellow-50 text-yellow-700 ring-yellow-600/20 dark:bg-yellow-900/20 dark:text-yellow-400",
    "server:rcon:command:move":
      "bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900/20 dark:text-blue-400",
    "server:role:create":
      "bg-blue-50 text-blue-700 ring-blue-600/20 dark:bg-blue-900/20 dark:text-blue-400",
    "server:role:delete":
      "bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/20 dark:text-red-400",
  };

  return (
    actionColors[actionType] ||
    "bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/20 dark:text-gray-400"
  );
}

// Format action type for display
export function formatActionType(actionType: string): string {
  const actionTypeMap: Record<string, string> = {
    "server:ban:create": "Ban Created",
    "server:ban:delete": "Ban Removed",
    "server:rcon:execute": "RCON Command",
    "server:rcon:command:kick": "Player Kicked",
    "server:rcon:command:warn": "Player Warned",
    "server:rcon:command:move": "Player Moved",
    "server:role:create": "Role Created",
    "server:role:delete": "Role Deleted",
  };

  return (
    actionTypeMap[actionType] ||
    actionType
      .split(":")
      .pop()
      ?.split("_")
      .map((word) => word.charAt(0).toUpperCase() + word.slice(1))
      .join(" ") ||
    actionType
  );
}

// One-line summary of the changes payload
export function getActionDescription(log: AuditLog): string {
  if (!log.changes) return "";

  if (typeof log.changes === "string") {
    try {
      const parsed = JSON.parse(log.changes);
      if (parsed.description) return parsed.description;
      if (parsed.target) return `Target: ${parsed.target}`;
      if (parsed.command) return `Command: ${parsed.command}`;
      return parsed.toString();
    } catch (e) {
      return log.changes;
    }
  }

  if (typeof log.changes === "object") {
    if (log.changes.description) return log.changes.description;
    if (log.changes.target) return `Target: ${log.changes.target}`;
    if (log.changes.command) return `Command: ${log.changes.command}`;
    if (log.changes.reason) return `Reason: ${log.changes.reason}`;
    return "";
  }

  return String(log.changes);
}

export function formatJsonForDisplay(data: any): string {
  if (!data) return "No additional data";

  try {
    if (typeof data === "string") {
      const parsed = JSON.parse(data);
      return JSON.stringify(parsed, null, 2);
    }
    return JSON.stringify(data, null, 2);
  } catch (e) {
    return typeof data === "string" ? data : String(data);
  }
}

export function hasDetailedChanges(changes: any): boolean {
  if (!changes) return false;

  if (typeof changes === "string") {
    try {
      const parsed = JSON.parse(changes);
      return Object.keys(parsed).length > 0;
    } catch (e) {
      return changes.length > 0;
    }
  }

  if (typeof changes === "object") {
    return Object.keys(changes).length > 0;
  }

  return false;
}
