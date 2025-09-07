package main

import (
	"context"
	"database/sql"
	"log"
	"time"

	_ "github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/clickhouse"
	"go.codycody31.dev/squad-aegis/internal/db"
	"go.codycody31.dev/squad-aegis/internal/shared/config"
)

// PlayerMigrationRecord represents a player record from PostgreSQL
type PlayerMigrationRecord struct {
	SteamID     int64
	DisplayName *string
	FirstSeen   time.Time
	LastSeen    time.Time
}

func main() {
	ctx := context.Background()

	// Initialize database connection
	postgresDSN := db.PostgresDSN(config.Config.Db.Host, config.Config.Db.Port, config.Config.Db.User, config.Config.Db.Pass, config.Config.Db.Name)
	database, err := sql.Open("postgres", postgresDSN)
	if err != nil {
		log.Fatalf("Failed to connect to PostgreSQL: %v", err)
	}
	defer database.Close()

	// Initialize ClickHouse
	clickhouseConfig := clickhouse.Config{
		Host:     config.Config.ClickHouse.Host,
		Port:     config.Config.ClickHouse.Port,
		Database: config.Config.ClickHouse.Database,
		Username: config.Config.ClickHouse.Username,
		Password: config.Config.ClickHouse.Password,
		Debug:    config.Config.ClickHouse.Debug,
	}

	clickhouseClient, err := clickhouse.NewClient(clickhouseConfig)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer clickhouseClient.Close()

	// Check if players table exists in PostgreSQL
	var tableExists bool
	err = database.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.tables 
			WHERE table_schema = 'public' AND table_name = 'players'
		)
	`).Scan(&tableExists)
	if err != nil {
		log.Fatalf("Failed to check if players table exists: %v", err)
	}

	if !tableExists {
		log.Println("Players table does not exist in PostgreSQL, skipping migration")
		return
	}

	// Query all players from PostgreSQL
	rows, err := database.QueryContext(ctx, `
		SELECT steam_id, display_name, first_seen, last_seen 
		FROM players 
		ORDER BY first_seen
	`)
	if err != nil {
		log.Fatalf("Failed to query players from PostgreSQL: %v", err)
	}
	defer rows.Close()

	var players []PlayerMigrationRecord
	for rows.Next() {
		var player PlayerMigrationRecord
		err := rows.Scan(&player.SteamID, &player.DisplayName, &player.FirstSeen, &player.LastSeen)
		if err != nil {
			log.Printf("Failed to scan player record: %v", err)
			continue
		}
		players = append(players, player)
	}

	if len(players) == 0 {
		log.Println("No players found in PostgreSQL, migration complete")
		return
	}

	log.Printf("Found %d players to migrate to ClickHouse", len(players))

	// Insert into ClickHouse using batch processing
	const batchSize = 100
	for i := 0; i < len(players); i += batchSize {
		end := i + batchSize
		if end > len(players) {
			end = len(players)
		}

		batch := players[i:end]

		// Build batch insert query
		query := `INSERT INTO squad_aegis.players (steam_id, display_name, first_seen, last_seen, updated_at) VALUES `
		args := make([]interface{}, 0, len(batch)*5)

		for j, player := range batch {
			if j > 0 {
				query += ", "
			}
			query += "(?, ?, ?, ?, ?)"

			displayName := ""
			if player.DisplayName != nil {
				displayName = *player.DisplayName
			}

			args = append(args, uint64(player.SteamID), displayName, player.FirstSeen, player.LastSeen, time.Now())
		}

		err := clickhouseClient.Exec(ctx, query, args...)
		if err != nil {
			log.Printf("Failed to insert batch %d-%d: %v", i, end-1, err)
			continue
		}

		log.Printf("Inserted batch %d-%d", i, end-1)
	}

	log.Printf("Successfully migrated %d players to ClickHouse", len(players))

	// Verify the migration
	var count uint64
	err = clickhouseClient.QueryRow(ctx, "SELECT count() FROM squad_aegis.players").Scan(&count)
	if err != nil {
		log.Printf("Failed to verify migration: %v", err)
	} else {
		log.Printf("ClickHouse now contains %d player records", count)
	}
}
