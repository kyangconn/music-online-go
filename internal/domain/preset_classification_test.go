package domain

import "testing"

func TestTokenizeGenresNormalizesAliasesAndHostileControls(t *testing.T) {
	tokens := TokenizeGenres(StringList{" D&B ; Melodic-Dubstep, Synth_Wave\x00 ", "dnb"})
	if len(tokens) != 4 {
		t.Fatalf("tokens = %#v", tokens)
	}
	want := []string{"drum and bass", "melodic dubstep", "synthwave", "drum and bass"}
	for index, token := range tokens {
		if token.Canonical != want[index] {
			t.Fatalf("token %d canonical = %q, want %q", index, token.Canonical, want[index])
		}
	}
	normalized := NormalizeGenreTokens(StringList{"D&B", "dnb", "Hard-Style"})
	if len(normalized) != 2 || normalized[0] != "drum and bass" || normalized[1] != "hardstyle" {
		t.Fatalf("normalized genre tokens = %#v", normalized)
	}
}

func TestBuildMusicPresetProjectionCrossStylePriority(t *testing.T) {
	tests := []struct {
		name   string
		genres StringList
		want   string
	}{
		{name: "chillstep outranks generic dubstep", genres: StringList{"Chill-Step; Dubstep"}, want: PresetCalmFlow},
		{name: "melodic dubstep", genres: StringList{"Melodic Dubstep"}, want: PresetKineticPulse},
		{name: "generic dubstep", genres: StringList{"Dubstep"}, want: PresetBassImpact},
		{name: "drum and bass alias", genres: StringList{"D&B"}, want: PresetKineticPulse},
		{name: "trance", genres: StringList{"Trance"}, want: PresetCosmicDrift},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classification, scores := BuildMusicPresetProjection(&Music{ID: 1, Genres: test.genres, MetadataRevision: 3}, DefaultPresetRulePolicy())
			if classification.Status != PresetStatusClassified || classification.AutomaticPreset != test.want || len(scores) != 4 {
				t.Fatalf("classification = %+v, scores=%+v", classification, scores)
			}
			if classification.MetadataRevision != 3 || classification.RuleVersion != PresetRuleVersion {
				t.Fatalf("classification version = %+v", classification)
			}
		})
	}
}

func TestBuildMusicPresetProjectionAbstainsForInstrumentalAndConflicts(t *testing.T) {
	instrumental, scores := BuildMusicPresetProjection(
		&Music{ID: 1, Genres: StringList{"Instrumental"}}, DefaultPresetRulePolicy(),
	)
	if instrumental.Status != PresetStatusUnclassified || instrumental.PrimaryPreset != "" || instrumental.AutomaticPreset != "" {
		t.Fatalf("instrumental classification = %+v", instrumental)
	}
	for _, score := range scores {
		if score.Score != 0 {
			t.Fatalf("instrumental score = %+v", score)
		}
	}

	conflict, _ := BuildMusicPresetProjection(
		&Music{ID: 2, Genres: StringList{"Ambient; Progressive House"}}, DefaultPresetRulePolicy(),
	)
	if conflict.Status != PresetStatusNeedsReview || conflict.PrimaryPreset != PresetCosmicDrift || conflict.AutomaticPreset != "" {
		t.Fatalf("conflicting classification = %+v", conflict)
	}
}

func TestBuildMusicPresetProjectionWithAudioUsesBoundedEvidencePriority(t *testing.T) {
	audio := &PresetAudioEvidence{
		AnalysisID: 41,
		Features: map[string]float64{
			"energy": 0.9, "danceability": 0.95, "pulse_clarity": 0.9,
			"bass_energy_ratio": 0.95, "sub_bass_energy_ratio": 0.9, "drop_contrast": 0.9,
		},
		ModelLabels: map[string]float64{"brostep": 0.99, "trance": 0.8},
	}
	policy := DefaultPresetRulePolicy()
	policy.ReviewMargin = 0.2
	classification, scores := BuildMusicPresetProjectionWithAudio(
		&Music{ID: 1, Genres: StringList{"Chillstep"}, MetadataRevision: 3}, audio, policy,
	)
	if classification.RuleVersion != PresetHybridRuleVersion || classification.AudioAnalysisID == nil || *classification.AudioAnalysisID != 41 {
		t.Fatalf("hybrid provenance = %+v", classification)
	}
	if classification.AutomaticPreset != "" || classification.Status != PresetStatusNeedsReview || classification.PrimaryPreset != PresetCalmFlow {
		t.Fatalf("conflicting lower-priority evidence should route to review: %+v", classification)
	}
	if len(scores) != 4 || scores[0].PresetID != PresetCalmFlow || scores[0].Score < 0.95 {
		t.Fatalf("hybrid scores = %+v", scores)
	}
}

func TestBuildMusicPresetProjectionWithAudioCanClassifyModelAndGatedInstrumentalEvidence(t *testing.T) {
	model, _ := BuildMusicPresetProjectionWithAudio(
		&Music{ID: 1},
		&PresetAudioEvidence{AnalysisID: 1, Features: map[string]float64{}, ModelLabels: map[string]float64{"trance": 1}},
		DefaultPresetRulePolicy(),
	)
	if model.Status != PresetStatusClassified || model.AutomaticPreset != PresetCosmicDrift {
		t.Fatalf("high-confidence model classification = %+v", model)
	}

	calm, _ := BuildMusicPresetProjectionWithAudio(
		&Music{ID: 2, Genres: StringList{"Instrumental"}},
		&PresetAudioEvidence{AnalysisID: 2, Features: map[string]float64{
			"instrumental_probability": 0.95, "energy": 0.1, "arousal": 0.1, "dynamic_smoothness": 0.9,
		}, ModelLabels: map[string]float64{}},
		DefaultPresetRulePolicy(),
	)
	if calm.Status != PresetStatusClassified || calm.AutomaticPreset != PresetCalmFlow {
		t.Fatalf("gated instrumental classification = %+v", calm)
	}

	aggressive, _ := BuildMusicPresetProjectionWithAudio(
		&Music{ID: 3, Genres: StringList{"Instrumental"}},
		&PresetAudioEvidence{AnalysisID: 3, Features: map[string]float64{
			"instrumental_probability": 0.99, "energy": 0.95, "arousal": 0.9,
		}, ModelLabels: map[string]float64{}},
		DefaultPresetRulePolicy(),
	)
	if aggressive.AutomaticPreset == PresetCalmFlow {
		t.Fatalf("aggressive instrumental must not be classified as calm: %+v", aggressive)
	}
}

func TestDecodePresetAudioEvidenceExtractsStableFeatureContract(t *testing.T) {
	features, err := NewJSONDocument(map[string]any{
		"Energy":         0.3,
		"BPM Confidence": 0.7,
		"bpm_candidates": []map[string]float64{
			{"bpm": 70, "confidence": 0.8},
			{"bpm": 900, "confidence": 1},
		},
		"vendor_nested": map[string]any{"ignored": 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	labels, err := NewJSONDocument(map[string]float64{"Trance": 0.8})
	if err != nil {
		t.Fatal(err)
	}
	audio, err := DecodePresetAudioEvidence(&MusicAudioAnalysis{
		ID: 8, Status: AnalysisStatusSucceeded, Features: features, ModelLabels: labels,
	})
	if err != nil {
		t.Fatal(err)
	}
	if audio.Features["energy"] != 0.3 || len(audio.BPMCandidates) != 1 || audio.ModelLabels["Trance"] != 0.8 {
		t.Fatalf("decoded audio evidence = %+v", audio)
	}

	legacyFeatures, err := NewJSONDocument(map[string]any{
		"bpm_confidence": 0.6,
		"BPM-Candidates": []any{"invalid", 64.0, map[string]float64{"bpm": 128}},
	})
	if err != nil {
		t.Fatal(err)
	}
	legacy, err := DecodePresetAudioEvidence(&MusicAudioAnalysis{
		ID: 9, Status: AnalysisStatusSucceeded, Features: legacyFeatures,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy.BPMCandidates) != 2 || legacy.BPMCandidates[0].BPM != 64 ||
		legacy.BPMCandidates[1].Confidence != 0.6 || len(legacy.ModelLabels) != 0 {
		t.Fatalf("legacy BPM candidates = %+v", legacy)
	}
}

func TestPresetEvidenceListRoundTrip(t *testing.T) {
	values := PresetEvidenceList{{Source: "genre", Key: "trance", Weight: 0.92}}
	encoded, err := values.Value()
	if err != nil {
		t.Fatalf("encode evidence: %v", err)
	}
	var decoded PresetEvidenceList
	if err := decoded.Scan(encoded); err != nil {
		t.Fatalf("decode evidence: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Key != "trance" || decoded[0].Weight != 0.92 {
		t.Fatalf("decoded evidence = %#v", decoded)
	}
}
