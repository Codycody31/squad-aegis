package plugin_manager

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"testing"
)

func TestDeleteInstalledConnectorPackageRejectsPersistedDatabaseInstances(t *testing.T) {
	t.Parallel()

	db := openTestSQLDB(t, &testSQLDriver{
		queryContext: func(query string, args []driver.NamedValue) (driver.Rows, error) {
			if !strings.Contains(query, "FROM connectors") {
				return nil, fmt.Errorf("unexpected query: %s", query)
			}
			if len(args) != 2 {
				t.Fatalf("len(args) = %d, want 2", len(args))
			}
			if got, want := fmt.Sprint(args[0].Value), "discord"; got != want {
				t.Fatalf("args[0] = %q, want %q", got, want)
			}
			if got, want := fmt.Sprint(args[1].Value), "com.example.connector"; got != want {
				t.Fatalf("args[1] = %q, want %q", got, want)
			}

			return &testSQLRows{
				columns: []string{"exists"},
				values:  [][]driver.Value{{true}},
			}, nil
		},
		execContext: func(query string, _ []driver.NamedValue) (driver.Result, error) {
			return nil, fmt.Errorf("unexpected exec: %s", query)
		},
	})

	registry := NewConnectorRegistry()
	if err := registry.RegisterConnector(ConnectorDefinition{
		ID:           "com.example.connector",
		LegacyIDs:    []string{"discord"},
		Name:         "Connector",
		Version:      "1.0.0",
		Source:       PluginSourceNative,
		InstallState: PluginInstallStateReady,
		CreateInstance: func() Connector {
			return testConnector{}
		},
	}); err != nil {
		t.Fatalf("registry.RegisterConnector() error = %v", err)
	}

	pm := &PluginManager{
		db:                db,
		connectorRegistry: registry,
		nativeConnectorPackages: map[string]*InstalledConnectorPackage{
			"com.example.connector": {
				ConnectorID: "com.example.connector",
				Manifest: ConnectorPackageManifest{
					ConnectorID: "com.example.connector",
					LegacyIDs:   []string{"discord"},
				},
			},
		},
	}

	err := pm.DeleteInstalledConnectorPackage(context.Background(), "com.example.connector")
	if err == nil {
		t.Fatal("DeleteInstalledConnectorPackage() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "still has configured instances") {
		t.Fatalf("DeleteInstalledConnectorPackage() error = %q, want configured instances", err)
	}
}
