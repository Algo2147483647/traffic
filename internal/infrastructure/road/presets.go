package model

import (
	"encoding/json"
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

	presets := []PresetConfig{}
	jsonFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			b, e := os.ReadFile(filepath.Join(directory, entry.Name(), "preset.json"))
			if os.IsNotExist(e) {
				continue
			}
			if e != nil {
				return nil, e
			}
			var manifest struct {
				ID    string `json:"id"`
				Road  string `json:"road"`
				Metro string `json:"metro"`
			}
			if e = json.Unmarshal(b, &manifest); e != nil {
				return nil, e
			}
			if manifest.ID == "" || manifest.Road == "" || filepath.Base(manifest.Road) != manifest.Road || filepath.Base(manifest.Metro) != manifest.Metro {
				return nil, fmt.Errorf("invalid example manifest")
			}
			path := filepath.Join(directory, entry.Name(), manifest.Road)
			cfg, e := LoadConfig(path)
			if e != nil {
				return nil, e
			}
			if cfg.MetroFile != manifest.Metro {
				return nil, fmt.Errorf("metro manifest mismatch")
			}
			presets = append(presets, PresetConfig{ID: manifest.ID, Path: path, Config: cfg})
			continue
		}
		if !strings.EqualFold(filepath.Ext(entry.Name()), ".json") {
			continue
		}
		jsonFiles = append(jsonFiles, entry.Name())
	}
	sort.Strings(jsonFiles)

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
	seen := map[string]bool{}
	for _, p := range presets {
		if seen[p.ID] {
			return nil, fmt.Errorf("duplicate preset %s", p.ID)
		}
		seen[p.ID] = true
	}
	sort.Slice(presets, func(i, j int) bool { return presets[i].ID < presets[j].ID })
	return presets, nil
}
