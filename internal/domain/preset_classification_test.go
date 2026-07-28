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
