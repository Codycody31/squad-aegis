package pluginrpc

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	pluginrpcpb "go.codycody31.dev/squad-aegis/pkg/pluginrpc/proto"
)

// HostAPIs is the plugin-facing surface for calling back into the Aegis host.
// It is constructed inside the plugin process after the gRPC server receives
// Initialize with a broker ID, and is passed to the plugin's Initialize
// method. Every method ultimately dispatches to a concrete RPC on the host's
// HostAPI gRPC service.
type HostAPIs struct {
	client pluginrpcpb.HostAPIClient

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

// newHostAPIsFromConn builds a HostAPIs around a gRPC client connection
// pointing at the host's HostAPI gRPC server. Called once inside Initialize.
func newHostAPIsFromConn(conn *grpc.ClientConn) *HostAPIs {
	client := pluginrpcpb.NewHostAPIClient(conn)
	apis := &HostAPIs{client: client}
	apis.LogAPI = &LogAPI{client: client}
	apis.RconAPI = &RconAPI{client: client}
	apis.ServerAPI = &ServerAPI{client: client}
	apis.DatabaseAPI = &DatabaseAPI{client: client}
	apis.RuleAPI = &RuleAPI{client: client}
	apis.AdminAPI = &AdminAPI{client: client}
	apis.EventAPI = &EventAPI{client: client}
	apis.DiscordAPI = &DiscordAPI{client: client}
	apis.ConnectorAPI = &ConnectorAPI{client: client}
	return apis
}

// -- LogAPI ------------------------------------------------------------------

// LogAPI writes structured log entries into the host's logger.
type LogAPI struct{ client pluginrpcpb.HostAPIClient }

func (l *LogAPI) buildRequest(message string, fields map[string]interface{}, errMsg string) (*pluginrpcpb.LogRequest, error) {
	encoded, err := encodeJSONMap(fields)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.LogRequest{
		Message:    message,
		FieldsJson: encoded,
		Error:      errMsg,
	}, nil
}

// Info writes an info-level log entry.
func (l *LogAPI) Info(message string, fields map[string]interface{}) {
	req, err := l.buildRequest(message, fields, "")
	if err != nil {
		return
	}
	_, _ = l.client.LogInfo(context.Background(), req)
}

// Warn writes a warn-level log entry.
func (l *LogAPI) Warn(message string, fields map[string]interface{}) {
	req, err := l.buildRequest(message, fields, "")
	if err != nil {
		return
	}
	_, _ = l.client.LogWarn(context.Background(), req)
}

// Error writes an error-level log entry. The err.Error() string is forwarded
// on the wire; the host re-attaches it to the log record.
func (l *LogAPI) Error(message string, err error, fields map[string]interface{}) {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	req, buildErr := l.buildRequest(message, fields, errMsg)
	if buildErr != nil {
		return
	}
	_, _ = l.client.LogError(context.Background(), req)
}

// Debug writes a debug-level log entry.
func (l *LogAPI) Debug(message string, fields map[string]interface{}) {
	req, err := l.buildRequest(message, fields, "")
	if err != nil {
		return
	}
	_, _ = l.client.LogDebug(context.Background(), req)
}

// -- RconAPI -----------------------------------------------------------------

// RconAPI exposes the restricted RCON surface available to plugins.
type RconAPI struct{ client pluginrpcpb.HostAPIClient }

func newRconBanRequest(playerID, reason string, duration time.Duration, eventID, eventType string, ruleID *string, metadata map[string]interface{}) (*pluginrpcpb.RconBanRequest, error) {
	encoded, err := encodeJSONMap(metadata)
	if err != nil {
		return nil, err
	}
	req := &pluginrpcpb.RconBanRequest{
		PlayerId:     playerID,
		Reason:       reason,
		DurationNs:   int64(duration),
		EventId:      eventID,
		EventType:    eventType,
		MetadataJson: encoded,
	}
	if ruleID != nil {
		req.RuleId = *ruleID
		req.RuleIdSet = true
	}
	return req, nil
}

// SendCommand runs an RCON command against the server the plugin is scoped to.
func (r *RconAPI) SendCommand(command string) (string, error) {
	resp, err := r.client.RconSendCommand(context.Background(), &pluginrpcpb.RconCommandRequest{Command: command})
	if err != nil {
		return "", err
	}
	return resp.GetResponse(), nil
}

// Broadcast sends a chat broadcast to every player on the server.
func (r *RconAPI) Broadcast(message string) error {
	_, err := r.client.RconBroadcast(context.Background(), &pluginrpcpb.RconBroadcastRequest{Message: message})
	return err
}

// SendWarningToPlayer sends an in-game warning to a single player.
func (r *RconAPI) SendWarningToPlayer(playerID, message string) error {
	_, err := r.client.RconSendWarningToPlayer(context.Background(), &pluginrpcpb.RconWarnPlayerRequest{
		PlayerId: playerID,
		Message:  message,
	})
	return err
}

// KickPlayer kicks a player from the server.
func (r *RconAPI) KickPlayer(playerID, reason string) error {
	_, err := r.client.RconKickPlayer(context.Background(), &pluginrpcpb.RconKickRequest{
		PlayerId: playerID,
		Reason:   reason,
	})
	return err
}

// BanPlayer bans a player for the given duration.
func (r *RconAPI) BanPlayer(playerID, reason string, duration time.Duration) error {
	req, err := newRconBanRequest(playerID, reason, duration, "", "", nil, nil)
	if err != nil {
		return err
	}
	_, err = r.client.RconBanPlayer(context.Background(), req)
	return err
}

// BanWithEvidence bans a player and attaches an event ID as evidence.
func (r *RconAPI) BanWithEvidence(playerID, reason string, duration time.Duration, eventID, eventType string) (string, error) {
	req, err := newRconBanRequest(playerID, reason, duration, eventID, eventType, nil, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.RconBanWithEvidence(context.Background(), req)
	if err != nil {
		return "", err
	}
	return resp.GetBanId(), nil
}

// WarnPlayerWithRule sends a warning and logs the violation against a rule.
func (r *RconAPI) WarnPlayerWithRule(playerID, message string, ruleID *string) error {
	req, err := newRconBanRequest(playerID, message, 0, "", "", ruleID, nil)
	if err != nil {
		return err
	}
	_, err = r.client.RconWarnPlayerWithRule(context.Background(), req)
	return err
}

// KickPlayerWithRule kicks a player and logs the violation against a rule.
func (r *RconAPI) KickPlayerWithRule(playerID, reason string, ruleID *string) error {
	req, err := newRconBanRequest(playerID, reason, 0, "", "", ruleID, nil)
	if err != nil {
		return err
	}
	_, err = r.client.RconKickPlayerWithRule(context.Background(), req)
	return err
}

// BanPlayerWithRule bans a player and logs the violation against a rule.
func (r *RconAPI) BanPlayerWithRule(playerID, reason string, duration time.Duration, ruleID *string) error {
	req, err := newRconBanRequest(playerID, reason, duration, "", "", ruleID, nil)
	if err != nil {
		return err
	}
	_, err = r.client.RconBanPlayerWithRule(context.Background(), req)
	return err
}

// BanWithEvidenceAndRule bans a player, attaches evidence, and logs against a rule.
func (r *RconAPI) BanWithEvidenceAndRule(playerID, reason string, duration time.Duration, eventID, eventType string, ruleID *string) (string, error) {
	req, err := newRconBanRequest(playerID, reason, duration, eventID, eventType, ruleID, nil)
	if err != nil {
		return "", err
	}
	resp, err := r.client.RconBanWithEvidenceAndRule(context.Background(), req)
	if err != nil {
		return "", err
	}
	return resp.GetBanId(), nil
}

// BanWithEvidenceAndRuleAndMetadata extends BanWithEvidenceAndRule with additional metadata.
func (r *RconAPI) BanWithEvidenceAndRuleAndMetadata(playerID, reason string, duration time.Duration, eventID, eventType string, ruleID *string, metadata map[string]interface{}) (string, error) {
	req, err := newRconBanRequest(playerID, reason, duration, eventID, eventType, ruleID, metadata)
	if err != nil {
		return "", err
	}
	resp, err := r.client.RconBanWithEvidenceAndRuleAndMetadata(context.Background(), req)
	if err != nil {
		return "", err
	}
	return resp.GetBanId(), nil
}

// RemovePlayerFromSquad removes a player from their squad (by various IDs).
func (r *RconAPI) RemovePlayerFromSquad(playerID string) error {
	_, err := r.client.RconRemovePlayerFromSquad(context.Background(), &pluginrpcpb.RconRemoveSquadRequest{PlayerId: playerID})
	return err
}

// RemovePlayerFromSquadById removes a player from their squad by player ID.
func (r *RconAPI) RemovePlayerFromSquadById(playerID string) error {
	_, err := r.client.RconRemovePlayerFromSquadById(context.Background(), &pluginrpcpb.RconRemoveSquadRequest{PlayerId: playerID})
	return err
}

// -- ServerAPI ---------------------------------------------------------------

// ServerAPI exposes read-only server metadata.
type ServerAPI struct{ client pluginrpcpb.HostAPIClient }

// GetServerID returns the UUID (as a string) of the server the plugin is scoped to.
func (s *ServerAPI) GetServerID() (string, error) {
	resp, err := s.client.ServerGetServerID(context.Background(), &pluginrpcpb.Empty{})
	if err != nil {
		return "", err
	}
	return resp.GetValue(), nil
}

func (s *ServerAPI) callJSONMap(fn func(context.Context, *pluginrpcpb.Empty, ...grpc.CallOption) (*pluginrpcpb.JSONResponse, error)) (map[string]interface{}, error) {
	resp, err := fn(context.Background(), &pluginrpcpb.Empty{})
	if err != nil {
		return nil, err
	}
	return decodeJSONMap(resp.GetDataJson())
}

func (s *ServerAPI) callJSONList(fn func(context.Context, *pluginrpcpb.Empty, ...grpc.CallOption) (*pluginrpcpb.JSONResponse, error)) ([]map[string]interface{}, error) {
	resp, err := fn(context.Background(), &pluginrpcpb.Empty{})
	if err != nil {
		return nil, err
	}
	return decodeJSONListOfMaps(resp.GetDataJson())
}

// GetServerInfo returns basic server information.
func (s *ServerAPI) GetServerInfo() (map[string]interface{}, error) {
	return s.callJSONMap(s.client.ServerGetServerInfo)
}

// GetPlayers returns the current player list as a list of maps.
func (s *ServerAPI) GetPlayers() ([]map[string]interface{}, error) {
	return s.callJSONList(s.client.ServerGetPlayers)
}

// GetAdmins returns the current admin list as a list of maps.
func (s *ServerAPI) GetAdmins() ([]map[string]interface{}, error) {
	return s.callJSONList(s.client.ServerGetAdmins)
}

// GetSquads returns the current squad list as a list of maps.
func (s *ServerAPI) GetSquads() ([]map[string]interface{}, error) {
	return s.callJSONList(s.client.ServerGetSquads)
}

// -- DatabaseAPI -------------------------------------------------------------

// DatabaseAPI provides plugin-scoped key/value storage.
type DatabaseAPI struct{ client pluginrpcpb.HostAPIClient }

// GetPluginData retrieves a plugin-scoped value by key.
func (d *DatabaseAPI) GetPluginData(key string) (string, error) {
	resp, err := d.client.DatabaseGetPluginData(context.Background(), &pluginrpcpb.DatabaseRequest{Key: key})
	if err != nil {
		return "", err
	}
	return resp.GetValue(), nil
}

// SetPluginData stores a plugin-scoped value.
func (d *DatabaseAPI) SetPluginData(key, value string) error {
	_, err := d.client.DatabaseSetPluginData(context.Background(), &pluginrpcpb.DatabaseRequest{Key: key, Value: value})
	return err
}

// DeletePluginData removes a plugin-scoped value.
func (d *DatabaseAPI) DeletePluginData(key string) error {
	_, err := d.client.DatabaseDeletePluginData(context.Background(), &pluginrpcpb.DatabaseRequest{Key: key})
	return err
}

// -- RuleAPI -----------------------------------------------------------------

// RuleAPI provides read-only access to server rules.
type RuleAPI struct{ client pluginrpcpb.HostAPIClient }

// ListServerRules returns rules for the current server, optionally scoped to
// a parent rule ID.
func (r *RuleAPI) ListServerRules(parentRuleID *string) ([]map[string]interface{}, error) {
	req := &pluginrpcpb.ListRulesRequest{}
	if parentRuleID != nil {
		req.ParentRuleId = *parentRuleID
		req.ParentRuleIdSet = true
	}
	resp, err := r.client.RuleListServerRules(context.Background(), req)
	if err != nil {
		return nil, err
	}
	return decodeJSONListOfMaps(resp.GetDataJson())
}

// ListServerRuleActions returns escalation actions configured for a rule.
func (r *RuleAPI) ListServerRuleActions(ruleID string) ([]map[string]interface{}, error) {
	resp, err := r.client.RuleListServerRuleActions(context.Background(), &pluginrpcpb.ListRuleActionsRequest{RuleId: ruleID})
	if err != nil {
		return nil, err
	}
	return decodeJSONListOfMaps(resp.GetDataJson())
}

// -- AdminAPI ----------------------------------------------------------------

// AdminAPI provides admin management functionality to plugins.
type AdminAPI struct{ client pluginrpcpb.HostAPIClient }

// AddTemporaryAdmin grants a player a temporary admin role.
func (a *AdminAPI) AddTemporaryAdmin(playerID, roleName, notes string, expiresAt *time.Time) error {
	req := &pluginrpcpb.AddTempAdminRequest{
		PlayerId: playerID,
		RoleName: roleName,
		Notes:    notes,
	}
	if expiresAt != nil {
		req.ExpiresAt = timestamppb.New(*expiresAt)
	}
	_, err := a.client.AdminAddTemporaryAdmin(context.Background(), req)
	return err
}

// RemoveTemporaryAdmin removes all temporary admin grants for a player.
func (a *AdminAPI) RemoveTemporaryAdmin(playerID, notes string) error {
	_, err := a.client.AdminRemoveTemporaryAdmin(context.Background(), &pluginrpcpb.RemoveTempAdminRequest{
		PlayerId: playerID,
		Notes:    notes,
	})
	return err
}

// RemoveTemporaryAdminRole removes a specific temporary admin role from a player.
func (a *AdminAPI) RemoveTemporaryAdminRole(playerID, roleName, notes string) error {
	_, err := a.client.AdminRemoveTemporaryAdminRole(context.Background(), &pluginrpcpb.RemoveTempAdminRequest{
		PlayerId: playerID,
		RoleName: roleName,
		Notes:    notes,
	})
	return err
}

// GetPlayerAdminStatus checks a player's admin status and roles.
func (a *AdminAPI) GetPlayerAdminStatus(playerID string) (map[string]interface{}, error) {
	resp, err := a.client.AdminGetPlayerAdminStatus(context.Background(), &pluginrpcpb.PlayerIDRequest{PlayerId: playerID})
	if err != nil {
		return nil, err
	}
	return decodeJSONMap(resp.GetDataJson())
}

// ListTemporaryAdmins lists all plugin-managed temporary admins.
func (a *AdminAPI) ListTemporaryAdmins() ([]map[string]interface{}, error) {
	resp, err := a.client.AdminListTemporaryAdmins(context.Background(), &pluginrpcpb.Empty{})
	if err != nil {
		return nil, err
	}
	return decodeJSONListOfMaps(resp.GetDataJson())
}

// -- EventAPI ----------------------------------------------------------------

// EventAPI lets plugins publish custom events into the host event bus.
type EventAPI struct{ client pluginrpcpb.HostAPIClient }

// PublishEvent publishes a custom plugin event. SubscribeToEvents is
// intentionally NOT exposed to subprocess plugins; events are delivered to
// the plugin via the HandleEvent RPC call instead.
func (e *EventAPI) PublishEvent(eventType string, data map[string]interface{}, raw string) error {
	encoded, err := encodeJSONMap(data)
	if err != nil {
		return fmt.Errorf("encode event data: %w", err)
	}
	_, err = e.client.EventPublishEvent(context.Background(), &pluginrpcpb.PublishEventRequest{
		EventType: eventType,
		DataJson:  encoded,
		Raw:       raw,
	})
	return err
}

// -- DiscordAPI --------------------------------------------------------------

// DiscordAPI sends messages through the host's configured Discord connector.
type DiscordAPI struct{ client pluginrpcpb.HostAPIClient }

// SendMessage sends a plain text message to a Discord channel.
func (d *DiscordAPI) SendMessage(channelID, content string) (string, error) {
	resp, err := d.client.DiscordSendMessage(context.Background(), &pluginrpcpb.DiscordMessageRequest{
		ChannelId: channelID,
		Content:   content,
	})
	if err != nil {
		return "", err
	}
	return resp.GetMessageId(), nil
}

// SendEmbed sends a Discord embed message to a channel. The embed is passed
// as a generic map so the wire format does not need to model every embed field.
func (d *DiscordAPI) SendEmbed(channelID string, embed map[string]interface{}) (string, error) {
	encoded, err := encodeJSONMap(embed)
	if err != nil {
		return "", fmt.Errorf("encode discord embed: %w", err)
	}
	resp, err := d.client.DiscordSendEmbed(context.Background(), &pluginrpcpb.DiscordMessageRequest{
		ChannelId: channelID,
		EmbedJson: encoded,
	})
	if err != nil {
		return "", err
	}
	return resp.GetMessageId(), nil
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
type ConnectorAPI struct{ client pluginrpcpb.HostAPIClient }

// Call invokes a connector by ID with a JSON envelope. The context deadline
// is translated into a millisecond timeout on the wire.
func (c *ConnectorAPI) Call(ctx context.Context, connectorID string, req *ConnectorInvokeRequest) (*ConnectorInvokeResponse, error) {
	pbReq := &pluginrpcpb.ConnectorCallRequest{ConnectorId: connectorID}
	if req != nil {
		pbReq.V = req.V
		encoded, err := encodeJSONMap(req.Data)
		if err != nil {
			return nil, fmt.Errorf("encode connector data: %w", err)
		}
		pbReq.DataJson = encoded
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining < 0 {
			return nil, context.DeadlineExceeded
		}
		pbReq.TimeoutMs = remaining.Milliseconds()
	}
	resp, err := c.client.ConnectorCall(ctx, pbReq)
	if err != nil {
		return nil, err
	}
	out := &ConnectorInvokeResponse{
		V:     resp.GetV(),
		OK:    resp.GetOk(),
		Error: resp.GetError(),
	}
	if data := resp.GetDataJson(); len(data) > 0 {
		decoded, err := decodeJSONMap(data)
		if err != nil {
			return nil, fmt.Errorf("decode connector response data: %w", err)
		}
		out.Data = decoded
	}
	return out, nil
}

// decodeJSONListOfMaps unmarshals JSON bytes into a []map[string]interface{}.
// Empty bytes return nil. The stricter typing keeps the public Go SDK API
// unchanged from the previous net/rpc implementation.
func decodeJSONListOfMaps(b []byte) ([]map[string]interface{}, error) {
	if len(b) == 0 {
		return nil, nil
	}
	out := []map[string]interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
