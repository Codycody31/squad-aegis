package config

type Struct struct {
	App struct {
		IsDevelopment bool   `default:"true"`
		WebUiProxy    string `default:""`
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
	Log struct {
		Level   string `default:"info"`
		ShowGin bool   `default:"false"`
		File    string `default:""`
	}
	Debug struct {
		Pretty  bool `default:"true"`
		NoColor bool `default:"false"`
	}
}
