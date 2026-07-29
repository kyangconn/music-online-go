package analysisbench

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

func TestEvaluateReportsClassificationAndResourceMetrics(t *testing.T) {
	manifest, digest := benchmarkFixtureManifest(t)
	run := &CandidateRun{
		SchemaVersion: SchemaVersion, ManifestSHA256: digest,
		Candidate: CandidateIdentity{
			ID: "fixture", ImplementationVersion: "1.0", ModelVersion: "model-1",
			ModelDigest: "sha256:" + strings.Repeat("b", 64),
			RuntimeKind: "container", CodeLicense: "Apache-2.0", ModelLicense: "Apache-2.0",
			ImageReference: "example.invalid/fixture@sha256:" + strings.Repeat("a", 64),
			ImageDigest:    "sha256:" + strings.Repeat("a", 64),
			ImageSizeBytes: 300 << 20, BaseImageSizeBytes: 100 << 20,
		},
		Samples: []CandidateSample{
			benchmarkSample("calm", 10, 100<<20, 0.90, 0.10, 0.05, 0.05),
			benchmarkSample("kinetic", 20, 110<<20, 0.05, 0.85, 0.05, 0.05),
			benchmarkSample("cosmic", 30, 120<<20, 0.05, 0.65, 0.70, 0.05),
			benchmarkSample("bass", 40, 130<<20, 0.05, 0.90, 0.05, 0.75),
			benchmarkSample("negative", 50, 90<<20, 0.20, 0.20, 0.20, 0.20),
			benchmarkSample("calm-calibration", 1000, 500<<20, 0.90, 0.10, 0.05, 0.05),
			benchmarkSample("kinetic-calibration", 1000, 500<<20, 0.05, 0.85, 0.05, 0.05),
			benchmarkSample("cosmic-calibration", 1000, 500<<20, 0.05, 0.05, 0.90, 0.05),
			benchmarkSample("bass-calibration", 1000, 500<<20, 0.05, 0.05, 0.05, 0.90),
			benchmarkSample("negative-calibration", 1000, 500<<20, 0.20, 0.20, 0.20, 0.20),
		},
	}
	report, err := Evaluate(manifest, digest, []*CandidateRun{run})
	if err != nil {
		t.Fatalf("evaluate fixture: %v", err)
	}
	got := report.Candidates[0]
	if report.CalibrationSamples != 5 || report.EvaluationSamples != 5 || got.SampleCount != 5 {
		t.Fatalf("split counts: report=%+v candidate=%+v", report, got)
	}
	assertClose(t, got.MacroF1, 5.0/12.0)
	assertClose(t, got.HighConfidencePrecision, 2.0/3.0)
	if got.HighConfidenceCount != 3 {
		t.Fatalf("high-confidence count = %d", got.HighConfidenceCount)
	}
	assertClose(t, got.Coverage, 0.6)
	assertClose(t, got.AbstentionRate, 0.4)
	assertClose(t, got.MeanCPUTimeMS, 30)
	assertClose(t, got.P95CPUTimeMS, 50)
	if got.PeakMemoryBytes != 130<<20 || got.ImageDeltaBytes != 200<<20 {
		t.Fatalf("resource metrics = %+v", got)
	}
	if got.ConfusionMatrix["cosmic_drift"][PredictionAbstained] != 1 ||
		got.ConfusionMatrix[ExpectedNone][PredictionAbstained] != 1 ||
		got.ConfusionMatrix["bass_impact"]["kinetic_pulse"] != 1 {
		t.Fatalf("confusion matrix = %+v", got.ConfusionMatrix)
	}
	kinetic := got.PerClass["kinetic_pulse"]
	if kinetic.Support != 1 || kinetic.Predicted != 2 || kinetic.TruePositive != 1 {
		t.Fatalf("kinetic metrics = %+v", kinetic)
	}
}

func TestEvaluateRejectsMismatchedAndMalformedCandidateResults(t *testing.T) {
	manifest, digest := benchmarkFixtureManifest(t)
	validIdentity := CandidateIdentity{
		ID: "fixture", ImplementationVersion: "1", ModelVersion: "1", RuntimeKind: "in_process",
		ModelDigest: "none", CodeLicense: "MIT", ModelLicense: "N/A",
	}
	tests := []struct {
		name string
		run  CandidateRun
	}{
		{
			name: "manifest digest",
			run:  CandidateRun{SchemaVersion: SchemaVersion, ManifestSHA256: strings.Repeat("0", 64), Candidate: validIdentity},
		},
		{
			name: "missing samples",
			run:  CandidateRun{SchemaVersion: SchemaVersion, ManifestSHA256: digest, Candidate: validIdentity},
		},
		{
			name: "invalid score",
			run: CandidateRun{
				SchemaVersion: SchemaVersion, ManifestSHA256: digest, Candidate: validIdentity,
				Samples: []CandidateSample{{
					ID: "calm", Scores: map[string]float64{
						"calm_flow": math.Inf(1), "kinetic_pulse": 0, "cosmic_drift": 0, "bass_impact": 0,
					},
				}},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := Evaluate(manifest, digest, []*CandidateRun{&test.run}); err == nil {
				t.Fatal("malformed candidate result should fail")
			}
		})
	}
}

func TestDecodeManifestIsStrictAndProducesRawDigest(t *testing.T) {
	fixture, _ := benchmarkFixtureManifest(t)
	raw, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(raw)
	manifest, digest, err := DecodeManifest(strings.NewReader(encoded))
	if err != nil || manifest.ID != "fixture" || !validSHA256(digest) {
		t.Fatalf("decode manifest: manifest=%+v digest=%q err=%v", manifest, digest, err)
	}
	if _, _, err := DecodeManifest(strings.NewReader(strings.TrimSuffix(encoded, "}") + `,"unknown":true}`)); err == nil {
		t.Fatal("unknown manifest field should fail")
	}
}

func TestManifestRejectsGroupLeakageAcrossSplits(t *testing.T) {
	manifest, _ := benchmarkFixtureManifest(t)
	manifest.Samples[0].Groups = []string{"shared-artist"}
	manifest.Samples[5].Groups = []string{"shared-artist"}
	if err := validateManifest(manifest); err == nil || !strings.Contains(err.Error(), "crosses") {
		t.Fatalf("group leakage error = %v", err)
	}
}

func TestMarkdownIncludesComparisonAndConfusionMatrix(t *testing.T) {
	manifest, digest := benchmarkFixtureManifest(t)
	run := &CandidateRun{
		SchemaVersion: SchemaVersion, ManifestSHA256: digest,
		Candidate: CandidateIdentity{
			ID: "fixture", ImplementationVersion: "1", ModelVersion: "1", RuntimeKind: "in_process",
			ModelDigest: "none", CodeLicense: "MIT", ModelLicense: "N/A",
		},
	}
	for _, sample := range manifest.Samples {
		scores := map[string]float64{"calm_flow": 0.1, "kinetic_pulse": 0.1, "cosmic_drift": 0.1, "bass_impact": 0.1}
		if sample.ExpectedPreset != ExpectedNone {
			scores[sample.ExpectedPreset] = 0.9
		}
		run.Samples = append(run.Samples, CandidateSample{ID: sample.ID, Scores: scores})
	}
	report, err := Evaluate(manifest, digest, []*CandidateRun{run})
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Markdown(report)
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"Macro-F1", "High-confidence precision", "held-out evaluation", "Confusion matrix", "calm_flow", "fixture"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("Markdown report is missing %q:\n%s", expected, rendered)
		}
	}
}

func benchmarkFixtureManifest(t *testing.T) (*Manifest, string) {
	t.Helper()
	manifest := Manifest{
		SchemaVersion: SchemaVersion, ID: "fixture", Revision: "r1",
		AutoThreshold: 0.65, ReviewMargin: 0.12, HighConfidenceThreshold: 0.8,
		Samples: []GoldSample{
			{ID: "calm-calibration", AudioRef: "private/calm-calibration.flac", ExpectedPreset: "calm_flow", Split: SplitCalibration},
			{ID: "kinetic-calibration", AudioRef: "private/kinetic-calibration.flac", ExpectedPreset: "kinetic_pulse", Split: SplitCalibration},
			{ID: "cosmic-calibration", AudioRef: "private/cosmic-calibration.flac", ExpectedPreset: "cosmic_drift", Split: SplitCalibration},
			{ID: "bass-calibration", AudioRef: "private/bass-calibration.flac", ExpectedPreset: "bass_impact", Split: SplitCalibration},
			{ID: "negative-calibration", AudioRef: "private/negative-calibration.flac", ExpectedPreset: ExpectedNone, Split: SplitCalibration},
			{ID: "calm", AudioRef: "private/calm.flac", ExpectedPreset: "calm_flow", Split: SplitEvaluation},
			{ID: "kinetic", AudioRef: "private/kinetic.flac", ExpectedPreset: "kinetic_pulse", Split: SplitEvaluation},
			{ID: "cosmic", AudioRef: "private/cosmic.flac", ExpectedPreset: "cosmic_drift", Split: SplitEvaluation},
			{ID: "bass", AudioRef: "private/bass.flac", ExpectedPreset: "bass_impact", Split: SplitEvaluation},
			{ID: "negative", AudioRef: "private/negative.flac", ExpectedPreset: ExpectedNone, Split: SplitEvaluation},
		},
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	decoded, digest, err := DecodeManifest(strings.NewReader(string(encoded)))
	if err != nil {
		t.Fatal(err)
	}
	return decoded, digest
}

func benchmarkSample(id string, cpu float64, memory int64, calm, kinetic, cosmic, bass float64) CandidateSample {
	return CandidateSample{
		ID: id, CPUTimeMS: cpu, PeakMemoryBytes: memory,
		Scores: map[string]float64{
			"calm_flow": calm, "kinetic_pulse": kinetic, "cosmic_drift": cosmic, "bass_impact": bass,
		},
	}
}

func assertClose(t *testing.T, actual, expected float64) {
	t.Helper()
	if math.Abs(actual-expected) > 1e-9 {
		t.Fatalf("value = %.12f, want %.12f", actual, expected)
	}
}
