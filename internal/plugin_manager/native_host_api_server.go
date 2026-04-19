package plugin_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/rpc"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
	"go.codycody31.dev/squad-aegis/pkg/pluginrpc"
)

const maxHostAPIPayloadSize = 1 << 20 // 1 MiB

// maxConnectorCallTimeout caps the timeout a plugin can request for a
// connector.Call to prevent goroutine starvation.
const maxConnectorCallTimeout = 30 * time.Second

// maxDatabaseValueSize limits the size of a single plugin data value to
// prevent database bloat from malicious or buggy plugins.
const maxDatabaseValueSize = 64 * 1024 // 64 KiB

// maxConcurrentHostAPICalls caps the number of concurrent in-flight HostAPI
// calls per plugin instance to prevent goroutine exhaustion.
const maxConcurrentHostAPICalls = 32

// hostAPIServer wires the in-process *PluginAPIs onto a net/rpc server that
// the plugin subprocess calls into. There is exactly one hostAPIServer per
// loaded subprocess plugin instance; the server closes when Stop fires.
type hostAPIServer struct {
	apis   *PluginAPIs
	server *rpc.Server
	stop   func()
}

// startHostAPIServer registers a new broker listener with the plugin client,
// wires up the HostAPI RPC server on it, and returns the broker ID plus a
// handle that the caller uses to Close() on shutdown. Each hostAPIServer
// gets its own rate limiter, so a compromised plugin cannot starve other
// plugins by burning through a shared token bucket.
func startHostAPIServer(rpcClient *pluginrpc.PluginRPCClient, apis *PluginAPIs, pluginID string) (*hostAPIServer, uint32, error) {
	rpcServer := rpc.NewServer()
	dispatcher := &hostAPIDispatcher{
		pluginID: pluginID,
		apis:     apis,
		limiter:  buildHostAPIRateLimiter(),
		sem:      make(chan struct{}, maxConcurrentHostAPICalls),
	}
	if err := rpcServer.RegisterName("HostAPI", dispatcher); err != nil {
		return nil, 0, fmt.Errorf("failed to register HostAPI service: %w", err)
	}
	brokerID, stop, err := rpcClient.StartHostAPIBroker(rpcServer)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to start host api broker: %w", err)
	}

	return &hostAPIServer{
		apis:   apis,
		server: rpcServer,
		stop:   stop,
	}, brokerID, nil
}

// buildHostAPIRateLimiter constructs a per-instance rate.Limiter configured
// from Plugins.HostAPIRatePerSec / Plugins.HostAPIBurst. A non-positive rate
// disables rate limiting entirely (nil limiter); callers must check for nil
// before calling Allow().
func buildHostAPIRateLimiter() *rate.Limiter {
	if config.Config == nil {
		return rate.NewLimiter(rate.Limit(50), 100)
	}
	r := config.Config.Plugins.HostAPIRatePerSec
	b := config.Config.Plugins.HostAPIBurst
	if r <= 0 || b <= 0 {
		return nil
	}
	return rate.NewLimiter(rate.Limit(r), b)
}

// Close stops the HostAPI listener.
func (s *hostAPIServer) Close() {
	if s == nil || s.stop == nil {
		return
	}
	s.stop()
	s.stop = nil
}

// hostAPIDispatcher routes pluginrpc.HostAPIRequest envelopes to the correct
// in-process API method based on the Target string. Each dispatcher owns its
// own rate limiter so one misbehaving subprocess cannot starve other
// subprocesses from host-side resources.
type hostAPIDispatcher struct {
	pluginID string
	apis     *PluginAPIs
	limiter  *rate.Limiter
	sem      chan struct{} // buffered semaphore limiting concurrent calls
}

// Call is the single dispatch method invoked by subprocess plugins. It
// enforces the per-instance rate limit before routing to the underlying
// PluginAPI method. A rate-limit violation is surfaced as a wire error so
// the plugin can decide to back off.
func (d *hostAPIDispatcher) Call(req pluginrpc.HostAPIRequest, reply *pluginrpc.HostAPIResponse) error {
	if d == nil || d.apis == nil {
		return errors.New("host apis are not configured")
	}
	reply.Payload = nil
	reply.Error = ""

	if len(req.Payload) > maxHostAPIPayloadSize {
		reply.Error = "payload exceeds maximum size"
		return nil
	}

	if d.limiter != nil && !d.limiter.Allow() {
		reply.Error = fmt.Sprintf("host api rate limit exceeded for target %s", req.Target)
		return nil
	}

	// Enforce concurrency limit to prevent goroutine exhaustion from slow calls.
	select {
	case d.sem <- struct{}{}:
		defer func() { <-d.sem }()
	default:
		reply.Error = "too many concurrent host API calls"
		return nil
	}

	out, err := d.dispatch(req)
	if err != nil {
		reply.Error = err.Error()
		return nil
	}
	if out != nil {
		encoded, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			reply.Error = fmt.Sprintf("failed to marshal host api reply: %v", marshalErr)
			return nil
		}
		reply.Payload = encoded
	}
	return nil
}

func (d *hostAPIDispatcher) dispatch(req pluginrpc.HostAPIRequest) (interface{}, error) {
	parts := strings.SplitN(req.Target, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid host api target %q", req.Target)
	}
	api, method := parts[0], parts[1]
	switch api {
	case "log":
		return d.dispatchLog(method, req.Payload)
	case "rcon":
		return d.dispatchRcon(method, req.Payload)
	case "server":
		return d.dispatchServer(method, req.Payload)
	case "database":
		return d.dispatchDatabase(method, req.Payload)
	case "rule":
		return d.dispatchRule(method, req.Payload)
	case "admin":
		return d.dispatchAdmin(method, req.Payload)
	case "event":
		return d.dispatchEvent(method, req.Payload)
	case "discord":
		return d.dispatchDiscord(method, req.Payload)
	case "connector":
		return d.dispatchConnector(method, req.Payload)
	default:
		return nil, fmt.Errorf("unknown host api %q", api)
	}
}

// -- Log --------------------------------------------------------------------

type logArgs struct {
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields,omitempty"`
	Error   string                 `json:"error,omitempty"`
}

func sanitizeLogMessage(msg string) string {
	// Strip control characters that could enable log injection
	msg = strings.ReplaceAll(msg, "\n", "\\n")
	msg = strings.ReplaceAll(msg, "\r", "\\r")
	if len(msg) > 4096 {
		msg = msg[:4096] + "...[truncated]"
	}
	return msg
}

// sanitizeLogFields caps individual field key/value sizes and strips control
// characters to prevent log injection and storage exhaustion.
func sanitizeLogFields(fields map[string]interface{}) map[string]interface{} {
	if fields == nil {
		return nil
	}
	const maxKeyLen = 128
	const maxValueLen = 1024
	sanitized := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		k = sanitizeLogMessage(k)
		if len(k) > maxKeyLen {
			k = k[:maxKeyLen]
		}
		if s, ok := v.(string); ok {
			s = sanitizeLogMessage(s)
			if len(s) > maxValueLen {
				s = s[:maxValueLen] + "...[truncated]"
			}
			v = s
		}
		sanitized[k] = v
	}
	return sanitized
}

func (d *hostAPIDispatcher) dispatchLog(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.LogAPI == nil {
		return nil, errors.New("log api is unavailable")
	}
	var args logArgs
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, fmt.Errorf("invalid log args: %w", err)
		}
	}
	args.Message = sanitizeLogMessage(args.Message)
	if len(args.Fields) > 32 {
		return nil, fmt.Errorf("log fields exceed maximum count of 32")
	}
	args.Fields = sanitizeLogFields(args.Fields)
	switch method {
	case "Info":
		d.apis.LogAPI.Info(args.Message, args.Fields)
	case "Warn":
		d.apis.LogAPI.Warn(args.Message, args.Fields)
	case "Error":
		var err error
		if args.Error != "" {
			err = errors.New(args.Error)
		}
		d.apis.LogAPI.Error(args.Message, err, args.Fields)
	case "Debug":
		d.apis.LogAPI.Debug(args.Message, args.Fields)
	default:
		return nil, fmt.Errorf("unknown log method %q", method)
	}
	return nil, nil
}

// -- Rcon -------------------------------------------------------------------

type rconCommandArgs struct {
	Command string `json:"command"`
}

type rconCommandReply struct {
	Response string `json:"response"`
}

type rconBroadcastArgs struct {
	Message string `json:"message"`
}

type rconWarnPlayerArgs struct {
	PlayerID string `json:"player_id"`
	Message  string `json:"message"`
}

type rconKickArgs struct {
	PlayerID string `json:"player_id"`
	Reason   string `json:"reason"`
}

type rconBanArgs struct {
	PlayerID  string                 `json:"player_id"`
	Reason    string                 `json:"reason"`
	Duration  time.Duration          `json:"duration"`
	EventID   string                 `json:"event_id,omitempty"`
	EventType string                 `json:"event_type,omitempty"`
	RuleID    *string                `json:"rule_id,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
}

type banResultReply struct {
	BanID string `json:"ban_id"`
}

type rconRemoveSquadArgs struct {
	PlayerID string `json:"player_id"`
}

// validatePlayerID checks that a player ID is a plausible Steam64 numeric
// string, preventing injection of RCON arguments via crafted IDs.
func validatePlayerID(playerID string) error {
	if playerID == "" {
		return fmt.Errorf("player_id must not be empty")
	}
	if len(playerID) > 20 {
		return fmt.Errorf("player_id exceeds maximum length")
	}
	for _, c := range playerID {
		if c < '0' || c > '9' {
			return fmt.Errorf("player_id contains invalid character %q", c)
		}
	}
	return nil
}

func (d *hostAPIDispatcher) dispatchRcon(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.RconAPI == nil {
		return nil, errors.New("rcon api is unavailable")
	}
	switch method {
	case "SendCommand":
		var args rconCommandArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		log.Warn().Str("plugin_id", d.pluginID).Str("command", args.Command).Msg("Plugin executing raw RCON command via SendCommand")
		resp, err := d.apis.RconAPI.SendCommand(args.Command)
		if err != nil {
			return nil, err
		}
		return rconCommandReply{Response: resp}, nil
	case "Broadcast":
		var args rconBroadcastArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.Broadcast(args.Message)
	case "SendWarningToPlayer":
		var args rconWarnPlayerArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.SendWarningToPlayer(args.PlayerID, args.Message)
	case "KickPlayer":
		var args rconKickArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.KickPlayer(args.PlayerID, args.Reason)
	case "BanPlayer":
		var args rconBanArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.BanPlayer(args.PlayerID, args.Reason, args.Duration)
	case "BanWithEvidence":
		var args rconBanArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		banID, err := d.apis.RconAPI.BanWithEvidence(args.PlayerID, args.Reason, args.Duration, args.EventID, args.EventType)
		if err != nil {
			return nil, err
		}
		return banResultReply{BanID: banID}, nil
	case "WarnPlayerWithRule":
		var args rconBanArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.WarnPlayerWithRule(args.PlayerID, args.Reason, args.RuleID)
	case "KickPlayerWithRule":
		var args rconBanArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.KickPlayerWithRule(args.PlayerID, args.Reason, args.RuleID)
	case "BanPlayerWithRule":
		var args rconBanArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.BanPlayerWithRule(args.PlayerID, args.Reason, args.Duration, args.RuleID)
	case "BanWithEvidenceAndRule":
		var args rconBanArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		banID, err := d.apis.RconAPI.BanWithEvidenceAndRule(args.PlayerID, args.Reason, args.Duration, args.EventID, args.EventType, args.RuleID)
		if err != nil {
			return nil, err
		}
		return banResultReply{BanID: banID}, nil
	case "BanWithEvidenceAndRuleAndMetadata":
		var args rconBanArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		banID, err := d.apis.RconAPI.BanWithEvidenceAndRuleAndMetadata(args.PlayerID, args.Reason, args.Duration, args.EventID, args.EventType, args.RuleID, args.Metadata)
		if err != nil {
			return nil, err
		}
		return banResultReply{BanID: banID}, nil
	case "RemovePlayerFromSquad":
		var args rconRemoveSquadArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.RemovePlayerFromSquad(args.PlayerID)
	case "RemovePlayerFromSquadById":
		var args rconRemoveSquadArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		if err := validatePlayerID(args.PlayerID); err != nil {
			return nil, err
		}
		return nil, d.apis.RconAPI.RemovePlayerFromSquadById(args.PlayerID)
	default:
		return nil, fmt.Errorf("unknown rcon method %q", method)
	}
}

// -- Server -----------------------------------------------------------------

func (d *hostAPIDispatcher) dispatchServer(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.ServerAPI == nil {
		return nil, errors.New("server api is unavailable")
	}
	switch method {
	case "GetServerID":
		return d.apis.ServerAPI.GetServerID().String(), nil
	case "GetServerInfo":
		info, err := d.apis.ServerAPI.GetServerInfo()
		if err != nil {
			return nil, err
		}
		return info, nil
	case "GetPlayers":
		players, err := d.apis.ServerAPI.GetPlayers()
		if err != nil {
			return nil, err
		}
		return players, nil
	case "GetAdmins":
		admins, err := d.apis.ServerAPI.GetAdmins()
		if err != nil {
			return nil, err
		}
		return admins, nil
	case "GetSquads":
		squads, err := d.apis.ServerAPI.GetSquads()
		if err != nil {
			return nil, err
		}
		return squads, nil
	default:
		return nil, fmt.Errorf("unknown server method %q", method)
	}
}

// -- Database ---------------------------------------------------------------

type dbArgs struct {
	Key   string `json:"key"`
	Value string `json:"value,omitempty"`
}

type dbReply struct {
	Value string `json:"value"`
}

func validatePluginDataKey(key string) error {
	if key == "" {
		return fmt.Errorf("database key must not be empty")
	}
	if len(key) > 256 {
		return fmt.Errorf("database key exceeds maximum length of 256")
	}
	return nil
}

func (d *hostAPIDispatcher) dispatchDatabase(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.DatabaseAPI == nil {
		return nil, errors.New("database api is unavailable")
	}
	var args dbArgs
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
	}
	switch method {
	case "GetPluginData":
		if err := validatePluginDataKey(args.Key); err != nil {
			return nil, err
		}
		value, err := d.apis.DatabaseAPI.GetPluginData(args.Key)
		if err != nil {
			return nil, err
		}
		return dbReply{Value: value}, nil
	case "SetPluginData":
		if err := validatePluginDataKey(args.Key); err != nil {
			return nil, err
		}
		if len(args.Value) > maxDatabaseValueSize {
			return nil, fmt.Errorf("database value exceeds maximum size of %d bytes", maxDatabaseValueSize)
		}
		return nil, d.apis.DatabaseAPI.SetPluginData(args.Key, args.Value)
	case "DeletePluginData":
		if err := validatePluginDataKey(args.Key); err != nil {
			return nil, err
		}
		return nil, d.apis.DatabaseAPI.DeletePluginData(args.Key)
	default:
		return nil, fmt.Errorf("unknown database method %q", method)
	}
}

// -- Rule -------------------------------------------------------------------

type listRulesArgs struct {
	ParentRuleID *string `json:"parent_rule_id,omitempty"`
}

type listRuleActionsArgs struct {
	RuleID string `json:"rule_id"`
}

func (d *hostAPIDispatcher) dispatchRule(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.RuleAPI == nil {
		return nil, errors.New("rule api is unavailable")
	}
	switch method {
	case "ListServerRules":
		var args listRulesArgs
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &args); err != nil {
				return nil, err
			}
		}
		return d.apis.RuleAPI.ListServerRules(args.ParentRuleID)
	case "ListServerRuleActions":
		var args listRuleActionsArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		return d.apis.RuleAPI.ListServerRuleActions(args.RuleID)
	default:
		return nil, fmt.Errorf("unknown rule method %q", method)
	}
}

// -- Admin ------------------------------------------------------------------

type addTempAdminArgs struct {
	PlayerID  string     `json:"player_id"`
	RoleName  string     `json:"role_name"`
	Notes     string     `json:"notes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type removeTempAdminArgs struct {
	PlayerID string `json:"player_id"`
	RoleName string `json:"role_name,omitempty"`
	Notes    string `json:"notes"`
}

type playerIDArgs struct {
	PlayerID string `json:"player_id"`
}

func (d *hostAPIDispatcher) dispatchAdmin(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.AdminAPI == nil {
		return nil, errors.New("admin api is unavailable")
	}
	switch method {
	case "AddTemporaryAdmin":
		var args addTempAdminArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		// Enforce maximum TTL for plugin-granted admin roles
		const maxAdminTTL = 24 * time.Hour
		if args.ExpiresAt == nil {
			defaultExpiry := time.Now().Add(maxAdminTTL)
			args.ExpiresAt = &defaultExpiry
		} else if time.Until(*args.ExpiresAt) > maxAdminTTL {
			clamped := time.Now().Add(maxAdminTTL)
			args.ExpiresAt = &clamped
		}
		log.Info().Str("plugin_id", d.pluginID).Str("player_id", args.PlayerID).Str("role", args.RoleName).Time("expires_at", *args.ExpiresAt).Msg("Plugin granting temporary admin")
		return nil, d.apis.AdminAPI.AddTemporaryAdmin(args.PlayerID, args.RoleName, args.Notes, args.ExpiresAt)
	case "RemoveTemporaryAdmin":
		var args removeTempAdminArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		return nil, d.apis.AdminAPI.RemoveTemporaryAdmin(args.PlayerID, args.Notes)
	case "RemoveTemporaryAdminRole":
		var args removeTempAdminArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		return nil, d.apis.AdminAPI.RemoveTemporaryAdminRole(args.PlayerID, args.RoleName, args.Notes)
	case "GetPlayerAdminStatus":
		var args playerIDArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		return d.apis.AdminAPI.GetPlayerAdminStatus(args.PlayerID)
	case "ListTemporaryAdmins":
		return d.apis.AdminAPI.ListTemporaryAdmins()
	default:
		return nil, fmt.Errorf("unknown admin method %q", method)
	}
}

// -- Event ------------------------------------------------------------------

type publishEventArgs struct {
	EventType string                 `json:"event_type"`
	Data      map[string]interface{} `json:"data,omitempty"`
	Raw       string                 `json:"raw,omitempty"`
}

func (d *hostAPIDispatcher) dispatchEvent(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.EventAPI == nil {
		return nil, errors.New("event api is unavailable")
	}
	switch method {
	case "PublishEvent":
		var args publishEventArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		// Namespace plugin-emitted events with the plugin ID to prevent both
		// impersonation of system events and cross-plugin event spoofing. We
		// prepend unconditionally rather than allowing the plugin-supplied
		// event type to pass through when it already starts with the expected
		// prefix: plugin IDs may contain underscores (e.g. "discord_teamkill"),
		// so a HasPrefix check would let plugin "foo" publish a
		// "PLUGIN_foo_bar_x" event indistinguishable from plugin "foo_bar"'s
		// "PLUGIN_foo_bar_x". Always prepending makes the emitting plugin
		// unambiguous: a plugin that double-namespaces just produces
		// "PLUGIN_<id>_PLUGIN_<id>_<suffix>", still attributable to <id>.
		args.EventType = "PLUGIN_" + d.pluginID + "_" + args.EventType
		return nil, d.apis.EventAPI.PublishEvent(args.EventType, args.Data, args.Raw)
	default:
		return nil, fmt.Errorf("unknown event method %q", method)
	}
}

// -- Discord ----------------------------------------------------------------

type discordMessageArgs struct {
	ChannelID string                 `json:"channel_id"`
	Content   string                 `json:"content,omitempty"`
	Embed     map[string]interface{} `json:"embed,omitempty"`
}

type discordMessageReply struct {
	MessageID string `json:"message_id"`
}

func (d *hostAPIDispatcher) dispatchDiscord(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.DiscordAPI == nil {
		return nil, errors.New("discord api is unavailable")
	}
	var args discordMessageArgs
	if len(payload) > 0 {
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
	}
	switch method {
	case "SendMessage":
		log.Info().Str("plugin_id", d.pluginID).Str("channel_id", args.ChannelID).Msg("Plugin sending Discord message")
		id, err := d.apis.DiscordAPI.SendMessage(args.ChannelID, args.Content)
		if err != nil {
			return nil, err
		}
		return discordMessageReply{MessageID: id}, nil
	case "SendEmbed":
		log.Info().Str("plugin_id", d.pluginID).Str("channel_id", args.ChannelID).Msg("Plugin sending Discord embed")
		embed, err := mapToDiscordEmbed(args.Embed)
		if err != nil {
			return nil, err
		}
		id, err := d.apis.DiscordAPI.SendEmbed(args.ChannelID, embed)
		if err != nil {
			return nil, err
		}
		return discordMessageReply{MessageID: id}, nil
	default:
		return nil, fmt.Errorf("unknown discord method %q", method)
	}
}

// mapToDiscordEmbed converts a generic map into a typed DiscordEmbed via
// JSON round-trip so the dispatch layer doesn't need to know every embed field.
func mapToDiscordEmbed(in map[string]interface{}) (*DiscordEmbed, error) {
	if in == nil {
		return nil, nil
	}
	raw, err := json.Marshal(in)
	if err != nil {
		return nil, err
	}
	var embed DiscordEmbed
	if err := json.Unmarshal(raw, &embed); err != nil {
		return nil, err
	}
	return &embed, nil
}

// -- Connector --------------------------------------------------------------

type connectorCallArgs struct {
	ConnectorID string                 `json:"connector_id"`
	V           string                 `json:"v"`
	Data        map[string]interface{} `json:"data,omitempty"`
	TimeoutMs   int64                  `json:"timeout_ms,omitempty"`
}

func (d *hostAPIDispatcher) dispatchConnector(method string, payload json.RawMessage) (interface{}, error) {
	if d.apis.ConnectorAPI == nil {
		return nil, errors.New("connector api is unavailable")
	}
	switch method {
	case "Call":
		var args connectorCallArgs
		if err := json.Unmarshal(payload, &args); err != nil {
			return nil, err
		}
		ctx := context.Background()
		if args.TimeoutMs > 0 {
			timeout := time.Duration(args.TimeoutMs) * time.Millisecond
			if timeout > maxConnectorCallTimeout {
				timeout = maxConnectorCallTimeout
			}
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}
		req := &ConnectorInvokeRequest{V: args.V, Data: args.Data}
		return d.apis.ConnectorAPI.Call(ctx, args.ConnectorID, req)
	default:
		return nil, fmt.Errorf("unknown connector method %q", method)
	}
}
