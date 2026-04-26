package plugin_manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
	"go.codycody31.dev/squad-aegis/internal/shared/utils"
	"go.codycody31.dev/squad-aegis/pkg/pluginrpc"
	pluginrpcpb "go.codycody31.dev/squad-aegis/pkg/pluginrpc/proto"
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

// hostAPIServer wires the in-process *PluginAPIs onto a gRPC server that
// the plugin subprocess calls into. There is exactly one hostAPIServer per
// loaded subprocess plugin instance; the server closes when Stop fires.
type hostAPIServer struct {
	apis *PluginAPIs
	stop func()
}

// startHostAPIServer registers a new broker listener with the plugin client,
// wires up the HostAPI gRPC server on it, and returns the broker ID plus a
// handle that the caller uses to Close() on shutdown. Each hostAPIServer
// gets its own rate limiter, so a compromised plugin cannot starve other
// plugins by burning through a shared token bucket.
func startHostAPIServer(rpcClient *pluginrpc.PluginGRPCClient, apis *PluginAPIs, pluginID string) (*hostAPIServer, uint32, error) {
	dispatcher := &hostAPIDispatcher{
		pluginID: pluginID,
		apis:     apis,
		limiter:  buildHostAPIRateLimiter(),
		sem:      make(chan struct{}, maxConcurrentHostAPICalls),
	}
	brokerID, stop, err := rpcClient.StartHostAPIBroker(func(s *grpc.Server) {
		pluginrpcpb.RegisterHostAPIServer(s, dispatcher)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to start host api broker: %w", err)
	}

	return &hostAPIServer{
		apis: apis,
		stop: stop,
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

// hostAPIDispatcher implements the HostAPI gRPC service. Each loaded plugin
// instance has its own dispatcher with its own rate limiter and concurrency
// semaphore so one misbehaving subprocess cannot starve others.
type hostAPIDispatcher struct {
	pluginrpcpb.UnimplementedHostAPIServer

	pluginID string
	apis     *PluginAPIs
	limiter  *rate.Limiter
	sem      chan struct{} // buffered semaphore limiting concurrent calls
}

// admit applies the rate limiter and concurrency semaphore for an incoming
// host API call. It returns a release function (or nil on rejection) and an
// error to report back to the plugin.
func (d *hostAPIDispatcher) admit() (func(), error) {
	if d == nil || d.apis == nil {
		return nil, errors.New("host apis are not configured")
	}
	if d.limiter != nil && !d.limiter.Allow() {
		return nil, errors.New("host api rate limit exceeded")
	}
	select {
	case d.sem <- struct{}{}:
		return func() { <-d.sem }, nil
	default:
		return nil, errors.New("too many concurrent host API calls")
	}
}

// checkPayload rejects oversized request payloads before the dispatcher
// wastes work on them. Each gRPC method calls this with the largest payload
// fragment it carries (typically a JSON-encoded map).
func checkPayload(b []byte) error {
	if len(b) > maxHostAPIPayloadSize {
		return errors.New("payload exceeds maximum size")
	}
	return nil
}

// -- Log --------------------------------------------------------------------

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

// prepareLogArgs decodes and sanitizes a LogRequest's payload. Returns the
// decoded fields ready to forward to the host LogAPI.
func (d *hostAPIDispatcher) prepareLogArgs(req *pluginrpcpb.LogRequest) (string, map[string]interface{}, error) {
	if d.apis.LogAPI == nil {
		return "", nil, errors.New("log api is unavailable")
	}
	if err := checkPayload(req.GetFieldsJson()); err != nil {
		return "", nil, err
	}
	fields, err := decodeJSONMap(req.GetFieldsJson())
	if err != nil {
		return "", nil, fmt.Errorf("invalid log fields: %w", err)
	}
	if len(fields) > 32 {
		return "", nil, fmt.Errorf("log fields exceed maximum count of 32")
	}
	return sanitizeLogMessage(req.GetMessage()), sanitizeLogFields(fields), nil
}

func (d *hostAPIDispatcher) LogInfo(_ context.Context, req *pluginrpcpb.LogRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	msg, fields, err := d.prepareLogArgs(req)
	if err != nil {
		return nil, err
	}
	d.apis.LogAPI.Info(msg, fields)
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) LogWarn(_ context.Context, req *pluginrpcpb.LogRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	msg, fields, err := d.prepareLogArgs(req)
	if err != nil {
		return nil, err
	}
	d.apis.LogAPI.Warn(msg, fields)
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) LogError(_ context.Context, req *pluginrpcpb.LogRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	msg, fields, err := d.prepareLogArgs(req)
	if err != nil {
		return nil, err
	}
	var inner error
	if e := req.GetError(); e != "" {
		inner = errors.New(e)
	}
	d.apis.LogAPI.Error(msg, inner, fields)
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) LogDebug(_ context.Context, req *pluginrpcpb.LogRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	msg, fields, err := d.prepareLogArgs(req)
	if err != nil {
		return nil, err
	}
	d.apis.LogAPI.Debug(msg, fields)
	return &pluginrpcpb.Empty{}, nil
}

// -- Rcon -------------------------------------------------------------------

// validatePlayerID checks that a player ID is either a Steam64 numeric string
// or a 32-char hex EOS ID, preventing injection of RCON arguments via crafted
// IDs while matching the identifier formats the Squad server actually uses.
func validatePlayerID(playerID string) error {
	id := strings.TrimSpace(playerID)
	if id == "" {
		return fmt.Errorf("player_id must not be empty")
	}
	if utils.IsSteamID(id) || utils.IsEOSID(id) {
		return nil
	}
	return fmt.Errorf("player_id must be a Steam64 or 32-char hex EOS ID")
}

func (d *hostAPIDispatcher) checkRcon() error {
	if d.apis.RconAPI == nil {
		return errors.New("rcon api is unavailable")
	}
	return nil
}

// ruleIDPtr extracts the optional rule_id from a RconBanRequest; it returns
// nil when the rule_id_set bit is false so callers see the same *string
// shape the in-process API expects.
func ruleIDPtr(req *pluginrpcpb.RconBanRequest) *string {
	if !req.GetRuleIdSet() {
		return nil
	}
	v := req.GetRuleId()
	return &v
}

func (d *hostAPIDispatcher) RconSendCommand(_ context.Context, req *pluginrpcpb.RconCommandRequest) (*pluginrpcpb.RconCommandResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	log.Warn().Str("plugin_id", d.pluginID).Str("command", req.GetCommand()).Msg("Plugin executing raw RCON command via SendCommand")
	resp, err := d.apis.RconAPI.SendCommand(req.GetCommand())
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.RconCommandResponse{Response: resp}, nil
}

func (d *hostAPIDispatcher) RconBroadcast(_ context.Context, req *pluginrpcpb.RconBroadcastRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.Broadcast(req.GetMessage()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconSendWarningToPlayer(_ context.Context, req *pluginrpcpb.RconWarnPlayerRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.SendWarningToPlayer(req.GetPlayerId(), req.GetMessage()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconKickPlayer(_ context.Context, req *pluginrpcpb.RconKickRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.KickPlayer(req.GetPlayerId(), req.GetReason()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconBanPlayer(_ context.Context, req *pluginrpcpb.RconBanRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.BanPlayer(req.GetPlayerId(), req.GetReason(), time.Duration(req.GetDurationNs())); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconBanWithEvidence(_ context.Context, req *pluginrpcpb.RconBanRequest) (*pluginrpcpb.BanResultResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	banID, err := d.apis.RconAPI.BanWithEvidence(req.GetPlayerId(), req.GetReason(), time.Duration(req.GetDurationNs()), req.GetEventId(), req.GetEventType())
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.BanResultResponse{BanId: banID}, nil
}

func (d *hostAPIDispatcher) RconWarnPlayerWithRule(_ context.Context, req *pluginrpcpb.RconBanRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.WarnPlayerWithRule(req.GetPlayerId(), req.GetReason(), ruleIDPtr(req)); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconKickPlayerWithRule(_ context.Context, req *pluginrpcpb.RconBanRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.KickPlayerWithRule(req.GetPlayerId(), req.GetReason(), ruleIDPtr(req)); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconBanPlayerWithRule(_ context.Context, req *pluginrpcpb.RconBanRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.BanPlayerWithRule(req.GetPlayerId(), req.GetReason(), time.Duration(req.GetDurationNs()), ruleIDPtr(req)); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconBanWithEvidenceAndRule(_ context.Context, req *pluginrpcpb.RconBanRequest) (*pluginrpcpb.BanResultResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	banID, err := d.apis.RconAPI.BanWithEvidenceAndRule(req.GetPlayerId(), req.GetReason(), time.Duration(req.GetDurationNs()), req.GetEventId(), req.GetEventType(), ruleIDPtr(req))
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.BanResultResponse{BanId: banID}, nil
}

func (d *hostAPIDispatcher) RconBanWithEvidenceAndRuleAndMetadata(_ context.Context, req *pluginrpcpb.RconBanRequest) (*pluginrpcpb.BanResultResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := checkPayload(req.GetMetadataJson()); err != nil {
		return nil, err
	}
	metadata, err := decodeJSONMap(req.GetMetadataJson())
	if err != nil {
		return nil, fmt.Errorf("decode metadata: %w", err)
	}
	banID, err := d.apis.RconAPI.BanWithEvidenceAndRuleAndMetadata(req.GetPlayerId(), req.GetReason(), time.Duration(req.GetDurationNs()), req.GetEventId(), req.GetEventType(), ruleIDPtr(req), metadata)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.BanResultResponse{BanId: banID}, nil
}

func (d *hostAPIDispatcher) RconRemovePlayerFromSquad(_ context.Context, req *pluginrpcpb.RconRemoveSquadRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.RemovePlayerFromSquad(req.GetPlayerId()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) RconRemovePlayerFromSquadById(_ context.Context, req *pluginrpcpb.RconRemoveSquadRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRcon(); err != nil {
		return nil, err
	}
	if err := validatePlayerID(req.GetPlayerId()); err != nil {
		return nil, err
	}
	if err := d.apis.RconAPI.RemovePlayerFromSquadById(req.GetPlayerId()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// -- Server -----------------------------------------------------------------

func (d *hostAPIDispatcher) checkServer() error {
	if d.apis.ServerAPI == nil {
		return errors.New("server api is unavailable")
	}
	return nil
}

func encodeJSONReply(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	return json.Marshal(v)
}

func (d *hostAPIDispatcher) ServerGetServerID(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.StringResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkServer(); err != nil {
		return nil, err
	}
	return &pluginrpcpb.StringResponse{Value: d.apis.ServerAPI.GetServerID().String()}, nil
}

func (d *hostAPIDispatcher) ServerGetServerInfo(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkServer(); err != nil {
		return nil, err
	}
	info, err := d.apis.ServerAPI.GetServerInfo()
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(info)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

func (d *hostAPIDispatcher) ServerGetPlayers(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkServer(); err != nil {
		return nil, err
	}
	players, err := d.apis.ServerAPI.GetPlayers()
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(players)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

func (d *hostAPIDispatcher) ServerGetAdmins(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkServer(); err != nil {
		return nil, err
	}
	admins, err := d.apis.ServerAPI.GetAdmins()
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(admins)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

func (d *hostAPIDispatcher) ServerGetSquads(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkServer(); err != nil {
		return nil, err
	}
	squads, err := d.apis.ServerAPI.GetSquads()
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(squads)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

// -- Database ---------------------------------------------------------------

func validatePluginDataKey(key string) error {
	if key == "" {
		return fmt.Errorf("database key must not be empty")
	}
	if len(key) > 256 {
		return fmt.Errorf("database key exceeds maximum length of 256")
	}
	return nil
}

func (d *hostAPIDispatcher) checkDatabase() error {
	if d.apis.DatabaseAPI == nil {
		return errors.New("database api is unavailable")
	}
	return nil
}

func (d *hostAPIDispatcher) DatabaseGetPluginData(_ context.Context, req *pluginrpcpb.DatabaseRequest) (*pluginrpcpb.DatabaseResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkDatabase(); err != nil {
		return nil, err
	}
	if err := validatePluginDataKey(req.GetKey()); err != nil {
		return nil, err
	}
	value, err := d.apis.DatabaseAPI.GetPluginData(req.GetKey())
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.DatabaseResponse{Value: value}, nil
}

func (d *hostAPIDispatcher) DatabaseSetPluginData(_ context.Context, req *pluginrpcpb.DatabaseRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkDatabase(); err != nil {
		return nil, err
	}
	if err := validatePluginDataKey(req.GetKey()); err != nil {
		return nil, err
	}
	if len(req.GetValue()) > maxDatabaseValueSize {
		return nil, fmt.Errorf("database value exceeds maximum size of %d bytes", maxDatabaseValueSize)
	}
	if err := d.apis.DatabaseAPI.SetPluginData(req.GetKey(), req.GetValue()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) DatabaseDeletePluginData(_ context.Context, req *pluginrpcpb.DatabaseRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkDatabase(); err != nil {
		return nil, err
	}
	if err := validatePluginDataKey(req.GetKey()); err != nil {
		return nil, err
	}
	if err := d.apis.DatabaseAPI.DeletePluginData(req.GetKey()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// -- Rule -------------------------------------------------------------------

func (d *hostAPIDispatcher) checkRule() error {
	if d.apis.RuleAPI == nil {
		return errors.New("rule api is unavailable")
	}
	return nil
}

func (d *hostAPIDispatcher) RuleListServerRules(_ context.Context, req *pluginrpcpb.ListRulesRequest) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRule(); err != nil {
		return nil, err
	}
	var parent *string
	if req.GetParentRuleIdSet() {
		v := req.GetParentRuleId()
		parent = &v
	}
	rules, err := d.apis.RuleAPI.ListServerRules(parent)
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(rules)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

func (d *hostAPIDispatcher) RuleListServerRuleActions(_ context.Context, req *pluginrpcpb.ListRuleActionsRequest) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkRule(); err != nil {
		return nil, err
	}
	actions, err := d.apis.RuleAPI.ListServerRuleActions(req.GetRuleId())
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(actions)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

// -- Admin ------------------------------------------------------------------

func (d *hostAPIDispatcher) checkAdmin() error {
	if d.apis.AdminAPI == nil {
		return errors.New("admin api is unavailable")
	}
	return nil
}

func (d *hostAPIDispatcher) AdminAddTemporaryAdmin(_ context.Context, req *pluginrpcpb.AddTempAdminRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkAdmin(); err != nil {
		return nil, err
	}
	const maxAdminTTL = 24 * time.Hour
	var expiresAt *time.Time
	if ts := req.GetExpiresAt(); ts != nil {
		t := ts.AsTime()
		expiresAt = &t
	}
	if expiresAt == nil {
		defaultExpiry := time.Now().Add(maxAdminTTL)
		expiresAt = &defaultExpiry
	} else if time.Until(*expiresAt) > maxAdminTTL {
		clamped := time.Now().Add(maxAdminTTL)
		expiresAt = &clamped
	}
	log.Info().Str("plugin_id", d.pluginID).Str("player_id", req.GetPlayerId()).Str("role", req.GetRoleName()).Time("expires_at", *expiresAt).Msg("Plugin granting temporary admin")
	if err := d.apis.AdminAPI.AddTemporaryAdmin(req.GetPlayerId(), req.GetRoleName(), req.GetNotes(), expiresAt); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) AdminRemoveTemporaryAdmin(_ context.Context, req *pluginrpcpb.RemoveTempAdminRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkAdmin(); err != nil {
		return nil, err
	}
	if err := d.apis.AdminAPI.RemoveTemporaryAdmin(req.GetPlayerId(), req.GetNotes()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) AdminRemoveTemporaryAdminRole(_ context.Context, req *pluginrpcpb.RemoveTempAdminRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkAdmin(); err != nil {
		return nil, err
	}
	if err := d.apis.AdminAPI.RemoveTemporaryAdminRole(req.GetPlayerId(), req.GetRoleName(), req.GetNotes()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

func (d *hostAPIDispatcher) AdminGetPlayerAdminStatus(_ context.Context, req *pluginrpcpb.PlayerIDRequest) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkAdmin(); err != nil {
		return nil, err
	}
	status, err := d.apis.AdminAPI.GetPlayerAdminStatus(req.GetPlayerId())
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(status)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

func (d *hostAPIDispatcher) AdminListTemporaryAdmins(_ context.Context, _ *pluginrpcpb.Empty) (*pluginrpcpb.JSONResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkAdmin(); err != nil {
		return nil, err
	}
	admins, err := d.apis.AdminAPI.ListTemporaryAdmins()
	if err != nil {
		return nil, err
	}
	encoded, err := encodeJSONReply(admins)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.JSONResponse{DataJson: encoded}, nil
}

// -- Event ------------------------------------------------------------------

const (
	maxPublishEventDataSize = 64 * 1024
	maxPublishEventRawSize  = 8 * 1024
)

func (d *hostAPIDispatcher) checkEvent() error {
	if d.apis.EventAPI == nil {
		return errors.New("event api is unavailable")
	}
	return nil
}

func (d *hostAPIDispatcher) EventPublishEvent(_ context.Context, req *pluginrpcpb.PublishEventRequest) (*pluginrpcpb.Empty, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkEvent(); err != nil {
		return nil, err
	}
	if err := checkPayload(req.GetDataJson()); err != nil {
		return nil, err
	}
	data, err := decodeJSONMap(req.GetDataJson())
	if err != nil {
		return nil, fmt.Errorf("decode event data: %w", err)
	}
	if len(data) > 0 {
		encoded, err := json.Marshal(data)
		if err != nil {
			return nil, fmt.Errorf("failed to encode event data: %w", err)
		}
		if len(encoded) > maxPublishEventDataSize {
			return nil, fmt.Errorf("event data exceeds maximum size of %d bytes", maxPublishEventDataSize)
		}
	}
	if len(req.GetRaw()) > maxPublishEventRawSize {
		return nil, fmt.Errorf("event raw exceeds maximum size of %d bytes", maxPublishEventRawSize)
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
	eventType := "PLUGIN_" + d.pluginID + "_" + req.GetEventType()
	if err := d.apis.EventAPI.PublishEvent(eventType, data, req.GetRaw()); err != nil {
		return nil, err
	}
	return &pluginrpcpb.Empty{}, nil
}

// -- Discord ----------------------------------------------------------------

// discordChannelAllowlister is implemented by DiscordAPI wrappers that scope
// a plugin instance to an operator-configured set of channel IDs (per-instance
// _aegis_discord_channels). A nil/empty result preserves the legacy behavior
// of allowing any channel the bot can see.
type discordChannelAllowlister interface {
	allowedDiscordChannels() []string
}

func (d *hostAPIDispatcher) checkDiscord() error {
	if d.apis.DiscordAPI == nil {
		return errors.New("discord api is unavailable")
	}
	return nil
}

func (d *hostAPIDispatcher) checkDiscordChannel(channelID string) error {
	allow, ok := d.apis.DiscordAPI.(discordChannelAllowlister)
	if !ok {
		return nil
	}
	allowed := allow.allowedDiscordChannels()
	if len(allowed) == 0 {
		return nil
	}
	for _, id := range allowed {
		if id == channelID {
			return nil
		}
	}
	return fmt.Errorf("discord channel %s is not in this plugin instance's allowlist", channelID)
}

func (d *hostAPIDispatcher) DiscordSendMessage(_ context.Context, req *pluginrpcpb.DiscordMessageRequest) (*pluginrpcpb.DiscordMessageResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkDiscord(); err != nil {
		return nil, err
	}
	if err := d.checkDiscordChannel(req.GetChannelId()); err != nil {
		return nil, err
	}
	log.Info().Str("plugin_id", d.pluginID).Str("channel_id", req.GetChannelId()).Msg("Plugin sending Discord message")
	id, err := d.apis.DiscordAPI.SendMessage(req.GetChannelId(), req.GetContent())
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.DiscordMessageResponse{MessageId: id}, nil
}

func (d *hostAPIDispatcher) DiscordSendEmbed(_ context.Context, req *pluginrpcpb.DiscordMessageRequest) (*pluginrpcpb.DiscordMessageResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if err := d.checkDiscord(); err != nil {
		return nil, err
	}
	if err := d.checkDiscordChannel(req.GetChannelId()); err != nil {
		return nil, err
	}
	if err := checkPayload(req.GetEmbedJson()); err != nil {
		return nil, err
	}
	embedMap, err := decodeJSONMap(req.GetEmbedJson())
	if err != nil {
		return nil, fmt.Errorf("decode embed: %w", err)
	}
	embed, err := mapToDiscordEmbed(embedMap)
	if err != nil {
		return nil, err
	}
	if embed == nil {
		return nil, errors.New("discord embed is required")
	}
	log.Info().Str("plugin_id", d.pluginID).Str("channel_id", req.GetChannelId()).Msg("Plugin sending Discord embed")
	id, err := d.apis.DiscordAPI.SendEmbed(req.GetChannelId(), embed)
	if err != nil {
		return nil, err
	}
	return &pluginrpcpb.DiscordMessageResponse{MessageId: id}, nil
}

// mapToDiscordEmbed converts a generic map into a typed DiscordEmbed via
// JSON round-trip so the dispatch layer doesn't need to know every embed field.
func mapToDiscordEmbed(in map[string]interface{}) (*DiscordEmbed, error) {
	if len(in) == 0 {
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

func (d *hostAPIDispatcher) ConnectorCall(_ context.Context, req *pluginrpcpb.ConnectorCallRequest) (*pluginrpcpb.ConnectorCallResponse, error) {
	release, err := d.admit()
	if err != nil {
		return nil, err
	}
	defer release()
	if d.apis.ConnectorAPI == nil {
		return nil, errors.New("connector api is unavailable")
	}
	if err := checkPayload(req.GetDataJson()); err != nil {
		return nil, err
	}
	data, err := decodeJSONMap(req.GetDataJson())
	if err != nil {
		return nil, fmt.Errorf("decode connector data: %w", err)
	}
	ctx := context.Background()
	if req.GetTimeoutMs() > 0 {
		timeout := time.Duration(req.GetTimeoutMs()) * time.Millisecond
		if timeout > maxConnectorCallTimeout {
			timeout = maxConnectorCallTimeout
		}
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	invokeReq := &ConnectorInvokeRequest{V: req.GetV(), Data: data}
	resp, err := d.apis.ConnectorAPI.Call(ctx, req.GetConnectorId(), invokeReq)
	if err != nil {
		return nil, err
	}
	out := &pluginrpcpb.ConnectorCallResponse{}
	if resp != nil {
		out.V = resp.V
		out.Ok = resp.OK
		out.Error = resp.Error
		if len(resp.Data) > 0 {
			encoded, err := json.Marshal(resp.Data)
			if err != nil {
				return nil, err
			}
			out.DataJson = encoded
		}
	}
	return out, nil
}

// decodeJSONMap is the host-side mirror of the SDK helper. Empty bytes
// return an empty (non-nil) map so dispatch callers can iterate without nil
// checks.
func decodeJSONMap(b []byte) (map[string]interface{}, error) {
	if len(b) == 0 {
		return map[string]interface{}{}, nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
