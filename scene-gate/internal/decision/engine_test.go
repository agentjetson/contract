package decision

import (
	"testing"

	"github.com/agentjetson/core/scene-gate/internal/profile"
	"github.com/agentjetson/core/scene-gate/internal/taxonomy"
)

func testTax() *taxonomy.Taxonomy {
	return &taxonomy.Taxonomy{
		L1Of: map[string]string{
			"driving":      "roadway",
			"traffic_stop": "roadway",
			"crash":        "roadway",
			"entry":        "facility",
			"walking":      "sidewalk",
			"gathering":    "sidewalk",
			"abstain":      "unknown",
		},
		SpecialistsOf: map[string][]string{
			"roadway":  {"alpr", "speed", "vehicle_attr"},
			"facility": {"face", "person_attr"},
			"sidewalk": {"person_attr"},
			"unknown":  {},
		},
		KnownL2: map[string]bool{
			"driving": true, "traffic_stop": true, "crash": true,
			"entry": true, "walking": true, "gathering": true, "abstain": true,
		},
	}
}

func testStore() *profile.Store {
	roadway := "roadway"
	facility := "facility"
	return &profile.Store{
		Defaults: profile.CameraProfile{
			VisualAbstainThreshold:   0.60,
			AudioConfidenceThreshold: 0.70,
		},
		Profiles: []profile.CameraProfile{
			{
				Match:                    "front-*",
				L1Prior:                  &roadway,
				SpecialistsAlways:        []string{"alpr", "speed", "vehicle_attr"},
				SkipVisual:               true,
				AudioConfidenceThreshold: 0.70,
			},
			{
				Match:                    "cabin-*",
				L1Prior:                  &facility,
				SpecialistsAlways:        []string{"face", "person_attr"},
				SkipVisual:               true,
				AudioConfidenceThreshold: 0.70,
			},
			{
				Match:                    "bodycam-*",
				SpecialistsAllowed:       []string{"alpr", "person_attr", "face", "vehicle_attr", "speed"},
				AudioPrimary:             true,
				VisualAbstainThreshold:   0.75,
				AudioConfidenceThreshold: 0.65,
			},
		},
		AudioIntents: []profile.AudioIntentRule{
			{Phrases: []string{"initiating traffic stop", "traffic stop"}, L2: "traffic_stop", Confidence: 0.90},
			{Phrases: []string{"you're free to go"}, L2: "driving", Confidence: 0.80},
		},
	}
}

func TestOnFrame_FrontCam_SkipVisual(t *testing.T) {
	e := New(testTax(), testStore())
	r := e.OnFrame("front-cam-01")
	if r.Action != EmitNow {
		t.Fatalf("want EmitNow, got %s (%s)", r.Action, r.Reason)
	}
	if r.L1 != "roadway" || r.L2 != "driving" {
		t.Fatalf("want roadway/driving, got %s/%s", r.L1, r.L2)
	}
	if r.Backend != "profile" || !r.SkipVisual {
		t.Fatalf("backend/skip mismatch: %s %v", r.Backend, r.SkipVisual)
	}
	if len(r.Specialists) != 3 {
		t.Fatalf("specialists: %v", r.Specialists)
	}
}

func TestOnFrame_Bodycam_Forward(t *testing.T) {
	e := New(testTax(), testStore())
	r := e.OnFrame("bodycam-12")
	if r.Action != ForwardVisual {
		t.Fatalf("want ForwardVisual, got %s (%s)", r.Action, r.Reason)
	}
}

func TestOnAudio_TrafficStop(t *testing.T) {
	e := New(testTax(), testStore())
	r := e.OnAudio("bodycam-12", "Unit 12 initiating traffic stop on northbound")
	if r.Action != EmitNow {
		t.Fatalf("want EmitNow, got %s (%s)", r.Action, r.Reason)
	}
	if r.L2 != "traffic_stop" || r.Backend != "audio" {
		t.Fatalf("got %s/%s", r.L2, r.Backend)
	}
	if r.Confidence < 0.85 {
		t.Fatalf("confidence %v", r.Confidence)
	}
}

func TestOnAudio_NoMatch_Forward(t *testing.T) {
	e := New(testTax(), testStore())
	r := e.OnAudio("bodycam-12", "radio check one two")
	if r.Action != ForwardVisual {
		t.Fatalf("want ForwardVisual, got %s", r.Action)
	}
}
