package main

import (
	"errors"
	"sort"
	"time"

	"github.com/xyuer/PingZero/internal/config"
	"github.com/xyuer/PingZero/internal/engine"
	"github.com/xyuer/PingZero/internal/state"

	_ "github.com/xyuer/PingZero/internal/tunnel/kcp"
)

type App struct {
	engine *engine.Engine
	store  *state.Store
	bundle *config.Bundle

	currentGame string
	currentNode string
	startedAt   time.Time
}

type GameInfo struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Running   bool     `json:"running"`
	Processes []string `json:"processes"`
}

type AccelStatus struct {
	Running     bool   `json:"running"`
	GameID      string `json:"gameID"`
	NodeAddr    string `json:"nodeAddr"`
	UptimeSecs  int64  `json:"uptimeSecs"`
	PacketsSent uint64 `json:"packetsSent"`
}

type LatencyInfo struct {
	CurrentMS int `json:"currentMS"`
	MinMS     int `json:"minMS"`
	MaxMS     int `json:"maxMS"`
	AvgMS     int `json:"avgMS"`
}

func NewApp() *App {
	store := state.NewStore()
	bundle, _ := config.LoadAll("config")
	return &App{engine: engine.New(store), store: store, bundle: bundle}
}

func (a *App) GetGames() []GameInfo {
	if a.bundle == nil {
		return nil
	}
	games := make([]GameInfo, 0, len(a.bundle.Games))
	for _, game := range a.bundle.Games {
		games = append(games, GameInfo{
			ID:        game.ID,
			Name:      game.Name,
			Running:   a.currentGame == game.ID && a.engine.Running(),
			Processes: append([]string(nil), game.Processes...),
		})
	}
	sort.Slice(games, func(i, j int) bool { return games[i].ID < games[j].ID })
	return games
}

func (a *App) StartAcceleration(gameID string, nodeAddr string) error {
	if gameID == "" {
		return errors.New("game ID is required")
	}
	if nodeAddr == "" {
		return errors.New("node address is required")
	}
	if a.bundle == nil {
		return errors.New("config is not loaded")
	}
	if _, ok := a.bundle.Games[gameID]; !ok {
		return errors.New("unknown game")
	}
	if err := a.engine.Start(); err != nil {
		return err
	}
	a.currentGame = gameID
	a.currentNode = nodeAddr
	a.startedAt = time.Now()
	return nil
}

func (a *App) StopAcceleration() error {
	if err := a.engine.Stop(); err != nil {
		return err
	}
	a.currentGame = ""
	a.currentNode = ""
	a.startedAt = time.Time{}
	return nil
}

func (a *App) GetStatus() AccelStatus {
	stats := a.engine.Stats()
	status := AccelStatus{
		Running:     a.engine.Running(),
		GameID:      a.currentGame,
		NodeAddr:    a.currentNode,
		PacketsSent: stats.PacketsSent,
	}
	if !a.startedAt.IsZero() {
		status.UptimeSecs = int64(time.Since(a.startedAt).Seconds())
	}
	return status
}

func (a *App) GetLatency() LatencyInfo {
	return LatencyInfo{}
}
