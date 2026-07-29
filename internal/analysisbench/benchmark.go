package analysisbench

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
)

const (
	SchemaVersion             = 1
	ExpectedNone              = "none"
	PredictionAbstained       = "abstain"
	SplitCalibration          = "calibration"
	SplitEvaluation           = "evaluation"
	maxManifestBytes    int64 = 4 << 20
	maxResultBytes      int64 = 32 << 20
)

var presetIDs = [...]string{"calm_flow", "kinetic_pulse", "cosmic_drift", "bass_impact"}

type Manifest struct {
	SchemaVersion           int          `json:"schema_version"`
	ID                      string       `json:"id"`
	Revision                string       `json:"revision"`
	AutoThreshold           float64      `json:"auto_threshold"`
	ReviewMargin            float64      `json:"review_margin"`
	HighConfidenceThreshold float64      `json:"high_confidence_threshold"`
	Samples                 []GoldSample `json:"samples"`
}

type GoldSample struct {
	ID             string   `json:"id"`
	AudioRef       string   `json:"audio_ref"`
	ExpectedPreset string   `json:"expected_preset"`
	Split          string   `json:"split"`
	Groups         []string `json:"groups,omitempty"`
}

type CandidateRun struct {
	SchemaVersion  int               `json:"schema_version"`
	ManifestSHA256 string            `json:"manifest_sha256"`
	Candidate      CandidateIdentity `json:"candidate"`
	Samples        []CandidateSample `json:"samples"`
}

type CandidateIdentity struct {
	ID                    string `json:"id"`
	ImplementationVersion string `json:"implementation_version"`
	ModelVersion          string `json:"model_version"`
	ModelDigest           string `json:"model_digest"`
	RuntimeKind           string `json:"runtime_kind"`
	CodeLicense           string `json:"code_license"`
	ModelLicense          string `json:"model_license"`
	ImageReference        string `json:"image_reference,omitempty"`
	ImageDigest           string `json:"image_digest,omitempty"`
	ImageSizeBytes        int64  `json:"image_size_bytes"`
	BaseImageSizeBytes    int64  `json:"base_image_size_bytes"`
}

type CandidateSample struct {
	ID              string             `json:"id"`
	Scores          map[string]float64 `json:"scores,omitempty"`
	CPUTimeMS       float64            `json:"cpu_time_ms"`
	PeakMemoryBytes int64              `json:"peak_memory_bytes"`
	ErrorCode       string             `json:"error_code,omitempty"`
}

type Report struct {
	SchemaVersion      int               `json:"schema_version"`
	ManifestID         string            `json:"manifest_id"`
	ManifestRevision   string            `json:"manifest_revision"`
	ManifestSHA256     string            `json:"manifest_sha256"`
	CalibrationSamples int               `json:"calibration_samples"`
	EvaluationSamples  int               `json:"evaluation_samples"`
	Candidates         []CandidateReport `json:"candidates"`
}

type CandidateReport struct {
	Candidate               CandidateIdentity         `json:"candidate"`
	SampleCount             int                       `json:"sample_count"`
	PerClass                map[string]ClassMetrics   `json:"per_class"`
	MacroF1                 float64                   `json:"macro_f1"`
	HighConfidencePrecision float64                   `json:"high_confidence_precision"`
	HighConfidenceCount     int                       `json:"high_confidence_count"`
	Coverage                float64                   `json:"coverage"`
	AbstentionRate          float64                   `json:"abstention_rate"`
	FailureRate             float64                   `json:"failure_rate"`
	MeanCPUTimeMS           float64                   `json:"mean_cpu_time_ms"`
	P95CPUTimeMS            float64                   `json:"p95_cpu_time_ms"`
	PeakMemoryBytes         int64                     `json:"peak_memory_bytes"`
	ImageDeltaBytes         int64                     `json:"image_delta_bytes"`
	ConfusionMatrix         map[string]map[string]int `json:"confusion_matrix"`
}

type ClassMetrics struct {
	Support      int     `json:"support"`
	Predicted    int     `json:"predicted"`
	TruePositive int     `json:"true_positive"`
	Precision    float64 `json:"precision"`
	Recall       float64 `json:"recall"`
	F1           float64 `json:"f1"`
}

func DecodeManifest(reader io.Reader) (*Manifest, string, error) {
	manifest, raw, err := decodeBounded[Manifest](reader, maxManifestBytes)
	if err != nil {
		return nil, "", fmt.Errorf("decode benchmark manifest: %w", err)
	}
	if err := validateManifest(manifest); err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(raw)
	return manifest, hex.EncodeToString(digest[:]), nil
}

func DecodeCandidateRun(reader io.Reader) (*CandidateRun, error) {
	run, _, err := decodeBounded[CandidateRun](reader, maxResultBytes)
	if err != nil {
		return nil, fmt.Errorf("decode candidate result: %w", err)
	}
	if err := validateCandidateIdentity(run.Candidate); err != nil {
		return nil, err
	}
	if run.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("candidate result schema_version must be %d", SchemaVersion)
	}
	if !validSHA256(run.ManifestSHA256) {
		return nil, errors.New("candidate result manifest_sha256 must be a SHA-256 digest")
	}
	return run, nil
}

func Evaluate(manifest *Manifest, manifestDigest string, runs []*CandidateRun) (*Report, error) {
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	if !validSHA256(manifestDigest) {
		return nil, errors.New("manifest digest must be a SHA-256 digest")
	}
	if len(runs) == 0 {
		return nil, errors.New("at least one candidate result is required")
	}
	report := &Report{
		SchemaVersion: SchemaVersion, ManifestID: manifest.ID,
		ManifestRevision: manifest.Revision, ManifestSHA256: strings.ToLower(manifestDigest),
		Candidates: make([]CandidateReport, 0, len(runs)),
	}
	for _, sample := range manifest.Samples {
		switch sample.Split {
		case SplitCalibration:
			report.CalibrationSamples++
		case SplitEvaluation:
			report.EvaluationSamples++
		}
	}
	seenCandidates := make(map[string]struct{}, len(runs))
	for _, run := range runs {
		if run == nil {
			return nil, errors.New("candidate result is nil")
		}
		if _, exists := seenCandidates[run.Candidate.ID]; exists {
			return nil, fmt.Errorf("candidate %q appears more than once", run.Candidate.ID)
		}
		seenCandidates[run.Candidate.ID] = struct{}{}
		candidateReport, err := evaluateCandidate(manifest, manifestDigest, run)
		if err != nil {
			return nil, fmt.Errorf("evaluate candidate %q: %w", run.Candidate.ID, err)
		}
		report.Candidates = append(report.Candidates, *candidateReport)
	}
	sort.Slice(report.Candidates, func(left, right int) bool {
		return report.Candidates[left].Candidate.ID < report.Candidates[right].Candidate.ID
	})
	return report, nil
}

func evaluateCandidate(manifest *Manifest, manifestDigest string, run *CandidateRun) (*CandidateReport, error) {
	if run.SchemaVersion != SchemaVersion {
		return nil, fmt.Errorf("schema_version must be %d", SchemaVersion)
	}
	if !strings.EqualFold(run.ManifestSHA256, manifestDigest) {
		return nil, errors.New("manifest_sha256 does not match the evaluated gold set")
	}
	if err := validateCandidateIdentity(run.Candidate); err != nil {
		return nil, err
	}
	results := make(map[string]CandidateSample, len(run.Samples))
	for _, sample := range run.Samples {
		if _, duplicate := results[sample.ID]; duplicate {
			return nil, fmt.Errorf("sample %q appears more than once", sample.ID)
		}
		if err := validateCandidateSample(sample); err != nil {
			return nil, fmt.Errorf("sample %q: %w", sample.ID, err)
		}
		results[sample.ID] = sample
	}
	if len(results) != len(manifest.Samples) {
		return nil, fmt.Errorf("result has %d samples; manifest requires %d", len(results), len(manifest.Samples))
	}

	report := &CandidateReport{
		Candidate:       run.Candidate,
		PerClass:        make(map[string]ClassMetrics, len(presetIDs)),
		ConfusionMatrix: newConfusionMatrix(),
		ImageDeltaBytes: run.Candidate.ImageSizeBytes - run.Candidate.BaseImageSizeBytes,
	}
	type counters struct{ support, predicted, truePositive int }
	classCounters := make(map[string]*counters, len(presetIDs))
	for _, preset := range presetIDs {
		classCounters[preset] = &counters{}
	}
	cpuTimes := make([]float64, 0, len(manifest.Samples))
	covered, failed, highConfidence, highConfidenceCorrect := 0, 0, 0, 0
	for _, gold := range manifest.Samples {
		sample, exists := results[gold.ID]
		if !exists {
			return nil, fmt.Errorf("sample %q is missing", gold.ID)
		}
		delete(results, gold.ID)
		if gold.Split != SplitEvaluation {
			continue
		}
		report.SampleCount++
		prediction, topScore := predict(sample, manifest.AutoThreshold, manifest.ReviewMargin)
		if prediction != PredictionAbstained {
			covered++
			classCounters[prediction].predicted++
		}
		if sample.ErrorCode != "" {
			failed++
		}
		if gold.ExpectedPreset != ExpectedNone {
			classCounters[gold.ExpectedPreset].support++
		}
		if prediction == gold.ExpectedPreset {
			classCounters[prediction].truePositive++
		}
		if prediction != PredictionAbstained && topScore >= manifest.HighConfidenceThreshold {
			highConfidence++
			if prediction == gold.ExpectedPreset {
				highConfidenceCorrect++
			}
		}
		report.ConfusionMatrix[gold.ExpectedPreset][prediction]++
		cpuTimes = append(cpuTimes, sample.CPUTimeMS)
		if sample.PeakMemoryBytes > report.PeakMemoryBytes {
			report.PeakMemoryBytes = sample.PeakMemoryBytes
		}
	}
	if len(results) != 0 {
		for id := range results {
			return nil, fmt.Errorf("sample %q is not present in the manifest", id)
		}
	}
	for _, preset := range presetIDs {
		counter := classCounters[preset]
		precision := ratio(counter.truePositive, counter.predicted)
		recall := ratio(counter.truePositive, counter.support)
		f1 := 0.0
		if precision+recall > 0 {
			f1 = 2 * precision * recall / (precision + recall)
		}
		report.PerClass[preset] = ClassMetrics{
			Support: counter.support, Predicted: counter.predicted, TruePositive: counter.truePositive,
			Precision: precision, Recall: recall, F1: f1,
		}
		report.MacroF1 += f1
	}
	report.MacroF1 /= float64(len(presetIDs))
	report.HighConfidencePrecision = ratio(highConfidenceCorrect, highConfidence)
	report.HighConfidenceCount = highConfidence
	report.Coverage = ratio(covered, report.SampleCount)
	report.AbstentionRate = 1 - report.Coverage
	report.FailureRate = ratio(failed, report.SampleCount)
	report.MeanCPUTimeMS = mean(cpuTimes)
	report.P95CPUTimeMS = percentile(cpuTimes, 0.95)
	return report, nil
}

func validateManifest(manifest *Manifest) error {
	if manifest == nil {
		return errors.New("benchmark manifest is nil")
	}
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("benchmark manifest schema_version must be %d", SchemaVersion)
	}
	if !validIdentifier(manifest.ID, 100) || !validText(manifest.Revision, 100) {
		return errors.New("benchmark manifest id or revision is invalid")
	}
	if !unitInterval(manifest.AutoThreshold) || manifest.AutoThreshold == 0 {
		return errors.New("auto_threshold must be greater than 0 and at most 1")
	}
	if !unitInterval(manifest.ReviewMargin) {
		return errors.New("review_margin must be between 0 and 1")
	}
	if !unitInterval(manifest.HighConfidenceThreshold) || manifest.HighConfidenceThreshold < manifest.AutoThreshold {
		return errors.New("high_confidence_threshold must be between auto_threshold and 1")
	}
	if len(manifest.Samples) == 0 || len(manifest.Samples) > 10000 {
		return errors.New("benchmark manifest must contain 1 to 10000 samples")
	}
	seen := make(map[string]struct{}, len(manifest.Samples))
	groupSplits := make(map[string]string)
	support := map[string]map[string]int{
		SplitCalibration: {ExpectedNone: 0},
		SplitEvaluation:  {ExpectedNone: 0},
	}
	for _, split := range []string{SplitCalibration, SplitEvaluation} {
		for _, preset := range presetIDs {
			support[split][preset] = 0
		}
	}
	for _, sample := range manifest.Samples {
		if !validIdentifier(sample.ID, 200) || !validText(sample.AudioRef, 1024) {
			return fmt.Errorf("benchmark sample %q has an invalid id or audio_ref", sample.ID)
		}
		if _, duplicate := seen[sample.ID]; duplicate {
			return fmt.Errorf("benchmark sample %q appears more than once", sample.ID)
		}
		seen[sample.ID] = struct{}{}
		if !isExpectedPreset(sample.ExpectedPreset) {
			return fmt.Errorf("benchmark sample %q has invalid expected_preset %q", sample.ID, sample.ExpectedPreset)
		}
		if sample.Split != SplitCalibration && sample.Split != SplitEvaluation {
			return fmt.Errorf("benchmark sample %q split must be %q or %q", sample.ID, SplitCalibration, SplitEvaluation)
		}
		support[sample.Split][sample.ExpectedPreset]++
		if len(sample.Groups) > 16 {
			return fmt.Errorf("benchmark sample %q has too many groups", sample.ID)
		}
		sampleGroups := make(map[string]struct{}, len(sample.Groups))
		for _, group := range sample.Groups {
			if !validIdentifier(group, 100) {
				return fmt.Errorf("benchmark sample %q has invalid group %q", sample.ID, group)
			}
			if _, duplicate := sampleGroups[group]; duplicate {
				return fmt.Errorf("benchmark sample %q repeats group %q", sample.ID, group)
			}
			sampleGroups[group] = struct{}{}
			if existingSplit, exists := groupSplits[group]; exists && existingSplit != sample.Split {
				return fmt.Errorf("benchmark group %q crosses the calibration/evaluation boundary", group)
			}
			groupSplits[group] = sample.Split
		}
	}
	for _, split := range []string{SplitCalibration, SplitEvaluation} {
		for _, preset := range append(presetIDSlice(), ExpectedNone) {
			if support[split][preset] == 0 {
				return fmt.Errorf("benchmark manifest needs at least one %q sample in the %q split", preset, split)
			}
		}
	}
	return nil
}

func validateCandidateIdentity(candidate CandidateIdentity) error {
	if !validIdentifier(candidate.ID, 100) || !validText(candidate.ImplementationVersion, 100) ||
		!validText(candidate.ModelVersion, 100) {
		return errors.New("candidate id or version is invalid")
	}
	if !validText(candidate.CodeLicense, 200) || !validText(candidate.ModelLicense, 200) {
		return errors.New("candidate code_license and model_license are required")
	}
	if candidate.ModelDigest != "none" &&
		(!strings.HasPrefix(candidate.ModelDigest, "sha256:") || !validSHA256(strings.TrimPrefix(candidate.ModelDigest, "sha256:"))) {
		return errors.New("candidate model_digest must be none or a SHA-256 digest")
	}
	if candidate.ImageSizeBytes < 0 || candidate.BaseImageSizeBytes < 0 || candidate.ImageSizeBytes < candidate.BaseImageSizeBytes {
		return errors.New("candidate image sizes are invalid")
	}
	switch candidate.RuntimeKind {
	case "in_process":
		if candidate.ImageReference != "" || candidate.ImageDigest != "" || candidate.ImageSizeBytes != 0 || candidate.BaseImageSizeBytes != 0 {
			return errors.New("in_process candidate cannot declare a container image")
		}
	case "container":
		if !validText(candidate.ImageReference, 500) || !strings.HasPrefix(candidate.ImageDigest, "sha256:") ||
			!validSHA256(strings.TrimPrefix(candidate.ImageDigest, "sha256:")) || candidate.ImageSizeBytes == 0 ||
			candidate.ModelDigest == "none" {
			return errors.New("container candidate requires an image reference, digest, and positive image size")
		}
		if !strings.HasSuffix(candidate.ImageReference, "@"+candidate.ImageDigest) {
			return errors.New("container image_reference must be pinned to image_digest")
		}
	default:
		return errors.New("candidate runtime_kind must be in_process or container")
	}
	return nil
}

func validateCandidateSample(sample CandidateSample) error {
	if !validIdentifier(sample.ID, 200) {
		return errors.New("sample id is invalid")
	}
	if math.IsNaN(sample.CPUTimeMS) || math.IsInf(sample.CPUTimeMS, 0) || sample.CPUTimeMS < 0 || sample.PeakMemoryBytes < 0 {
		return errors.New("sample resource measurements are invalid")
	}
	if sample.ErrorCode != "" {
		if !validIdentifier(sample.ErrorCode, 100) || len(sample.Scores) != 0 {
			return errors.New("failed sample needs a bounded error_code and no scores")
		}
		return nil
	}
	if len(sample.Scores) != len(presetIDs) {
		return fmt.Errorf("successful sample must contain exactly %d preset scores", len(presetIDs))
	}
	for key, value := range sample.Scores {
		if !isPresetID(key) || !unitInterval(value) {
			return fmt.Errorf("score %q must be a known preset with a finite value between 0 and 1", key)
		}
	}
	return nil
}

func predict(sample CandidateSample, threshold, margin float64) (string, float64) {
	if sample.ErrorCode != "" {
		return PredictionAbstained, 0
	}
	topPreset, topScore, secondScore := presetIDs[0], sample.Scores[presetIDs[0]], -1.0
	for _, preset := range presetIDs[1:] {
		score := sample.Scores[preset]
		if score > topScore {
			secondScore = topScore
			topPreset, topScore = preset, score
		} else if score > secondScore {
			secondScore = score
		}
	}
	if topScore < threshold || topScore-secondScore < margin {
		return PredictionAbstained, topScore
	}
	return topPreset, topScore
}

func newConfusionMatrix() map[string]map[string]int {
	rows := append(presetIDSlice(), ExpectedNone)
	columns := append(presetIDSlice(), PredictionAbstained)
	matrix := make(map[string]map[string]int, len(rows))
	for _, row := range rows {
		matrix[row] = make(map[string]int, len(columns))
		for _, column := range columns {
			matrix[row][column] = 0
		}
	}
	return matrix
}

func presetIDSlice() []string {
	result := make([]string, len(presetIDs))
	copy(result, presetIDs[:])
	return result
}

func isPresetID(value string) bool {
	for _, preset := range presetIDs {
		if value == preset {
			return true
		}
	}
	return false
}

func isExpectedPreset(value string) bool { return value == ExpectedNone || isPresetID(value) }

func decodeBounded[T any](reader io.Reader, maximum int64) (*T, []byte, error) {
	if reader == nil {
		return nil, nil, errors.New("reader is nil")
	}
	raw, err := io.ReadAll(io.LimitReader(reader, maximum+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(raw)) > maximum {
		return nil, nil, fmt.Errorf("JSON exceeds %d bytes", maximum)
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var target T
	if err := decoder.Decode(&target); err != nil {
		return nil, nil, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, nil, errors.New("JSON contains more than one value")
		}
		return nil, nil, err
	}
	return &target, raw, nil
}

func validIdentifier(value string, maximum int) bool {
	if value == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if (character >= 'a' && character <= 'z') || (character >= 'A' && character <= 'Z') ||
			(character >= '0' && character <= '9') || character == '-' || character == '_' || character == '.' {
			continue
		}
		return false
	}
	return true
}

func validText(value string, maximum int) bool {
	if strings.TrimSpace(value) == "" || len(value) > maximum {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func validSHA256(value string) bool {
	if len(value) != sha256.Size*2 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func unitInterval(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func ratio(numerator, denominator int) float64 {
	if denominator == 0 {
		return 0
	}
	return float64(numerator) / float64(denominator)
}

func mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func percentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}
	ordered := append([]float64(nil), values...)
	sort.Float64s(ordered)
	index := int(math.Ceil(percentile*float64(len(ordered)))) - 1
	if index < 0 {
		index = 0
	}
	return ordered[index]
}
