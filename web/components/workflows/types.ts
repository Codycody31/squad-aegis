export interface WorkflowTrigger {
  id: string;
  name: string;
  event_type: string;
  conditions?: any[];
  enabled: boolean;
}

export interface WorkflowStep {
  id: string;
  name: string;
  type: string;
  enabled: boolean;
  config: Record<string, any>;
  on_error?: any;
  next_steps?: string[];
}

export interface WorkflowDefinition {
  version: string;
  triggers: WorkflowTrigger[];
  variables: Record<string, any>;
  steps: WorkflowStep[];
  error_handling?: any;
}

export interface Workflow {
  id: string;
  server_id: string;
  name: string;
  description?: string;
  enabled: boolean;
  definition: WorkflowDefinition;
  created_by: string;
  created_at: string;
  updated_at: string;
  variables?: any[];
}

/** Form values shared by the create/edit dialog and the import flow. */
export interface WorkflowFormValues {
  name: string;
  description: string;
  enabled: boolean;
  definition: WorkflowDefinition;
}

export function defaultWorkflowDefinition(): WorkflowDefinition {
  return {
    version: "1.0",
    triggers: [],
    variables: {},
    steps: [],
    error_handling: {
      default_action: "stop",
      max_retries: 3,
      retry_delay_ms: 1000,
    },
  };
}

// Available event types for triggers
export const eventTypes = [
  { value: "RCON_CHAT_MESSAGE", label: "Chat Message" },
  { value: "RCON_PLAYER_WARNED", label: "Player Warned" },
  { value: "RCON_PLAYER_KICKED", label: "Player Kicked" },
  { value: "RCON_PLAYER_BANNED", label: "Player Banned" },
  { value: "RCON_SQUAD_CREATED", label: "Squad Created" },
  { value: "RCON_SERVER_INFO", label: "Server Info" },
  { value: "LOG_PLAYER_CONNECTED", label: "Player Connected" },
  { value: "LOG_JOIN_SUCCEEDED", label: "Player Join Succeeded" },
  { value: "LOG_PLAYER_DISCONNECTED", label: "Player Disconnected" },
  { value: "LOG_PLAYER_DIED", label: "Player Died" },
  { value: "LOG_PLAYER_WOUNDED", label: "Player Wounded" },
  { value: "LOG_ADMIN_BROADCAST", label: "Admin Broadcast" },
  { value: "LOG_GAME_EVENT_UNIFIED", label: "Game Event" },
];

// Available step types for workflow actions
export const stepTypes = [
  {
    value: "action",
    label: "Action",
    description: "Perform an action like RCON command or HTTP request",
  },
  {
    value: "condition",
    label: "Condition",
    description: "Check conditions and branch execution",
  },
  {
    value: "variable",
    label: "Variable",
    description: "Set or modify workflow variables",
  },
  {
    value: "delay",
    label: "Delay",
    description: "Wait for a specified amount of time",
  },
];

// Available action types for action steps
export const actionTypes = [
  {
    value: "rcon_command",
    label: "RCON Command",
    description: "Execute an RCON command",
  },
  {
    value: "admin_broadcast",
    label: "Admin Broadcast",
    description: "Send an admin broadcast message",
  },
  {
    value: "chat_message",
    label: "Chat Message",
    description: "Send a chat message to players",
  },
  {
    value: "kick_player",
    label: "Kick Player",
    description: "Kick a player from the server",
  },
  {
    value: "ban_player",
    label: "Ban Player",
    description: "Ban a player from the server",
  },
  {
    value: "ban_player_with_evidence",
    label: "Ban Player with Evidence",
    description: "Ban a player and link the triggering event as evidence",
  },
  {
    value: "warn_player",
    label: "Warn Player",
    description: "Send a warning to a player",
  },
  {
    value: "http_request",
    label: "HTTP Request",
    description: "Make an HTTP request to external API",
  },
  {
    value: "webhook",
    label: "Webhook",
    description: "Send data to webhook endpoint",
  },
  {
    value: "discord_message",
    label: "Discord Message",
    description: "Send a message to Discord channel",
  },
  {
    value: "log_message",
    label: "Log Message",
    description: "Write a message to the logs",
  },
  {
    value: "set_variable",
    label: "Set Variable",
    description: "Set a workflow variable",
  },
  {
    value: "lua_script",
    label: "Lua Script",
    description: "Execute custom Lua script with workflow context",
  },
];
