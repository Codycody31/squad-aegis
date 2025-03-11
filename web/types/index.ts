export interface User {
  id: string;
  steam_id: number;
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
  rcon_port: number;
  rcon_password: string;
  created_at: string;
  updated_at: string;
}

export interface ServerBan {
  id: string;
  server_id: string;
  admin_id: string;
  steam_id: number;
  reason: string;
  duration: number;
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