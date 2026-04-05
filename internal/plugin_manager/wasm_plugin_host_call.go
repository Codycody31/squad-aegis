package plugin_manager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/tetratelabs/wazero/api"
)

// WASM host_call category -> required_capabilities (see docs/wasm-guest-abi.md).
var wasmHostCategoryCapability = map[string]string{
	"server":   NativePluginCapabilityAPIServer,
	"rcon":     NativePluginCapabilityAPIRCON,
	"database": NativePluginCapabilityAPIDatabase,
	"rule":     NativePluginCapabilityAPIRule,
	"admin":    NativePluginCapabilityAPIAdmin,
	"discord":  NativePluginCapabilityAPIDiscord,
	"event":    NativePluginCapabilityAPIEvent,
}

type wasmHostCallEnvelope struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (p *wasmPlugin) hostCall(ctx context.Context, m api.Module,
	catPtr, catLen, methPtr, methLen, reqPtr, reqLen, outPtr, outCap, outWrittenPtr uint32,
) uint32 {
	mem := m.Memory()
	if mem == nil {
		return wasmHostErrMemory
	}

	cat, ok := wasmMemUTF8(mem, catPtr, catLen)
	if !ok {
		return wasmHostErrMemory
	}
	method, ok := wasmMemUTF8(mem, methPtr, methLen)
	if !ok {
		return wasmHostErrMemory
	}

	if reqLen > maxWasmHostCallRequestJSONBytes {
		return wasmHostErrBuffer
	}
	var reqBytes []byte
	if reqLen > 0 {
		var rok bool
		reqBytes, rok = mem.Read(reqPtr, reqLen)
		if !rok {
			return wasmHostErrMemory
		}
	} else {
		reqBytes = []byte("{}")
	}

	category := strings.TrimSpace(cat)
	meth := strings.TrimSpace(method)

	capName := wasmHostCategoryCapability[category]
	if capName == "" {
		return wasmHostErrInvalid
	}
	if !pluginDefinitionHasCapability(p.def, capName) {
		return wasmHostErrDenied
	}

	callCtx := ctx
	if _, has := callCtx.Deadline(); !has {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(callCtx, wasmHostCallTimeout)
		defer cancel()
	}

	data, err := p.wasmHostDispatch(callCtx, category, meth, json.RawMessage(reqBytes))
	env := wasmHostCallEnvelope{OK: err == nil, Data: data}
	if err != nil {
		env.Error = err.Error()
	}

	raw, jerr := json.Marshal(env)
	if jerr != nil {
		return wasmHostErrInvalid
	}
	if uint32(len(raw)) > maxWasmHostCallResponseJSONBytes || uint32(len(raw)) > outCap {
		return wasmHostErrBuffer
	}
	if !mem.Write(outPtr, raw) {
		return wasmHostErrMemory
	}
	if !mem.WriteUint32Le(outWrittenPtr, uint32(len(raw))) {
		return wasmHostErrMemory
	}
	return wasmHostOK
}

func wasmMemUTF8(mem api.Memory, ptr, length uint32) (string, bool) {
	if length == 0 {
		return "", true
	}
	b, ok := mem.Read(ptr, length)
	if !ok {
		return "", false
	}
	return string(b), true
}

func (p *wasmPlugin) wasmHostDispatch(ctx context.Context, category, method string, req json.RawMessage) (interface{}, error) {
	_ = ctx
	if p.apis == nil {
		return nil, fmt.Errorf("plugin apis not available")
	}

	switch category {
	case "server":
		return p.wasmHostDispatchServer(method, req)
	case "rcon":
		return p.wasmHostDispatchRcon(method, req)
	case "database":
		return p.wasmHostDispatchDatabase(method, req)
	case "rule":
		return p.wasmHostDispatchRule(method, req)
	case "admin":
		return p.wasmHostDispatchAdmin(method, req)
	case "discord":
		return p.wasmHostDispatchDiscord(method, req)
	case "event":
		return p.wasmHostDispatchEvent(method, req)
	default:
		return nil, fmt.Errorf("unknown category %q", category)
	}
}

func (p *wasmPlugin) wasmHostDispatchServer(method string, req json.RawMessage) (interface{}, error) {
	if p.apis.ServerAPI == nil {
		return nil, fmt.Errorf("server api not available")
	}
	switch method {
	case "GetServerID":
		return map[string]string{"server_id": p.apis.ServerAPI.GetServerID().String()}, nil
	case "GetServerInfo":
		info, err := p.apis.ServerAPI.GetServerInfo()
		if err != nil {
			return nil, err
		}
		return info, nil
	case "GetPlayers":
		players, err := p.apis.ServerAPI.GetPlayers()
		if err != nil {
			return nil, err
		}
		return players, nil
	case "GetAdmins":
		admins, err := p.apis.ServerAPI.GetAdmins()
		if err != nil {
			return nil, err
		}
		return admins, nil
	case "GetSquads":
		squads, err := p.apis.ServerAPI.GetSquads()
		if err != nil {
			return nil, err
		}
		return squads, nil
	default:
		return nil, fmt.Errorf("unknown server method %q", method)
	}
}

func (p *wasmPlugin) wasmHostDispatchDatabase(method string, req json.RawMessage) (interface{}, error) {
	if p.apis.DatabaseAPI == nil {
		return nil, fmt.Errorf("database api not available")
	}
	switch method {
	case "GetPluginData":
		var in struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.Key) == "" {
			return nil, fmt.Errorf("key is required")
		}
		v, err := p.apis.DatabaseAPI.GetPluginData(in.Key)
		if err != nil {
			return nil, err
		}
		return map[string]string{"value": v}, nil
	case "SetPluginData":
		var in struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.Key) == "" {
			return nil, fmt.Errorf("key is required")
		}
		return nil, p.apis.DatabaseAPI.SetPluginData(in.Key, in.Value)
	case "DeletePluginData":
		var in struct {
			Key string `json:"key"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.Key) == "" {
			return nil, fmt.Errorf("key is required")
		}
		return nil, p.apis.DatabaseAPI.DeletePluginData(in.Key)
	default:
		return nil, fmt.Errorf("unknown database method %q", method)
	}
}

func (p *wasmPlugin) wasmHostDispatchRule(method string, req json.RawMessage) (interface{}, error) {
	if p.apis.RuleAPI == nil {
		return nil, fmt.Errorf("rule api not available")
	}
	switch method {
	case "ListServerRules":
		var in struct {
			ParentRuleID *string `json:"parent_rule_id"`
		}
		if len(req) > 0 && string(req) != "null" {
			if err := json.Unmarshal(req, &in); err != nil {
				return nil, err
			}
		}
		rules, err := p.apis.RuleAPI.ListServerRules(in.ParentRuleID)
		if err != nil {
			return nil, err
		}
		return rules, nil
	case "ListServerRuleActions":
		var in struct {
			RuleID string `json:"rule_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.RuleID) == "" {
			return nil, fmt.Errorf("rule_id is required")
		}
		actions, err := p.apis.RuleAPI.ListServerRuleActions(in.RuleID)
		if err != nil {
			return nil, err
		}
		return actions, nil
	default:
		return nil, fmt.Errorf("unknown rule method %q", method)
	}
}

func (p *wasmPlugin) wasmHostDispatchRcon(method string, req json.RawMessage) (interface{}, error) {
	if p.apis.RconAPI == nil {
		return nil, fmt.Errorf("rcon api not available")
	}
	switch method {
	case "SendCommand":
		var in struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		out, err := p.apis.RconAPI.SendCommand(in.Command)
		if err != nil {
			return nil, err
		}
		return map[string]string{"output": out}, nil
	case "Broadcast":
		var in struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.Broadcast(in.Message)
	case "SendWarningToPlayer":
		var in struct {
			PlayerID string `json:"player_id"`
			Message  string `json:"message"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.SendWarningToPlayer(in.PlayerID, in.Message)
	case "KickPlayer":
		var in struct {
			PlayerID string `json:"player_id"`
			Reason   string `json:"reason"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.KickPlayer(in.PlayerID, in.Reason)
	case "BanPlayer":
		var in struct {
			PlayerID   string `json:"player_id"`
			Reason     string `json:"reason"`
			DurationNS int64  `json:"duration_ns"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.BanPlayer(in.PlayerID, in.Reason, time.Duration(in.DurationNS))
	case "BanWithEvidence":
		var in struct {
			PlayerID   string `json:"player_id"`
			Reason     string `json:"reason"`
			DurationNS int64  `json:"duration_ns"`
			EventID    string `json:"event_id"`
			EventType  string `json:"event_type"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		id, err := p.apis.RconAPI.BanWithEvidence(in.PlayerID, in.Reason, time.Duration(in.DurationNS), in.EventID, in.EventType)
		if err != nil {
			return nil, err
		}
		return map[string]string{"ban_id": id}, nil
	case "WarnPlayerWithRule":
		var in struct {
			PlayerID string  `json:"player_id"`
			Message  string  `json:"message"`
			RuleID   *string `json:"rule_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.WarnPlayerWithRule(in.PlayerID, in.Message, in.RuleID)
	case "KickPlayerWithRule":
		var in struct {
			PlayerID string  `json:"player_id"`
			Reason   string  `json:"reason"`
			RuleID   *string `json:"rule_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.KickPlayerWithRule(in.PlayerID, in.Reason, in.RuleID)
	case "BanPlayerWithRule":
		var in struct {
			PlayerID   string  `json:"player_id"`
			Reason     string  `json:"reason"`
			DurationNS int64   `json:"duration_ns"`
			RuleID     *string `json:"rule_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.BanPlayerWithRule(in.PlayerID, in.Reason, time.Duration(in.DurationNS), in.RuleID)
	case "BanWithEvidenceAndRule":
		var in struct {
			PlayerID   string  `json:"player_id"`
			Reason     string  `json:"reason"`
			DurationNS int64   `json:"duration_ns"`
			EventID    string  `json:"event_id"`
			EventType  string  `json:"event_type"`
			RuleID     *string `json:"rule_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		id, err := p.apis.RconAPI.BanWithEvidenceAndRule(in.PlayerID, in.Reason, time.Duration(in.DurationNS), in.EventID, in.EventType, in.RuleID)
		if err != nil {
			return nil, err
		}
		return map[string]string{"ban_id": id}, nil
	case "BanWithEvidenceAndRuleAndMetadata":
		var in struct {
			PlayerID   string                 `json:"player_id"`
			Reason     string                 `json:"reason"`
			DurationNS int64                  `json:"duration_ns"`
			EventID    string                 `json:"event_id"`
			EventType  string                 `json:"event_type"`
			RuleID     *string                `json:"rule_id"`
			Metadata   map[string]interface{} `json:"metadata"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		id, err := p.apis.RconAPI.BanWithEvidenceAndRuleAndMetadata(in.PlayerID, in.Reason, time.Duration(in.DurationNS), in.EventID, in.EventType, in.RuleID, in.Metadata)
		if err != nil {
			return nil, err
		}
		return map[string]string{"ban_id": id}, nil
	case "RemovePlayerFromSquad":
		var in struct {
			PlayerID string `json:"player_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.RemovePlayerFromSquad(in.PlayerID)
	case "RemovePlayerFromSquadById":
		var in struct {
			PlayerID string `json:"player_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.RconAPI.RemovePlayerFromSquadById(in.PlayerID)
	default:
		return nil, fmt.Errorf("unknown rcon method %q", method)
	}
}

func (p *wasmPlugin) wasmHostDispatchAdmin(method string, req json.RawMessage) (interface{}, error) {
	if p.apis.AdminAPI == nil {
		return nil, fmt.Errorf("admin api not available")
	}
	switch method {
	case "AddTemporaryAdmin":
		var in struct {
			PlayerID  string  `json:"player_id"`
			RoleName  string  `json:"role_name"`
			Notes     string  `json:"notes"`
			ExpiresAt *string `json:"expires_at"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		var exp *time.Time
		if in.ExpiresAt != nil && strings.TrimSpace(*in.ExpiresAt) != "" {
			t, err := time.Parse(time.RFC3339, strings.TrimSpace(*in.ExpiresAt))
			if err != nil {
				return nil, fmt.Errorf("expires_at: %w", err)
			}
			exp = &t
		}
		return nil, p.apis.AdminAPI.AddTemporaryAdmin(in.PlayerID, in.RoleName, in.Notes, exp)
	case "RemoveTemporaryAdmin":
		var in struct {
			PlayerID string `json:"player_id"`
			Notes    string `json:"notes"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.AdminAPI.RemoveTemporaryAdmin(in.PlayerID, in.Notes)
	case "RemoveTemporaryAdminRole":
		var in struct {
			PlayerID string `json:"player_id"`
			RoleName string `json:"role_name"`
			Notes    string `json:"notes"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		return nil, p.apis.AdminAPI.RemoveTemporaryAdminRole(in.PlayerID, in.RoleName, in.Notes)
	case "GetPlayerAdminStatus":
		var in struct {
			PlayerID string `json:"player_id"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		st, err := p.apis.AdminAPI.GetPlayerAdminStatus(in.PlayerID)
		if err != nil {
			return nil, err
		}
		return st, nil
	case "ListTemporaryAdmins":
		list, err := p.apis.AdminAPI.ListTemporaryAdmins()
		if err != nil {
			return nil, err
		}
		return list, nil
	default:
		return nil, fmt.Errorf("unknown admin method %q", method)
	}
}

func (p *wasmPlugin) wasmHostDispatchDiscord(method string, req json.RawMessage) (interface{}, error) {
	if p.apis.DiscordAPI == nil {
		return nil, fmt.Errorf("discord api not available")
	}
	switch method {
	case "SendMessage":
		var in struct {
			ChannelID string `json:"channel_id"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		id, err := p.apis.DiscordAPI.SendMessage(in.ChannelID, in.Content)
		if err != nil {
			return nil, err
		}
		return map[string]string{"message_id": id}, nil
	case "SendEmbed":
		var in struct {
			ChannelID string         `json:"channel_id"`
			Embed     *DiscordEmbed `json:"embed"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		id, err := p.apis.DiscordAPI.SendEmbed(in.ChannelID, in.Embed)
		if err != nil {
			return nil, err
		}
		return map[string]string{"message_id": id}, nil
	default:
		return nil, fmt.Errorf("unknown discord method %q", method)
	}
}

func (p *wasmPlugin) wasmHostDispatchEvent(method string, req json.RawMessage) (interface{}, error) {
	if p.apis.EventAPI == nil {
		return nil, fmt.Errorf("event api not available")
	}
	switch method {
	case "PublishEvent":
		var in struct {
			EventType string                 `json:"event_type"`
			Data      map[string]interface{} `json:"data"`
			Raw       string                 `json:"raw"`
		}
		if err := json.Unmarshal(req, &in); err != nil {
			return nil, err
		}
		if strings.TrimSpace(in.EventType) == "" {
			return nil, fmt.Errorf("event_type is required")
		}
		return nil, p.apis.EventAPI.PublishEvent(in.EventType, in.Data, in.Raw)
	default:
		return nil, fmt.Errorf("unknown event method %q", method)
	}
}
