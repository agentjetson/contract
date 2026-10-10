package taxonomy

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Taxonomy is the hierarchical L1 → L2 + specialists map.
type Taxonomy struct {
	L1Of          map[string]string   // l2 → l1
	SpecialistsOf map[string][]string // l1 → specialists
	KnownL2       map[string]bool
}

type yamlTaxonomy struct {
	Version int `yaml:"version"`
	Levels  map[string]struct {
		Description string              `yaml:"description"`
		Specialists []string            `yaml:"specialists"`
		L2          map[string]struct {
			Description string `yaml:"description"`
		} `yaml:"l2"`
	} `yaml:"levels"`
}

// Load reads domain/taxonomy.yaml.
func Load(path string) (*Taxonomy, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read taxonomy %s: %w", path, err)
	}
	var raw yamlTaxonomy
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse taxonomy: %w", err)
	}
	t := &Taxonomy{
		L1Of:          make(map[string]string),
		SpecialistsOf: make(map[string][]string),
		KnownL2:       make(map[string]bool),
	}
	for l1, level := range raw.Levels {
		t.SpecialistsOf[l1] = append([]string{}, level.Specialists...)
		for l2 := range level.L2 {
			t.L1Of[l2] = l1
			t.KnownL2[l2] = true
		}
	}
	return t, nil
}

func (t *Taxonomy) L1OfLabel(l2 string) string {
	if l1, ok := t.L1Of[l2]; ok {
		return l1
	}
	return "unknown"
}

func (t *Taxonomy) Specialists(l1 string) []string {
	return append([]string{}, t.SpecialistsOf[l1]...)
}

func (t *Taxonomy) Known(l2 string) bool {
	return t.KnownL2[l2]
}
