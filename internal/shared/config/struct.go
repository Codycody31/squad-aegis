package config

type Struct struct {
	App struct {
		IsDevelopment bool `default:"true"`
		WebUiProxy    string
		Port          string `default:"3113"`
		Url           string `default:"http://localhost:3113"`
		InContainer   bool   `default:"false"`
	}
	Initial struct {
		Admin struct {
			Username string `default:"admin"`
			Password string `default:"admin"`
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
		Password string
		Database int `default:"0"`
	}
	Log struct {
		Level          string `default:"info"`
		ShowGin        bool   `default:"false"`
		ShowPluginLogs bool   `default:"true"`
		File           string
	}
	Debug struct {
		Pretty  bool `default:"true"`
		NoColor bool `default:"false"`
	}
	Storage struct {
		Type string `default:"local"` // "local" or "s3"
		S3   struct {
			Region          string `default:"us-east-1"`
			Bucket          string
			AccessKeyID     string
			SecretAccessKey string
			Endpoint        string
			UseSSL          bool `default:"true"`
		}
		Local struct {
			Path string `default:"storage"`
		}
	}
}
