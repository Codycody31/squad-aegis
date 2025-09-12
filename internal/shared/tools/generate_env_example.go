//go:build ignore

// generate_env_example.go
package main

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

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

func envKey(prefix, name string) string {
	if prefix != "" {
		return prefix + "_" + toUpperSnakeCase(name)
	}
	return toUpperSnakeCase(name)
}

func generateEnvExample(s interface{}, prefix string) []string {
	val := reflect.ValueOf(s).Elem()
	typ := val.Type()
	var envVars []string

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)
		key := envKey(prefix, fieldType.Name)

		// Skip fields that are not modifiable
		if fieldType.Tag.Get("modifiable") == "false" {
			continue
		}

		var defaultValue string
		if field.Kind() == reflect.String {
			defaultValue = "\"" + fieldType.Tag.Get("default") + "\""
		} else {
			defaultValue = fieldType.Tag.Get("default")
		}

		if field.Kind() == reflect.Struct {
			sectionName := strings.Title(strings.ToLower(fieldType.Name))
			envVars = append(envVars, "\n# "+sectionName)
			envVars = append(envVars, generateEnvExample(field.Addr().Interface(), key)...)
		} else {
			envVar := key + "=" + defaultValue
			envVars = append(envVars, envVar)
		}
	}

	return envVars
}

func main() {
	config := config.Struct{}
	envExample := generateEnvExample(&config, "")

	outputFile := "../../.env.example"
	if err := os.WriteFile(outputFile, []byte(strings.Join(envExample, "\n")), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write .env.example file: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generated .env.example file")
}
