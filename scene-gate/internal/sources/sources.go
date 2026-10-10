package sources

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Entry is one registered source_id.
type Entry struct {
	ID          string
	Description string
}

type yamlFile struct {
	Version int `yaml:"version"`
	Sources []struct {
		ID          string `yaml:"id"`
		Description string `yaml:"description"`
	} `yaml:"sources"`
}

// Load reads domain/camera_sources.yaml. Missing file → empty list (not an error
// when path is empty); invalid YAML is an error.
func Load(path string) ([]Entry, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sources %s: %w", path, err)
	}
	var raw yamlFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse sources: %w", err)
	}
	out := make([]Entry, 0, len(raw.Sources))
	for _, s := range raw.Sources {
		if s.ID == "" {
			continue
		}
		out = append(out, Entry{ID: s.ID, Description: s.Description})
	}
	return out, nil
}
