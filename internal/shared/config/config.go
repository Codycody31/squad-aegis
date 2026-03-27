package config

import (
	"os"
	"strings"
	"sync"

	"github.com/cristalhq/aconfig"
	"github.com/cristalhq/aconfig/aconfigyaml"
	"github.com/joho/godotenv"
)

var (
	Config *Struct
	once   sync.Once
)

func init() {
	load()
}

func load() {
	once.Do(func() {
		godotenv.Load()

		var cfg Struct
		runningUnderTest := false
		for _, arg := range os.Args[1:] {
			if strings.HasPrefix(arg, "-test.") {
				runningUnderTest = true
				break
			}
		}

		loader := aconfig.LoaderFor(&cfg, aconfig.Config{
			Files: []string{"/etc/squad-aegis/config.yaml", "config.yaml"},
			FileDecoders: map[string]aconfig.FileDecoder{
				".yaml": aconfigyaml.New(),
			},
			AllowUnknownFlags: runningUnderTest,
			SkipFlags:         runningUnderTest,
		})

		if err := loader.Load(); err != nil {
			panic(err)
		}

		Config = &cfg
	})
}
