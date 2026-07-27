// Ban domain types shared by the banned-players page and components/bans/*.
// Field names mirror the backend API payloads exactly.

import type { PageInfo } from "~/types";

export interface BanEvidence {
    id: string;
    evidence_type: string;
    clickhouse_table?: string | null;
    record_id?: string | null;
    event_time?: string | null;
    metadata?: any;
    // File upload evidence fields
    file_path?: string | null;
    file_name?: string | null;
    file_size?: number | null;
    file_type?: string | null;
    // Text paste evidence field
    text_content?: string | null;
}

export interface BannedPlayer {
    id: string;
    server_id: string;
    admin_id?: string;
    admin_name: string;
    steam_id: string;
    eos_id?: string;
    name: string;
    reason: string;
    permanent: boolean;
    expires_at?: string;
    created_at: string;
    updated_at: string;
    ban_list_id?: string;
    ban_list_name?: string;
    rule_id?: string;
    rule_name?: string;
    rule_number?: string;
    player_name?: string;
    evidence_text?: string;
    evidence?: BanEvidence[];
    has_evidence: boolean;
}

export interface BannedPlayersResponse {
    data: {
        bans: BannedPlayer[];
        page_info: PageInfo;
    };
}

export interface BannedPlayerDetailResponse {
    data: {
        ban: BannedPlayer;
    };
}

// ---- Evidence drafts (composed in the add/edit ban dialogs before submit) ----

// ClickHouse event evidence selected from /servers/:id/events/search results.
export interface EventEvidenceDraft {
    evidence_type: string;
    clickhouse_table: string;
    record_id: string;
    event_time?: string | null;
    metadata?: any;
}

// File evidence uploaded via /servers/:id/evidence/upload.
export interface FileEvidenceDraft {
    evidence_type: "file_upload";
    file_path: string;
    file_name: string;
    file_size: number;
    file_type: string;
    file_id?: string;
}

// Pasted text evidence.
export interface TextEvidenceDraft {
    evidence_type: "text_paste";
    text_content: string;
}

// ---- Ban lists ----

export interface BanList {
    id: string;
    name: string;
    is_remote?: boolean;
}

export interface BanListSubscription {
    ban_list_id: string;
    ban_list_name?: string;
    created_at: string;
}

// Flattened server rule as used in the ban dialogs' rule dropdowns.
export interface ServerRuleOption {
    id: string;
    title: string;
    description?: string;
    number: string;
}

// ---- Ban Import from Bans.cfg ----

export interface CfgBanEntry {
    steam_id: string;
    eos_id: string;
    expiry_timestamp: number;
    reason: string;
    permanent: boolean;
    expired: boolean;
    is_auto_ban: boolean;
    raw_line: string;
}

export interface BanImportPreviewData {
    cfg_available: boolean;
    cfg_path: string;
    new_bans: CfgBanEntry[];
    existing_bans: CfgBanEntry[];
    expired_bans: CfgBanEntry[];
    auto_bans: CfgBanEntry[];
    unparseable_count: number;
}

export interface BanImportResultData {
    imported_count: number;
    skipped_count: number;
    expired_count: number;
    errors: string[];
}
