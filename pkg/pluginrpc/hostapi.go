package pluginrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/rpc"
	"time"
)

// HostAPIs is the plugin-facing surface for calling back into the Aegis host.
// It is constructed inside the plugin process after the RPC server receives
// Initialize with a broker ID, and is passed to the plugin's Initialize
// method. Every method ultimately dispatches to a generic RPC call on the
// host's HostAPI dispatcher, which invokes the concrete in-host API method
// by name.
type HostAPIs struct {
	bridge *hostAPIBridge

	LogAPI       *LogAPI
	RconAPI      *RconAPI
	ServerAPI    *ServerAPI
	DatabaseAPI  *DatabaseAPI
	RuleAPI      *RuleAPI
	AdminAPI     *AdminAPI
	EventAPI     *EventAPI
	DiscordAPI   *DiscordAPI
	ConnectorAPI *ConnectorAPI
}

// newHostAPIsFromClient builds a HostAPIs around a net/rpc client pointing at
// the host's HostAPI RPC server. Called once inside Initialize.
func newHostAPIsFromClient(client *rpc.Client) *HostAPIs {
	bridge := &hostAPIBridge{client: client}
	return &HostAPIs{
		bridge:       bridge,
		LogAPI:       &LogAPI{bridge: bridge},
		RconAPI:      &RconAPI{bridge: bridge},
		ServerAPI:    &ServerAPI{bridge: bridge},
		DatabaseAPI:  &DatabaseAPI{bridge: bridge},
		RuleAPI:      &RuleAPI{bridge: bridge},
		AdminAPI:     &AdminAPI{bridge: bridge},
		EventAPI:     &EventAPI{bridge: bridge},
		DiscordAPI:   &DiscordAPI{bridge: bridge},
		ConnectorAPI: &ConnectorAPI{bridge: bridge},
	}
}

// hostAPIBridge marshals typed arguments into HostAPIRequest envelopes and
// unmarshals the response envelope back into typed output. It is the single
// RPC chokepoint for every plugin → host call.
type hostAPIBridge struct {
	client *rpc.Client
}

func (b *hostAPIBridge) call(target string, args interface{}, out interface{}) error {
	payload, err := json.Marshal(args)
	if err != nil {
		return fmt.Errorf("pluginrpc: marshal host api %s args: %w", target, err)
	}
	req := HostAPIRequest{Target: target, Payload: payload}
	var resp HostAPIResponse
	if err := b.client.Call("HostAPI.Call", req, &resp); err != nil {
		return fmt.Errorf("pluginrpc: host api %s call: %w", target, err)
	}
	if resp.Error != "" {
		return errors.New(resp.Error)
	}
	if out != nil && len(resp.Payload) > 0 {
		if err := json.Unmarshal(resp.Payload, out); err != nil {
			return fmt.Errorf("pluginrpc: unmarshal host api %s reply: %w", target, err)
		}
	}
	return nil
}

// -- LogAPI ------------------------------------------------------------------

// LogAPI writes structured log entries into the host's logger.
type LogAPI struct{ bridge *hostAPIBridge }

type logArgs struct {
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

// Info writes an info-level log entry.
func (l *LogAPI) Info(message string, fields map[string]interface{}) {
	_ = l.bridge.call("log.Info", logArgs{Message: message, Fields: fields}, nil)
}

// Warn writes a warn-level log entry.
func (l *LogAPI) Warn(message string, fields map[string]interface{}) {
	_ = l.bridge.call("log.Warn", logArgs{Message: message, Fields: fields}, nil)
}

// Error writes an error-level log entry. The err.Error() string is forwarded
// on the wire; the host re-attaches it to the log record.
func (l *LogAPI) Error(message string, err error, fields map[string]interface{}) {
	args := logArgs{Message: message, Fields: fields}
	if err != nil {
		args.Error = err.Error()
	}
	_ = l.bridge.call("log.Error", args, nil)
}

// Debug writes a debug-level log entry.
func (l *LogAPI) Debug(message string, fields map[string]interface{}) {
	_ = l.bridge.call("log.Debug", logArgs{Message: message, Fields: fields}, nil)
}

// -- RconAPI -----------------------------------------------------------------

// RconAPI exposes the restricted RCON surface available to plugins.
type RconAPI struct{ bridge *hostAPIBridge }

type rconCommandArgs struct {
	Command string `json:"command"`
}

type rconCommandReply struct {
	Response string `json:"response"`
}

// SendCommand runs an RCON command against the server the plugin is scoped to.
func (r *RconAPI) SendCommand(command string) (string, error) {
	var reply rconCommandReply
	if err := r.bridge.call("rcon.SendCommand", rconCommandArgs{Command: command}, &reply); err != nil {
		return "", err
	}
	return reply.Response, nil
}

type rconBroadcastArgs struct {
	Message string `json:"message"`
}

// Broadcast sends a chat broadcast to every player on the server.
func (r *RconAPI) Broadcast(message string) error {
	return r.bridge.call("rcon.Broadcast", rconBroadcastArgs{Message: message}, nil)
}

type rconWarnPlayerArgs struct {
	PlayerID string `json:"player_id"`
	Message  string `json:"message"`
}

// SendWarningToPlayer sends an in-game warning to a single player.
func (r *RconAPI) SendWarningToPlayer(playerID, message string) error {
	return r.bridge.call("rcon.SendWarningToPlayer", rconWarnPlayerArgs{PlayerID: playerID, Message: message}, nil)
}

type rconKickArgs struct {
	PlayerID string `json:"player_id"`
	Reason   string `json:"reason"`
}

// KickPlayer kicks a player from the server.
func (r *RconAPI) KickPlayer(playerID, reason string) error {
	return r.bridge.call("rcon.KickPlayer", rconKickArgs{PlayerID: playerID, Reason: reason}, nil)
}

type rconBanArgs struct {
	PlayerID   string        `json:"player_id"`
	Reason     string        `json:"reason"`
	Duration   time.Duration `json:"duration"`
	EventID    string        `json:"event_id,omitempty"`
	EventType  string        `json:"event_type,omitempty"`
	RuleID     *string       `json:"rule_id,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// BanPlayer bans a player for the given duration.
func (r *RconAPI) BanPlayer(playerID, reason string, duration time.Duration) error {
	return r.bridge.call("rcon.BanPlayer", rconBanArgs{PlayerID: playerID, Reason: reason, Duration: duration}, nil)
}

type banResultReply struct {
	BanID string `json:"ban_id"`
}

// BanWithEvidence bans a player and attaches an event ID as evidence.
func (r *RconAPI) BanWithEvidence(playerID, reason string, duration time.Duration, eventID, eventType string) (string, error) {
	var reply banResultReply
	if err := r.bridge.call("rcon.BanWithEvidence", rconBanArgs{PlayerID: playerID, Reason: reason, Duration: duration, EventID: eventID, EventType: eventType}, &reply); err != nil {
		return "", err
	}
	return reply.BanID, nil
}

// WarnPlayerWithRule sends a warning and logs the violation against a rule.
func (r *RconAPI) WarnPlayerWithRule(playerID, message string, ruleID *string) error {
	return r.bridge.call("rcon.WarnPlayerWithRule", rconBanArgs{PlayerID: playerID, Reason: message, RuleID: ruleID}, nil)
}

// KickPlayerWithRule kicks a player and logs the violation against a rule.
func (r *RconAPI) KickPlayerWithRule(playerID, reason string, ruleID *string) error {
	return r.bridge.call("rcon.KickPlayerWithRule", rconBanArgs{PlayerID: playerID, Reason: reason, RuleID: ruleID}, nil)
}

// BanPlayerWithRule bans a player and logs the violation against a rule.
func (r *RconAPI) BanPlayerWithRule(playerID, reason string, duration time.Duration, ruleID *string) error {
	return r.bridge.call("rcon.BanPlayerWithRule", rconBanArgs{PlayerID: playerID, Reason: reason, Duration: duration, RuleID: ruleID}, nil)
}

// BanWithEvidenceAndRule bans a player, attaches evidence, and logs against a rule.
func (r *RconAPI) BanWithEvidenceAndRule(playerID, reason string, duration time.Duration, eventID, eventType string, ruleID *string) (string, error) {
	var reply banResultReply
	if err := r.bridge.call("rcon.BanWithEvidenceAndRule", rconBanArgs{PlayerID: playerID, Reason: reason, Duration: duration, EventID: eventID, EventType: eventType, RuleID: ruleID}, &reply); err != nil {
		return "", err
	}
	return reply.BanID, nil
}

// BanWithEvidenceAndRuleAndMetadata extends BanWithEvidenceAndRule with additional metadata.
func (r *RconAPI) BanWithEvidenceAndRuleAndMetadata(playerID, reason string, duration time.Duration, eventID, eventType string, ruleID *string, metadata map[string]interface{}) (string, error) {
	var reply banResultReply
	if err := r.bridge.call("rcon.BanWithEvidenceAndRuleAndMetadata", rconBanArgs{PlayerID: playerID, Reason: reason, Duration: duration, EventID: eventID, EventType: eventType, RuleID: ruleID, Metadata: metadata}, &reply); err != nil {
		return "", err
	}
	return reply.BanID, nil
}

type rconRemoveSquadArgs struct {
	PlayerID string `json:"player_id"`
}

// RemovePlayerFromSquad removes a player from their squad (by various IDs).
func (r *RconAPI) RemovePlayerFromSquad(playerID string) error {
	return r.bridge.call("rcon.RemovePlayerFromSquad", rconRemoveSquadArgs{PlayerID: playerID}, nil)
}

// RemovePlayerFromSquadById removes a player from their squad by player ID.
func (r *RconAPI) RemovePlayerFromSquadById(playerID string) error {
	return r.bridge.call("rcon.RemovePlayerFromSquadById", rconRemoveSquadArgs{PlayerID: playerID}, nil)
}

// -- ServerAPI ---------------------------------------------------------------

// ServerAPI exposes read-only server metadata.
type ServerAPI struct{ bridge *hostAPIBridge }

// GetServerID returns the UUID (as a string) of the server the plugin is scoped to.
func (s *ServerAPI) GetServerID() (string, error) {
	var reply string
	if err := s.bridge.call("server.GetServerID", struct{}{}, &reply); err != nil {
		return "", err
	}
	return reply, nil
}

// GetServerInfo returns basic server information. Returned type is a raw map
// so the plugin SDK doesn't need to model every host struct.
func (s *ServerAPI) GetServerInfo() (map[string]interface{}, error) {
	var reply map[string]interface{}
	if err := s.bridge.call("server.GetServerInfo", struct{}{}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// GetPlayers returns the current player list as a list of maps.
func (s *ServerAPI) GetPlayers() ([]map[string]interface{}, error) {
	var reply []map[string]interface{}
	if err := s.bridge.call("server.GetPlayers", struct{}{}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// GetAdmins returns the current admin list as a list of maps.
func (s *ServerAPI) GetAdmins() ([]map[string]interface{}, error) {
	var reply []map[string]interface{}
	if err := s.bridge.call("server.GetAdmins", struct{}{}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// GetSquads returns the current squad list as a list of maps.
func (s *ServerAPI) GetSquads() ([]map[string]interface{}, error) {
	var reply []map[string]interface{}
	if err := s.bridge.call("server.GetSquads", struct{}{}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// -- DatabaseAPI -------------------------------------------------------------

// DatabaseAPI provides plugin-scoped key/value storage.
type DatabaseAPI struct{ bridge *hostAPIBridge }

type dbArgs struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type dbReply struct {
	Value string `json:"value"`
}

// GetPluginData retrieves a plugin-scoped value by key.
func (d *DatabaseAPI) GetPluginData(key string) (string, error) {
	var reply dbReply
	if err := d.bridge.call("database.GetPluginData", dbArgs{Key: key}, &reply); err != nil {
		return "", err
	}
	return reply.Value, nil
}

// SetPluginData stores a plugin-scoped value.
func (d *DatabaseAPI) SetPluginData(key, value string) error {
	return d.bridge.call("database.SetPluginData", dbArgs{Key: key, Value: value}, nil)
}

// DeletePluginData removes a plugin-scoped value.
func (d *DatabaseAPI) DeletePluginData(key string) error {
	return d.bridge.call("database.DeletePluginData", dbArgs{Key: key}, nil)
}

// -- RuleAPI -----------------------------------------------------------------

// RuleAPI provides read-only access to server rules.
type RuleAPI struct{ bridge *hostAPIBridge }

type listRulesArgs struct {
	ParentRuleID *string `json:"parent_rule_id,omitempty"`
}

// ListServerRules returns rules for the current server, optionally scoped to
// a parent rule ID.
func (r *RuleAPI) ListServerRules(parentRuleID *string) ([]map[string]interface{}, error) {
	var reply []map[string]interface{}
	if err := r.bridge.call("rule.ListServerRules", listRulesArgs{ParentRuleID: parentRuleID}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

type listRuleActionsArgs struct {
	RuleID string `json:"rule_id"`
}

// ListServerRuleActions returns escalation actions configured for a rule.
func (r *RuleAPI) ListServerRuleActions(ruleID string) ([]map[string]interface{}, error) {
	var reply []map[string]interface{}
	if err := r.bridge.call("rule.ListServerRuleActions", listRuleActionsArgs{RuleID: ruleID}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// -- AdminAPI ----------------------------------------------------------------

// AdminAPI provides admin management functionality to plugins.
type AdminAPI struct{ bridge *hostAPIBridge }

type addTempAdminArgs struct {
	PlayerID  string     `json:"player_id"`
	RoleName  string     `json:"role_name"`
	Notes     string     `json:"notes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// AddTemporaryAdmin grants a player a temporary admin role.
func (a *AdminAPI) AddTemporaryAdmin(playerID, roleName, notes string, expiresAt *time.Time) error {
	return a.bridge.call("admin.AddTemporaryAdmin", addTempAdminArgs{PlayerID: playerID, RoleName: roleName, Notes: notes, ExpiresAt: expiresAt}, nil)
}

type removeTempAdminArgs struct {
	PlayerID string `json:"player_id"`
	RoleName string `json:"role_name,omitempty"`
	Notes    string `json:"notes"`
}

// RemoveTemporaryAdmin removes all temporary admin grants for a player.
func (a *AdminAPI) RemoveTemporaryAdmin(playerID, notes string) error {
	return a.bridge.call("admin.RemoveTemporaryAdmin", removeTempAdminArgs{PlayerID: playerID, Notes: notes}, nil)
}

// RemoveTemporaryAdminRole removes a specific temporary admin role from a player.
func (a *AdminAPI) RemoveTemporaryAdminRole(playerID, roleName, notes string) error {
	return a.bridge.call("admin.RemoveTemporaryAdminRole", removeTempAdminArgs{PlayerID: playerID, RoleName: roleName, Notes: notes}, nil)
}

type playerIDArgs struct {
	PlayerID string `json:"player_id"`
}

// GetPlayerAdminStatus checks a player's admin status and roles.
func (a *AdminAPI) GetPlayerAdminStatus(playerID string) (map[string]interface{}, error) {
	var reply map[string]interface{}
	if err := a.bridge.call("admin.GetPlayerAdminStatus", playerIDArgs{PlayerID: playerID}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// ListTemporaryAdmins lists all plugin-managed temporary admins.
func (a *AdminAPI) ListTemporaryAdmins() ([]map[string]interface{}, error) {
	var reply []map[string]interface{}
	if err := a.bridge.call("admin.ListTemporaryAdmins", struct{}{}, &reply); err != nil {
		return nil, err
	}
	return reply, nil
}

// -- EventAPI ----------------------------------------------------------------

// EventAPI lets plugins publish custom events into the host event bus.
type EventAPI struct{ bridge *hostAPIBridge }

type publishEventArgs struct {
	EventType string                 `json:"event_type"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Raw       string                 `json:"raw,omitempty"`
}

// PublishEvent publishes a custom plugin event. Note that SubscribeToEvents
// is intentionally NOT exposed to subprocess plugins — events are delivered
// to the plugin via the HandleEvent RPC call instead.
func (e *EventAPI) PublishEvent(eventType string, data map[string]interface{}, raw string) error {
	return e.bridge.call("event.PublishEvent", publishEventArgs{EventType: eventType, Data: data, Raw: raw}, nil)
}

// -- DiscordAPI --------------------------------------------------------------

// DiscordAPI sends messages through the host's configured Discord connector.
type DiscordAPI struct{ bridge *hostAPIBridge }

type discordMessageArgs struct {
	ChannelID string                 `json:"channel_id"`
	Content   string                 `json:"content,omitempty"`
	Embed     map[string]interface{} `json:"embed,omitempty"`
}

type discordMessageReply struct {
	MessageID string `json:"message_id"`
}

// SendMessage sends a plain text message to a Discord channel.
func (d *DiscordAPI) SendMessage(channelID, content string) (string, error) {
	var reply discordMessageReply
	if err := d.bridge.call("discord.SendMessage", discordMessageArgs{ChannelID: channelID, Content: content}, &reply); err != nil {
		return "", err
	}
	return reply.MessageID, nil
}

// SendEmbed sends a Discord embed message to a channel. The embed is passed
// as a generic map so the wire format does not need to model every embed field.
func (d *DiscordAPI) SendEmbed(channelID string, embed map[string]interface{}) (string, error) {
	var reply discordMessageReply
	if err := d.bridge.call("discord.SendEmbed", discordMessageArgs{ChannelID: channelID, Embed: embed}, &reply); err != nil {
		return "", err
	}
	return reply.MessageID, nil
}

// -- ConnectorAPI ------------------------------------------------------------

// ConnectorInvokeRequest mirrors plugin_manager.ConnectorInvokeRequest on the wire.
type ConnectorInvokeRequest struct {
	V    string                 `json:"v"`
	Data map[string]interface{} `json:"data"`
}

// ConnectorInvokeResponse mirrors plugin_manager.ConnectorInvokeResponse on the wire.
type ConnectorInvokeResponse struct {
	V     string                 `json:"v"`
	OK    bool                   `json:"ok"`
	Data  map[string]interface{} `json:"data,omitempty"`
	Error string                 `json:"error,omitempty"`
}

// ConnectorAPI lets plugins invoke connectors by ID with a JSON envelope.
type ConnectorAPI struct{ bridge *hostAPIBridge }

type connectorCallArgs struct {
	ConnectorID string                 `json:"connector_id"`
	V           string                 `json:"v"`
	Data        map[string]interface{} `json:"data,omitempty"`
	TimeoutMs   int64                  `json:"timeout_ms,omitempty"`
}

// Call invokes a connector by ID with a JSON envelope. The context deadline
// is translated into a millisecond timeout on the wire.
func (c *ConnectorAPI) Call(ctx context.Context, connectorID string, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error) {
	args := connectorCallArgs{ConnectorID: connectorID}
	if req != nil {
		args.V = req.V
		args.Data = req.Data
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < 0 {
			return nil, context.DeadlineExceeded
		}
		args.TimeoutMs = remaining.Milliseconds()
	}
	var reply ConnectorInvokeResponse
	if err := c.bridge.call("connector.Call", args, &reply); err != nil {
		return nil, err
	}
	return &reply, nil
}
