package config

import (
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

		loader := aconfig.LoaderFor(&cfg, aconfig.Config{
			Files: []string{"/etc/squad-aegis/config.yaml", "config.yaml"},
			FileDecoders: map[string]aconfig.FileDecoder{
				".yaml": aconfigyaml.New(),
			},
		})

		if err := loader.Load(); err != nil {
			panic(err)
		}

		Config = &cfg
	})
}
