-- Create ban_lists table
CREATE TABLE ban_lists (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_global BOOLEAN DEFAULT FALSE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Create server_ban_list_subscriptions table to link servers to ban lists
CREATE TABLE server_ban_list_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id UUID NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    ban_list_id UUID NOT NULL REFERENCES ban_lists(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    UNIQUE(server_id, ban_list_id)
);

-- Add ban_list_id column to server_bans table
ALTER TABLE server_bans ADD COLUMN ban_list_id UUID REFERENCES ban_lists(id) ON DELETE SET NULL;

-- Add index for faster lookups
CREATE INDEX server_bans_ban_list_id_idx ON server_bans(ban_list_id);
CREATE INDEX server_ban_list_subscriptions_ban_list_id_idx ON server_ban_list_subscriptions(ban_list_id);
CREATE INDEX server_ban_list_subscriptions_server_id_idx ON server_ban_list_subscriptions(server_id); 