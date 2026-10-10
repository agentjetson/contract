package decision

import (
	"fmt"

	"github.com/agentjetson/core/scene-gate/internal/intent"
	"github.com/agentjetson/core/scene-gate/internal/profile"
	"github.com/agentjetson/core/scene-gate/internal/taxonomy"
)

// Action is the gate decision.
type Action int

const (
	EmitNow Action = iota
	ForwardVisual
	Abstain
)

func (a Action) String() string {
	switch a {
	case EmitNow:
		return "EmitNow"
	case ForwardVisual:
		return "ForwardVisual"
	case Abstain:
		return "Abstain"
	default:
		return "Unknown"
	}
}

// Result is a gate decision ready to publish or forward.
type Result struct {
	Action      Action
	SourceID    string
	L1          string
	L2          string
	Confidence  float32
	Backend     string // "profile" | "audio" | ...
	Specialists []string
	SkipVisual  bool
	Reason      string
}

// Engine implements the decision table from the C++ prototype.
type Engine struct {
	tax     *taxonomy.Taxonomy
	store   *profile.Store
	matcher *intent.Matcher
}

func New(tax *taxonomy.Taxonomy, store *profile.Store) *Engine {
	return &Engine{
		tax:     tax,
		store:   store,
		matcher: intent.New(store.AudioIntents),
	}
}

func (e *Engine) pickSpecialists(prof profile.CameraProfile, l1 string) []string {
	if len(prof.SpecialistsAlways) > 0 {
		return append([]string{}, prof.SpecialistsAlways...)
	}
	fromTax := e.tax.Specialists(l1)
	if len(prof.SpecialistsAllowed) == 0 {
		return fromTax
	}
	allowed := make(map[string]bool, len(prof.SpecialistsAllowed))
	for _, a := range prof.SpecialistsAllowed {
		allowed[a] = true
	}
	var out []string
	seen := make(map[string]bool)
	for _, s := range fromTax {
		if allowed[s] && !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	// Also allow specialists in allowed even if not in current L1
	for _, a := range prof.SpecialistsAllowed {
		if !seen[a] {
			out = append(out, a)
			seen[a] = true
		}
	}
	return out
}

// OnAudio is called when a transcript arrives for a source.
func (e *Engine) OnAudio(sourceID, transcript string) Result {
	prof := e.store.Resolve(sourceID)
	hit := e.matcher.Match(transcript)
	if hit == nil {
		r := Result{
			Action:   ForwardVisual,
			SourceID: sourceID,
			Reason:   "no audio intent matched",
		}
		if prof.AudioPrimary && prof.SkipVisual {
			r.Action = Abstain
			r.Reason = "audio_primary + skip_visual and no intent"
		}
		return r
	}
	if hit.Confidence < prof.AudioConfidenceThreshold {
		return Result{
			Action:   ForwardVisual,
			SourceID: sourceID,
			Reason:   "audio intent below threshold",
		}
	}
	l1 := e.tax.L1OfLabel(hit.L2)
	specs := e.pickSpecialists(prof, l1)
	return Result{
		Action:      EmitNow,
		SourceID:    sourceID,
		L1:          l1,
		L2:          hit.L2,
		Confidence:  hit.Confidence,
		Backend:     "audio",
		Specialists: specs,
		SkipVisual:  true,
		Reason:      fmt.Sprintf("audio intent: %q → %s", hit.MatchedPhrase, hit.L2),
	}
}

// OnFrame is called when a frame / visual request arrives for a source.
func (e *Engine) OnFrame(sourceID string) Result {
	prof := e.store.Resolve(sourceID)
	if !prof.SkipVisual {
		return Result{
			Action:   ForwardVisual,
			SourceID: sourceID,
			Reason:   "profile allows visual routing",
		}
	}
	if prof.L1Prior == nil {
		return Result{
			Action:   Abstain,
			SourceID: sourceID,
			Reason:   "skip_visual and no l1_prior",
		}
	}
	l1 := *prof.L1Prior
	l2 := "abstain"
	switch l1 {
	case "roadway":
		l2 = "driving"
	case "facility":
		l2 = "entry"
	case "sidewalk":
		l2 = "walking"
	}
	specs := e.pickSpecialists(prof, l1)
	return Result{
		Action:      EmitNow,
		SourceID:    sourceID,
		L1:          l1,
		L2:          l2,
		Confidence:  0.55,
		Backend:     "profile",
		Specialists: specs,
		SkipVisual:  true,
		Reason:      fmt.Sprintf("skip_visual + l1_prior=%s", l1),
	}
}
