package models

import (
	"time"

	"github.com/google/uuid"
)

type ServerMOTDConfig struct {
	ID       uuid.UUID `json:"id"`
	ServerID uuid.UUID `json:"server_id"`

	// Content
	PrefixText string `json:"prefix_text"`
	SuffixText string `json:"suffix_text"`

	// Generation settings
	AutoGenerateFromRules   bool `json:"auto_generate_from_rules"`
	IncludeRuleDescriptions bool `json:"include_rule_descriptions"`

	// Upload settings
	UploadEnabled      bool   `json:"upload_enabled"`
	AutoUploadOnChange bool   `json:"auto_upload_on_change"`
	UseLogCredentials  bool   `json:"use_log_credentials"`
	UploadHost         *string `json:"upload_host,omitempty"`
	UploadPort         *int    `json:"upload_port,omitempty"`
	UploadUsername     *string `json:"upload_username,omitempty"`
	UploadPassword     *string `json:"-"` // Hidden from JSON responses
	UploadProtocol     *string `json:"upload_protocol,omitempty"`

	// Tracking
	LastUploadedAt       *time.Time `json:"last_uploaded_at,omitempty"`
	LastUploadError      *string    `json:"last_upload_error,omitempty"`
	LastGeneratedContent *string    `json:"last_generated_content,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ServerMOTDConfigUpdateRequest struct {
	PrefixText              *string `json:"prefix_text,omitempty"`
	SuffixText              *string `json:"suffix_text,omitempty"`
	AutoGenerateFromRules   *bool   `json:"auto_generate_from_rules,omitempty"`
	IncludeRuleDescriptions *bool   `json:"include_rule_descriptions,omitempty"`
	UploadEnabled           *bool   `json:"upload_enabled,omitempty"`
	AutoUploadOnChange      *bool   `json:"auto_upload_on_change,omitempty"`
	UseLogCredentials       *bool   `json:"use_log_credentials,omitempty"`
	UploadHost              *string `json:"upload_host,omitempty"`
	UploadPort              *int    `json:"upload_port,omitempty"`
	UploadUsername          *string `json:"upload_username,omitempty"`
	UploadPassword          *string `json:"upload_password,omitempty"`
	UploadProtocol          *string `json:"upload_protocol,omitempty"`
}

type MOTDPreviewResponse struct {
	Content     string `json:"content"`
	RulesCount  int    `json:"rules_count"`
	GeneratedAt string `json:"generated_at"`
}

type MOTDUploadResponse struct {
	Success    bool    `json:"success"`
	UploadedAt string  `json:"uploaded_at,omitempty"`
	Error      *string `json:"error,omitempty"`
}

type MOTDConnectionTestResponse struct {
	Success bool    `json:"success"`
	Message string  `json:"message"`
	Error   *string `json:"error,omitempty"`
}
