DROP VIEW IF EXISTS squad_aegis.mv_daily_chat_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_connected_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_disconnected_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_wounded_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_revived_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_possess_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_died_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_damaged_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_deployable_damaged_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_admin_broadcast_count;
DROP VIEW IF EXISTS squad_aegis.mv_daily_round_ended_count;
DROP VIEW IF EXISTS squad_aegis.mv_server_info_daily;
DROP VIEW IF EXISTS squad_aegis.mv_server_player_daily_uniques;
DROP VIEW IF EXISTS squad_aegis.mv_server_workflow_daily;
DROP VIEW IF EXISTS squad_aegis.mv_player_ip_activity;

--migration:split

DROP TABLE IF EXISTS squad_aegis.sql_rollup_state;
DROP TABLE IF EXISTS squad_aegis.player_ip_activity;
DROP TABLE IF EXISTS squad_aegis.server_workflow_daily;
DROP TABLE IF EXISTS squad_aegis.server_player_daily_uniques;
DROP TABLE IF EXISTS squad_aegis.server_info_daily;
DROP TABLE IF EXISTS squad_aegis.server_event_daily_counts;
