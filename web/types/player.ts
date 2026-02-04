// Player Profile Types for Admin Dashboard

export interface PlayerProfile {
  steam_id: string;
  eos_id: string;
  player_name: string;
  last_seen: string | null;
  first_seen: string | null;
  total_play_time: number;
  total_sessions: number;
  statistics: PlayerStatistics;
  recent_activity: PlayerActivity[];
  chat_history: ChatMessage[];
  violations: RuleViolation[];
  recent_servers: RecentServerInfo[];

  // Admin-focused fields
  active_bans: ActiveBan[];
  violation_summary: ViolationSummary;
  teamkill_metrics: TeamkillMetrics;
  risk_indicators: RiskIndicator[];
  name_history: NameHistoryEntry[];
  weapon_stats: WeaponStat[];
}

export interface PlayerStatistics {
  kills: number;
  deaths: number;
  teamkills: number;
  revives: number;
  times_revived: number;
  damage_dealt: number;
  damage_taken: number;
  kd_ratio: number;
}

export interface PlayerActivity {
  event_time: string;
  event_type: string;
  description: string;
  server_id: string;
  server_name?: string;
}

export interface ChatMessage {
  message_id?: string;
  sent_at: string;
  message: string;
  chat_type: "all" | "team" | "squad" | "admin";
  server_id: string;
  server_name?: string;
}

export interface RuleViolation {
  violation_id: string;
  server_id: string;
  server_name?: string;
  rule_id: string | null;
  rule_name?: string | null;
  action_type: "WARN" | "KICK" | "BAN";
  admin_user_id: string | null;
  admin_name?: string | null;
  reason?: string;
  created_at: string;
}

export interface RecentServerInfo {
  server_id: string;
  server_name: string;
  last_seen: string;
  sessions: number;
}

export interface ActiveBan {
  ban_id: string;
  server_id: string;
  server_name: string;
  reason: string;
  duration: number;
  permanent: boolean;
  expires_at?: string;
  created_at: string;
  admin_name: string;
}

export interface ViolationSummary {
  total_warns: number;
  total_kicks: number;
  total_bans: number;
  last_action?: string;
}

export interface TeamkillMetrics {
  total_teamkills: number;
  teamkills_per_session: number;
  teamkill_ratio: number;
  recent_teamkills: number;
}

export interface RiskIndicator {
  type:
    | "active_ban"
    | "high_tk_rate"
    | "elevated_tk_rate"
    | "recent_teamkills"
    | "multiple_names"
    | "prior_bans"
    | "high_violations"
    | "multiple_violations"
    | "cbl_flagged"
    | "ip_shared";
  severity: "critical" | "high" | "medium" | "low";
  description: string;
}

export interface NameHistoryEntry {
  name: string;
  first_used: string;
  last_used: string;
  session_count: number;
}

export interface WeaponStat {
  weapon: string;
  kills: number;
  teamkills: number;
}

export interface TeamkillVictim {
  victim_name: string;
  victim_steam: string;
  victim_eos: string;
  tk_count: number;
  weapons_used: string[];
  first_tk: string;
  last_tk: string;
}

export interface SessionHistoryEntry {
  connect_time: string;
  disconnect_time?: string | null;
  duration_seconds?: number | null;
  server_id: string;
  server_name?: string;
  ip?: string;
  missing_disconnect: boolean;
  ongoing: boolean;
}

export interface CombatHistoryEntry {
  event_time: string;
  event_type: "kill" | "death" | "wounded" | "wounded_by" | "damaged" | "damaged_by";
  server_id: string;
  server_name?: string;
  weapon: string;
  damage: number;
  teamkill: boolean;
  other_name: string;
  other_steam_id?: string;
  other_eos_id?: string;
  other_team: string;
  other_squad: string;
  player_team: string;
  player_squad: string;
}

export interface RelatedPlayer {
  steam_id: string;
  eos_id: string;
  player_name: string;
  relation_type: "same_ip";
  shared_sessions: number;
  is_banned: boolean;
}

export interface AltAccountPlayer {
  steam_id: string;
  eos_id: string;
  player_name: string;
  is_banned: boolean;
  shared_sessions: number;
  last_seen: string | null;
}

export interface AltAccountGroup {
  group_id: string;
  players: AltAccountPlayer[];
  shared_ip_count: number;
  last_activity: string | null;
}

export interface AltAccountGroupsResponse {
  alt_groups: AltAccountGroup[];
  total_groups: number;
  page: number;
  limit: number;
}

export interface PaginatedChatHistory {
  messages: ChatMessage[];
  total: number;
  page: number;
  limit: number;
  total_pages: number;
}

export interface CBLUser {
  id: string;
  name: string;
  avatarFull: string;
  reputationPoints: number;
  riskRating: number;
  reputationRank: number;
  lastRefreshedInfo: string;
  lastRefreshedReputationPoints: string;
  lastRefreshedReputationRank: string;
  reputationPointsMonthChange: number;
}
