CREATE TABLE public.server_motd_config (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    server_id uuid NOT NULL UNIQUE REFERENCES servers(id) ON DELETE CASCADE,

    -- Content
    prefix_text TEXT DEFAULT '',
    suffix_text TEXT DEFAULT '',

    -- Generation settings
    auto_generate_from_rules BOOLEAN DEFAULT TRUE,
    include_rule_descriptions BOOLEAN DEFAULT TRUE,

    -- Upload settings
    upload_enabled BOOLEAN DEFAULT FALSE,
    auto_upload_on_change BOOLEAN DEFAULT FALSE,
    motd_file_path TEXT DEFAULT '/SquadGame/ServerConfig/MOTD.cfg',

    -- Credential override (if different from log config)
    use_log_credentials BOOLEAN DEFAULT TRUE,
    upload_host TEXT,
    upload_port INTEGER CHECK (upload_port > 0 AND upload_port <= 65535),
    upload_username TEXT,
    upload_password TEXT,
    upload_protocol TEXT CHECK (upload_protocol IN ('sftp', 'ftp')),

    -- Tracking
    last_uploaded_at TIMESTAMPTZ,
    last_upload_error TEXT,
    last_generated_content TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_server_motd_config_server_id ON public.server_motd_config(server_id);
