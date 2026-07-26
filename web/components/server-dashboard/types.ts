export interface Player {
    steam_id: string;
    name: string;
    squadId: number | null;
    isSquadLeader: boolean;
    role: string;
    kills?: number;
    deaths?: number;
    team?: string;
    squad?: string;
}

export interface Squad {
    id: number;
    name: string;
    size: number;
    locked: boolean;
    leader: Player | null;
    players: Player[];
}

export interface Team {
    id: number;
    name: string;
    squads: Squad[];
    players: Player[]; // Unassigned players
}

export interface RecentJoin {
    id: string;
    player_name: string;
    steam_id?: string;
    eos_id?: string;
    joined_at: string;
}
