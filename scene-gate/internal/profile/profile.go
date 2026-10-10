package profile

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// CameraProfile is the resolved config for one source_id.
type CameraProfile struct {
	Match                    string
	Description              string
	L1Prior                  *string
	SpecialistsAlways        []string
	SpecialistsAllowed       []string
	SkipVisual               bool
	AudioPrimary             bool
	VisualAbstainThreshold   float32
	AudioConfidenceThreshold float32
}

// AudioIntentRule maps phrases → L2.
type AudioIntentRule struct {
	Phrases    []string
	L2         string
	Confidence float32
}

// Store holds defaults + profiles + intent rules.
type Store struct {
	Defaults     CameraProfile
	Profiles     []CameraProfile
	AudioIntents []AudioIntentRule
}

type yamlFile struct {
	Version  int `yaml:"version"`
	Defaults struct {
		L1Prior                  *string  `yaml:"l1_prior"`
		SpecialistsAlways        []string `yaml:"specialists_always"`
		SpecialistsAllowed       []string `yaml:"specialists_allowed"`
		SkipVisual               bool     `yaml:"skip_visual"`
		AudioPrimary             bool     `yaml:"audio_primary"`
		VisualAbstainThreshold   float32  `yaml:"visual_abstain_threshold"`
		AudioConfidenceThreshold float32  `yaml:"audio_confidence_threshold"`
	} `yaml:"defaults"`
	Profiles []struct {
		Match                    string   `yaml:"match"`
		Description              string   `yaml:"description"`
		L1Prior                  *string  `yaml:"l1_prior"`
		SpecialistsAlways        []string `yaml:"specialists_always"`
		SpecialistsAllowed       []string `yaml:"specialists_allowed"`
		SkipVisual               bool     `yaml:"skip_visual"`
		AudioPrimary             bool     `yaml:"audio_primary"`
		VisualAbstainThreshold   *float32 `yaml:"visual_abstain_threshold"`
		AudioConfidenceThreshold *float32 `yaml:"audio_confidence_threshold"`
	} `yaml:"profiles"`
	AudioIntents []struct {
		Phrases    []string `yaml:"phrases"`
		L2         string   `yaml:"l2"`
		Confidence float32  `yaml:"confidence"`
	} `yaml:"audio_intents"`
}

// Load reads camera_profiles.yaml.
func Load(path string) (*Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profiles %s: %w", path, err)
	}
	var raw yamlFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse profiles: %w", err)
	}

s := &Store{
		Defaults: CameraProfile{
			L1Prior:                  raw.Defaults.L1Prior,
			SpecialistsAlways:        raw.Defaults.SpecialistsAlways,
			SpecialistsAllowed:       raw.Defaults.SpecialistsAllowed,
			SkipVisual:               raw.Defaults.SkipVisual,
			AudioPrimary:             raw.Defaults.AudioPrimary,
			VisualAbstainThreshold:   raw.Defaults.VisualAbstainThreshold,
			AudioConfidenceThreshold: raw.Defaults.AudioConfidenceThreshold,
		},
	}
	if s.Defaults.VisualAbstainThreshold == 0 {
		s.Defaults.VisualAbstainThreshold = 0.60
	}
	if s.Defaults.AudioConfidenceThreshold == 0 {
		s.Defaults.AudioConfidenceThreshold = 0.70
	}
	for _, p := range raw.Profiles {
		cp := CameraProfile{
			Match:                    p.Match,
			Description:              p.Description,
			L1Prior:                  p.L1Prior,
			SpecialistsAlways:        p.SpecialistsAlways,
			SpecialistsAllowed:       p.SpecialistsAllowed,
			SkipVisual:               p.SkipVisual,
			AudioPrimary:             p.AudioPrimary,
			VisualAbstainThreshold:   s.Defaults.VisualAbstainThreshold,
			AudioConfidenceThreshold: s.Defaults.AudioConfidenceThreshold,
		}
		if p.VisualAbstainThreshold != nil {
			cp.VisualAbstainThreshold = *p.VisualAbstainThreshold
		}
		if p.AudioConfidenceThreshold != nil {
			cp.AudioConfidenceThreshold = *p.AudioConfidenceThreshold
		}
		s.Profiles = append(s.Profiles, cp)
	}
	for _, r := range raw.AudioIntents {
		s.AudioIntents = append(s.AudioIntents, AudioIntentRule{
			Phrases:    r.Phrases,
			L2:         r.L2,
			Confidence: r.Confidence,
		})
	}
	return s, nil
}

// Resolve returns the best matching profile for source_id (or defaults).
func (s *Store) Resolve(sourceID string) CameraProfile {
	for _, p := range s.Profiles {
		if matchPattern(p.Match, sourceID) {
			return p
		}
	}
	return s.Defaults
}

func matchPattern(pattern, sourceID string) bool {
	if pattern == sourceID {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(sourceID, prefix)
	}
	return false
}
