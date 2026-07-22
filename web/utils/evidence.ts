// Pure ban/evidence formatting helpers extracted from
// pages/servers/[serverId]/banned-players.vue.

import type { BannedPlayer } from "~/types/bans";

// Helper function to clean and validate a player ID (Steam ID or EOS ID)
export function cleanPlayerId(playerId: string | number | undefined | null): { steamId?: string; eosId?: string; rconId: string } {
    if (!playerId) {
        throw new Error("Player ID is required");
    }
    // Remove any quotes and whitespace
    const cleaned = String(playerId).trim().replace(/['"]/g, '');

    // Check if it's a Steam ID (17-digit number)
    if (/^\d{17}$/.test(cleaned)) {
        return { steamId: cleaned, rconId: cleaned };
    }

    // Check if it's an EOS ID (32-char hex)
    if (/^[0-9a-fA-F]{32}$/.test(cleaned)) {
        const normalizedEOSID = cleaned.toLowerCase();
        return { eosId: normalizedEOSID, rconId: normalizedEOSID };
    }

    throw new Error("Must be a 17-digit Steam ID or 32-character hex EOS ID");
}

export function hasValidEvidencePlayerId(playerId?: string | null): boolean {
    try {
        cleanPlayerId(playerId);
        return true;
    } catch {
        return false;
    }
}

// Function to get ClickHouse table name for event type
export function getClickhouseTableForType(type: string): string {
    const tableMap: Record<string, string> = {
        chat_message: "server_player_chat_messages",
        player_connected: "server_player_connected_events",
    };
    return tableMap[type] || "server_player_chat_messages";
}

// Function to format event for display
export function formatEventDescription(event: any, type: string): string {
    if (type === "chat_message") {
        return event.message || "No message";
    } else if (type === "player_connected") {
        const joinedName = event.joined_name || event.player_controller || "Unknown Player";
        const ip = event.ip || "Unknown IP";
        return `${joinedName} @ ${ip}`;
    } else if (type === "file_upload") {
        return event.file_name || "Uploaded file";
    } else if (type === "text_paste") {
        const text = event.text_content || "";
        return text.length > 50 ? text.substring(0, 50) + "..." : text;
    }
    return "Event";
}

export function getEvidenceDisplayPayload(evidence: any): any {
    if (evidence?.metadata && Object.keys(evidence.metadata).length > 0) {
        return evidence.metadata;
    }
    return evidence || {};
}

export function getEvidenceDisplayTime(evidence: any): string | null {
    const payload = getEvidenceDisplayPayload(evidence);
    return payload.event_time || payload.sent_at || evidence?.event_time || evidence?.sent_at || null;
}

// Function to format file size
export function formatFileSize(bytes: number): string {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + " KB";
    return (bytes / (1024 * 1024)).toFixed(2) + " MB";
}

// Format expires_at for display
export function formatExpiresAt(ban: BannedPlayer): string {
    if (ban.permanent || !ban.expires_at) return "Permanent";
    const expiresAt = new Date(ban.expires_at);
    const now = new Date();
    const diffMs = expiresAt.getTime() - now.getTime();
    if (diffMs <= 0) return "Expired";
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    const diffHours = Math.floor((diffMs % (1000 * 60 * 60 * 24)) / (1000 * 60 * 60));
    const diffMins = Math.floor((diffMs % (1000 * 60 * 60)) / (1000 * 60));
    if (diffDays > 0) return `${diffDays}d ${diffHours}h remaining`;
    if (diffHours > 0) return `${diffHours}h ${diffMins}m remaining`;
    return `${diffMins}m remaining`;
}

// Format date
export function formatDate(dateString: string): string {
    return new Date(dateString).toLocaleString();
}
