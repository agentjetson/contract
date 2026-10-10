package intent

import (
	"strings"

	"github.com/agentjetson/core/scene-gate/internal/profile"
)

// Hit is a matched audio intent.
type Hit struct {
	L2            string
	Confidence    float32
	MatchedPhrase string
}

// Matcher is a simple case-insensitive phrase matcher.
type Matcher struct {
	rules []profile.AudioIntentRule
}

func New(rules []profile.AudioIntentRule) *Matcher {
	return &Matcher{rules: rules}
}

// Match returns the first rule whose phrase appears in transcript.
func (m *Matcher) Match(transcript string) *Hit {
	lower := strings.ToLower(transcript)
	for _, r := range m.rules {
		for _, p := range r.Phrases {
			if strings.Contains(lower, strings.ToLower(p)) {
				return &Hit{
					L2:            r.L2,
					Confidence:    r.Confidence,
					MatchedPhrase: p,
				}
			}
		}
	}
	return nil
}
