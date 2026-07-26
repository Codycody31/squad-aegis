package server

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"go.codycody31.dev/squad-aegis/internal/core"
	"go.codycody31.dev/squad-aegis/internal/models"
	"go.codycody31.dev/squad-aegis/internal/server/responses"
	squadRcon "go.codycody31.dev/squad-aegis/internal/squad-rcon"
	"golang.org/x/sync/errgroup"
)

const (
	summaryConcurrency     = 8
	summaryPerServerBudget = 4 * time.Second
)

type ServerSummaryPlayers struct {
	Current int `json:"current"`
	Max     int `json:"max"`
	Queue   int `json:"queue"`
}

type ServerSummaryEntry struct {
	Id           uuid.UUID             `json:"id"`
	Name         string                `json:"name"`
	IpAddress    string                `json:"ip_address"`
	GamePort     int                   `json:"game_port"`
	RconPort     int                   `json:"rcon_port"`
	GameOnline   bool                  `json:"game_online"`
	RconOnline   bool                  `json:"rcon_online"`
	Players      *ServerSummaryPlayers `json:"players,omitempty"`
	CurrentLayer string                `json:"current_layer,omitempty"`
	BanCount     int                   `json:"ban_count"`
}

// ServersSummary returns one row per server the user can see, with live
// status gathered concurrently. A dead server degrades to offline fields
// instead of failing or stalling the whole response.
func (s *Server) ServersSummary(c *gin.Context) {
	user := s.getUserFromSession(c)

	servers, err := core.GetServers(c.Request.Context(), s.Dependencies.DB, user)
	if err != nil {
		responses.BadRequest(c, "Failed to get servers", &gin.H{"error": err.Error()})
		return
	}

	serverIDs := make([]uuid.UUID, 0, len(servers))
	for _, srv := range servers {
		serverIDs = append(serverIDs, srv.Id)
	}

	banCounts, err := s.activeBanCounts(c, serverIDs)
	if err != nil {
		// Ban counts are non-critical for the fleet view; degrade to zeros.
		banCounts = map[uuid.UUID]int{}
	}

	entries := make([]ServerSummaryEntry, len(servers))
	var group errgroup.Group
	group.SetLimit(summaryConcurrency)

	for i, srv := range servers {
		i, srv := i, srv
		group.Go(func() error {
			entries[i] = s.buildServerSummaryEntry(srv, banCounts[srv.Id])
			return nil
		})
	}
	_ = group.Wait()

	responses.Success(c, "Server summary fetched successfully", &gin.H{"servers": entries})
}

func (s *Server) buildServerSummaryEntry(srv *models.Server, banCount int) ServerSummaryEntry {
	entry := ServerSummaryEntry{
		Id:        srv.Id,
		Name:      srv.Name,
		IpAddress: srv.IpAddress,
		GamePort:  srv.GamePort,
		RconPort:  srv.RconPort,
		BanCount:  banCount,
	}

	type liveInfo struct {
		gameOnline bool
		info       *squadRcon.ServerInfo
	}

	done := make(chan liveInfo, 1)
	go func() {
		result := liveInfo{gameOnline: checkUDPPort(srv.IpAddress, srv.GamePort)}

		rconIP := srv.IpAddress
		if srv.RconIpAddress != nil && *srv.RconIpAddress != "" {
			rconIP = *srv.RconIpAddress
		}
		rconClient, err := squadRcon.NewSquadRconWithConnection(
			s.Dependencies.RconManager, srv.Id, rconIP, srv.RconPort, srv.RconPassword,
		)
		if err == nil {
			defer rconClient.Close()
			if info, err := rconClient.GetServerInfo(); err == nil {
				result.info = &info
			}
		}
		done <- result
	}()

	select {
	case result := <-done:
		entry.GameOnline = result.gameOnline
		if result.info != nil {
			entry.RconOnline = true
			entry.CurrentLayer = result.info.MapName
			entry.Players = &ServerSummaryPlayers{
				Current: result.info.PlayerCount,
				Max:     result.info.MaxPlayers,
				Queue:   result.info.PublicQueue + result.info.ReservedQueue,
			}
		}
	case <-time.After(summaryPerServerBudget):
		// Probe still hanging: report the server as unreachable rather than
		// holding up the rest of the fleet.
	}

	return entry
}

func (s *Server) activeBanCounts(c *gin.Context, serverIDs []uuid.UUID) (map[uuid.UUID]int, error) {
	counts := make(map[uuid.UUID]int, len(serverIDs))
	if len(serverIDs) == 0 {
		return counts, nil
	}

	rows, err := s.Dependencies.DB.QueryContext(
		c.Request.Context(),
		`SELECT server_id, COUNT(*)
		 FROM server_bans
		 WHERE server_id = ANY($1)
		   AND (expires_at IS NULL OR expires_at > NOW())
		 GROUP BY server_id`,
		pq.Array(serverIDs),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var id uuid.UUID
		var count int
		if err := rows.Scan(&id, &count); err != nil {
			return nil, err
		}
		counts[id] = count
	}
	return counts, rows.Err()
}
