//go:generate go run ../tools/generate_env_example.go
package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"
	"sync"

	"github.com/joho/godotenv"
)

var (
	// Config is the instance of the loaded configuration that is exported.
	Config *Struct
	once   sync.Once
)

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvAsBool(key string, fallback bool) bool {
	valueStr := getEnv(key, "")
	if value, err := strconv.ParseBool(valueStr); err == nil {
		return value
	}
	return fallback
}

func getEnvAsInt(key string, fallback int) int {
	valueStr := getEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return fallback
}

// toUpperSnakeCase converts a camelCase string to an UPPER_SNAKE_CASE string.
func toUpperSnakeCase(s string) string {
	var result string
	for i, r := range s {
		if r >= 'A' && r <= 'Z' && i > 0 {
			result += "_"
		}
		result += strings.ToUpper(string(r))
	}
	return result
}

// envKey generates a string of environment keys based on struct field names.
func envKey(prefix, name string) string {
	if prefix != "" {
		return prefix + "_" + toUpperSnakeCase(name)
	}
	return toUpperSnakeCase(name)
}

// fillStruct fills the struct with environment variables based on their keys.
func fillStruct(s interface{}, prefix string) {
	val := reflect.ValueOf(s).Elem()
	typ := val.Type()

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		key := envKey(prefix, fieldType.Name)

		if field.Kind() == reflect.Struct {
			fillStruct(field.Addr().Interface(), key)
		} else {
			if field.CanSet() {
				// Check if field is modifiable (defaults to true if not specified)
				modifiable := fieldType.Tag.Get("modifiable")
				if modifiable == "false" {
					field.SetString(fieldType.Tag.Get("default"))
					continue // Skip this field if it's not modifiable
				}

				switch field.Kind() {
				case reflect.String:
					field.SetString(getEnv(key, fieldType.Tag.Get("default")))
				case reflect.Slice:
					// Parse default value as slice from tag
					defaultSliceValue := strings.Split(fieldType.Tag.Get("default"), ",")
					field.Set(reflect.ValueOf(defaultSliceValue))
				case reflect.Int:
					// Parse default value as int from tag
					defaultIntValue, _ := strconv.Atoi(fieldType.Tag.Get("default"))
					field.SetInt(int64(getEnvAsInt(key, defaultIntValue)))
				case reflect.Bool:
					field.SetBool(getEnvAsBool(key, fieldType.Tag.Get("default") == "true"))
				}
			}
		}
	}
}

func init() {
	load()
}

func load() {
	once.Do(func() {
		_ = godotenv.Load()
		Config = &Struct{}
		fillStruct(Config, "")
	})
}
