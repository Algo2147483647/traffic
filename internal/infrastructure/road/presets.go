package model

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PresetConfig is one JSON-backed city configuration discovered in a preset
// directory. ID is the JSON filename without its extension.
type PresetConfig struct {
	ID     string
	Path   string
	Config CityConfig
}

// LoadPresetDirectory loads every *.json city configuration in filename
// order. Adding another JSON file is therefore enough to expose a new preset
// without changing Go source code.
func LoadPresetDirectory(directory string) ([]PresetConfig, error) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return nil, fmt.Errorf("read preset directory: %w", err)
	}

	jsonFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		jsonFiles = append(jsonFiles, entry.Name())
	}
	sort.Strings(jsonFiles)

	presets := make([]PresetConfig, 0, len(jsonFiles))
	for _, name := range jsonFiles {
		path := filepath.Join(directory, name)
		config, err := LoadConfig(path)
		if err != nil {
			return nil, fmt.Errorf("load preset %q: %w", name, err)
		}
		presets = append(presets, PresetConfig{
			ID:     strings.TrimSuffix(name, filepath.Ext(name)),
			Path:   path,
			Config: config,
		})
	}
	return presets, nil
}
