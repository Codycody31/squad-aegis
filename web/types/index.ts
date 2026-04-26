export interface User {
  id: string;
  steam_id: string;
  name: string;
  username: string;
  super_admin: boolean;
  created_at: string;
  updated_at: string;
}

export interface AuditLog {
  id: string;
  server_id?: string;
  user_id?: string;
  action: string;
  changes: Record<string, any>;
  timestamp: string;
}

export interface Server {
  id: string;
  name: string;
  ip_address: string;
  game_port: number;
  rcon_ip_address: string | null;
  rcon_port: number;
  // Log & file access
  log_source_type: "local" | "sftp" | "ftp" | null;
  log_host: string | null;
  log_port: number | null;
  log_username: string | null;
  log_poll_frequency: number | null;
  log_read_from_start: boolean | null;
  squad_game_path: string | null;
  created_at: string;
  updated_at: string;
}

export interface ServerBan {
  id: string;
  server_id: string;
  admin_id?: string;
  steam_id: string;
  eos_id?: string;
  reason: string;
  permanent: boolean;
  expires_at?: string;
  created_at: string;
  updated_at: string;
}

export interface ServerAdmin {
  id: string;
  server_id: string;
  name: string;
  user_id: string;
  server_role_id: string;
  created_at: string;
}

export interface ServerRole {
  id: string;
  server_id: string;
  name: string;
  permissions: string;
  created_at: string;
}

export interface Player {
  playerId: number;
  eosId: string;
  eos_id?: string;
  epicId?: string;
  epic_id?: string;
  steam_id: string;
  name: string;
  teamId: number;
  squadId: number;
  isSquadLeader: boolean;
  role: string;
  sinceDisconnect: string;
}

export interface NavigationItem {
  heading?: string;
  title?: string;
  to?: {
    name: string;
    params?: Record<string, any>;
  };
  icon?: string;
  items?: NavigationItem[];
  isActive?: boolean;
  url?: string;
  permissions?: string[];
}

// Sudo/Superadmin Types

export interface StorageFile {
  path: string;
  size: number;
  size_readable: string;
  modified_time: string;
  is_dir: boolean;
  extension: string;
}

export interface StorageSummary {
  total_size: number;
  total_size_readable: string;
  total_files: number;
  files_by_type: Record<string, number>;
  size_by_type: Record<string, number>;
  storage_type: string;
  recent_files: StorageFile[];
}

export interface MetricsOverview {
  total_servers: number;
  active_servers: number;
  total_players: number;
  total_events: number;
  events_this_week: number;
  events_this_month: number;
  total_chat_messages: number;
  total_workflow_runs: number;
  storage_used: number;
  storage_used_readable: string;
}

export interface MetricsTimelinePoint {
  timestamp: string;
  value: number;
}

export interface MetricsTimeline {
  events_over_time: MetricsTimelinePoint[];
  chat_over_time: MetricsTimelinePoint[];
  players_over_time: MetricsTimelinePoint[];
}

export interface ServerActivity {
  server_id: string;
  server_name: string;
  total_events: number;
  chat_messages: number;
  unique_players: number;
  workflow_runs: number;
  avg_player_count: number;
  last_activity: string;
}

export interface TopServers {
  by_events: ServerActivity[];
  by_players: ServerActivity[];
  by_messages: ServerActivity[];
}

export interface SystemServiceHealth {
  status: string;
  latency: number;
  message: string;
  details?: Record<string, any>;
}

export interface SystemHealth {
  postgresql: SystemServiceHealth;
  clickhouse: SystemServiceHealth;
  valkey: SystemServiceHealth;
  storage: SystemServiceHealth;
  overall: string;
}

export interface SystemConfig {
  app: Record<string, any>;
  database: Record<string, any>;
  clickhouse: Record<string, any>;
  valkey: Record<string, any>;
  storage: Record<string, any>;
  log: Record<string, any>;
  plugins?: {
    native_enabled?: boolean;
    allow_unsafe_sideload?: boolean;
    max_upload_size?: number;
    trusted_signing_keys_set?: boolean;
  };
}

export interface GlobalAuditLog {
  id: string;
  server_id?: string;
  server_name?: string;
  user_id?: string;
  username?: string;
  action: string;
  changes?: Record<string, any>;
  timestamp: string;
}

export interface ActionCount {
  action: string;
  count: number;
}

export interface UserActionCount {
  user_id: string;
  username: string;
  count: number;
}

export interface GlobalAuditStats {
  total_logs: number;
  logs_today: number;
  logs_this_week: number;
  logs_this_month: number;
  top_actions: ActionCount[];
  top_users: UserActionCount[];
  recent_activity: GlobalAuditLog[];
}

export interface SessionInfo {
  id: string;
  user_id: string;
  username: string;
  token: string;
  created_at: string;
  expires_at?: string;
  last_seen: string;
  last_seen_ip: string;
  is_expired: boolean;
  time_remaining: string;
}

export interface UserSessionCount {
  user_id: string;
  username: string;
  session_count: number;
}

export interface SessionStats {
  total_sessions: number;
  active_sessions: number;
  expired_sessions: number;
  sessions_by_user: UserSessionCount[];
}

export interface DatabaseTableStats {
  table_name: string;
  row_count: number;
  total_size: string;
  data_size: string;
  index_size: string;
  last_analyzed?: string;
}

export interface PostgreSQLStats {
  database_name: string;
  database_size: string;
  table_count: number;
  total_connections: number;
  active_queries: number;
  cache_hit_ratio: number;
  tables: DatabaseTableStats[];
}

export interface PluginPackageManifest {
  plugin_id: string;
  name: string;
  description: string;
  version: string;
  official: boolean;
  license?: string;
  entry_symbol: string;
  targets: PluginPackageTarget[];
}

export interface PluginPackageTarget {
  min_host_api_version: number;
  required_capabilities?: string[];
  target_os: string;
  target_arch: string;
  sha256?: string;
  library_path: string;
}

export interface PluginPackage {
  plugin_id: string;
  name: string;
  description: string;
  version: string;
  source: "bundled" | "native";
  distribution: "bundled" | "sideload";
  official: boolean;
  install_state: "ready" | "not_installed" | "pending_restart" | "error";
  runtime_path?: string;
  manifest: PluginPackageManifest;
  signature_verified: boolean;
  unsafe: boolean;
  // Backend dropped the checksum column; signature metadata replaces it.
  signature_key_id?: string;
  signature_signed_at?: string;
  signature_expires_at?: string;
  min_host_api_version: number;
  required_capabilities?: string[];
  target_os: string;
  target_arch: string;
  last_error?: string;
  created_at?: string;
  updated_at?: string;
}

// Plugin & Connector Types

export interface ConfigSchemaField {
  name: string;
  type: string;
  label?: string;
  description?: string;
  required?: boolean;
  default?: any;
  enum?: any[];
  sensitive?: boolean;
  nested?: ConfigSchemaField[];
  item_type?: string;
  item_fields?: ConfigSchemaField[];
}

export interface PluginInstance {
  id: string;
  server_id: string;
  plugin_id: string;
  plugin_name: string;
  source?: string;
  official?: boolean;
  distribution?: string;
  install_state?: string;
  min_host_api_version?: number;
  notes: string;
  config: Record<string, any>;
  status: string;
  enabled: boolean;
  log_level: string;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface PluginDefinition {
  id: string;
  name: string;
  description: string;
  version?: string;
  source?: string;
  official?: boolean;
  distribution?: string;
  install_state?: string;
  config_schema?: { fields?: ConfigSchemaField[] };
  events?: string[];
  allow_multiple_instances?: boolean;
  long_running?: boolean;
  required_connectors?: string[];
  optional_connectors?: string[];
}

export interface ConnectorInstance {
  id: string;
  config: Record<string, any>;
  status: string;
  enabled: boolean;
  last_error?: string;
  created_at: string;
  updated_at: string;
}

export interface ConnectorDefinition {
  id: string;
  name: string;
  description: string;
  version?: string;
  config_schema?: { fields?: ConfigSchemaField[] };
  source?: string;
  official?: boolean;
  distribution?: string;
  install_state?: string;
}

export interface PluginCommand {
  id: string;
  name: string;
  description: string;
  required_permissions?: string[];
  params?: CommandParam[];
  execution_type?: string;
}

export interface CommandParam {
  name: string;
  type: string;
  label?: string;
  description?: string;
  required?: boolean;
  default?: any;
  enum?: any[];
}

export interface ClickHouseTableStats {
  table_name: string;
  total_rows: number;
  total_bytes: string;
  compressed_bytes: string;
  compression_ratio: number;
  partition_count: number;
}

export interface ClickHouseStats {
  database_name: string;
  table_count: number;
  total_rows: number;
  total_bytes: string;
  tables: ClickHouseTableStats[];
}
