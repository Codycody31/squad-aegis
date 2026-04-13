package config

type Struct struct {
	App struct {
		IsDevelopment bool   `default:"false"`
		WebUiProxy    string `default:""`
		Port          string `default:"3113"`
		Url           string `default:"http://localhost:3113"`
		InContainer   bool   `default:"false"`
	}
	Initial struct {
		Admin struct {
			Username string `default:""`
			Password string `default:""`
		}
	}
	Db struct {
		Host    string `default:"localhost"`
		Port    int    `default:"5432"`
		Name    string `default:"squad-aegis"`
		User    string `default:"squad-aegis"`
		Pass    string `default:"squad-aegis"`
		Migrate struct {
			Verbose bool `default:"false"`
		}
	}
	ClickHouse struct {
		Host     string `default:"localhost"`
		Port     int    `default:"9000"`
		Database string `default:"default"`
		Username string `default:"squad_aegis"`
		Password string `default:"squad_aegis"`
		Debug    bool   `default:"false"`
	}
	Valkey struct {
		Host     string `default:"localhost"`
		Port     int    `default:"6379"`
		Password string `default:""`
		Database int    `default:"0"`
	}
	Log struct {
		Level          string `default:"info"`
		ShowGin        bool   `default:"false"`
		ShowPluginLogs bool   `default:"true"`
		File           string `default:""`
	}
	Debug struct {
		Pretty  bool `default:"true"`
		NoColor bool `default:"false"`
	}
	Storage struct {
		Type      string `default:"local"` // "local" or "s3"
		LocalPath string `default:"storage"`
		S3        struct {
			Region          string `default:"us-east-1"`
			Bucket          string `default:""`
			AccessKeyID     string `default:""`
			SecretAccessKey string `default:""`
			Endpoint        string `default:""` // Optional: for S3-compatible services (MinIO, etc.)
			UseSSL          bool   `default:"true"`
		}
	}
	Plugins struct {
		NativeEnabled       bool   `default:"true"`
		RuntimeDir          string `default:""`
		ConnectorRuntimeDir string `default:""`
		AllowUnsafeSideload bool   `default:"false"`
		MaxUploadSize       int64  `default:"52428800"`
		// Comma-separated base64 ed25519 public keys. A manifest.pub not in
		// this list is treated as unsigned regardless of signature validity.
		TrustedSigningKeys string `default:""`

		// Subprocess rate limiting: per-instance HostAPI token bucket. A
		// compromised plugin that floods the host with RconAPI/LogAPI calls
		// is throttled to HostAPIRatePerSec sustained with HostAPIBurst peak.
		// Zero or negative values disable rate limiting (NOT recommended in
		// production).
		HostAPIRatePerSec float64 `default:"50"`
		HostAPIBurst      int     `default:"100"`

		// Subprocess health monitoring: how often to ping each subprocess.
		// Zero or negative disables the background health monitor.
		HealthCheckIntervalSeconds int `default:"10"`

		// Subprocess privilege drop (unix only). Set a non-zero UID/GID to
		// launch native plugin/connector subprocesses under a different
		// user/group. Subprocesses always run with NoNewPrivs=true so they
		// cannot gain additional capabilities after exec. Groups is a
		// comma-separated list of supplementary group IDs.
		SubprocessUID         int    `default:"0"`
		SubprocessGID         int    `default:"0"`
		SubprocessGroups      string `default:""`
		SubprocessNoNewPrivs  bool   `default:"true"`
	}
}
