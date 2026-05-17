package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

import "gopkg.in/yaml.v3"

type Bundle struct {
	Games  map[string]GameConfig
	Global GlobalConfig
}

func LoadAll(root string) (*Bundle, error) {
	if root == "" {
		root = "config"
	}
	gamesDir := filepath.Join(root, "games")
	info, err := os.Stat(gamesDir)
	if err != nil {
		return nil, fmt.Errorf("read games directory: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("games path is not a directory")
	}
	bundle := &Bundle{Games: make(map[string]GameConfig)}
	matches, err := filepath.Glob(filepath.Join(gamesDir, "*.yaml"))
	if err != nil {
		return nil, err
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		var game GameConfig
		if err := yaml.Unmarshal(data, &game); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		if game.ID == "" {
			game.ID = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		}
		bundle.Games[game.ID] = game
	}
	globalPath := filepath.Join(root, "global_bypass.yaml")
	if data, err := os.ReadFile(globalPath); err == nil {
		if err := yaml.Unmarshal(data, &bundle.Global); err != nil {
			return nil, fmt.Errorf("parse %s: %w", globalPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read %s: %w", globalPath, err)
	}
	return bundle, nil
}
