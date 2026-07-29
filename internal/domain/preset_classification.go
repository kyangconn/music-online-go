package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	PresetCalmFlow     = "calm_flow"
	PresetKineticPulse = "kinetic_pulse"
	PresetCosmicDrift  = "cosmic_drift"
	PresetBassImpact   = "bass_impact"

	PresetStatusClassified   = "classified"
	PresetStatusNeedsReview  = "needs_review"
	PresetStatusUnclassified = "unclassified"

	PresetRuleVersion       = "metadata-v1"
	PresetHybridRuleVersion = "hybrid-v1"
)

var presetIDs = [...]string{
	PresetCalmFlow,
	PresetKineticPulse,
	PresetCosmicDrift,
	PresetBassImpact,
}

const MaxPresetBatchSize = 100

func PresetIDs() []string {
	values := make([]string, len(presetIDs))
	copy(values, presetIDs[:])
	return values
}

// PresetRulePolicy is deliberately model-independent. The values control only
// the inexpensive metadata rule layer, so an optional audio analyzer can be
// changed or disabled without changing the stable preset identities.
type PresetRulePolicy struct {
	Enabled            bool
	AutoThreshold      float64
	ReviewMargin       float64
	CalmFlowWeight     float64
	KineticPulseWeight float64
	CosmicDriftWeight  float64
	BassImpactWeight   float64
}

func DefaultPresetRulePolicy() PresetRulePolicy {
	return PresetRulePolicy{
		Enabled: true, AutoThreshold: 0.65, ReviewMargin: 0.12,
		CalmFlowWeight: 1, KineticPulseWeight: 1, CosmicDriftWeight: 1, BassImpactWeight: 1,
	}
}

func IsPresetID(value string) bool {
	for _, preset := range presetIDs {
		if value == preset {
			return true
		}
	}
	return false
}

func IsPresetStatus(value string) bool {
	switch value {
	case PresetStatusClassified, PresetStatusNeedsReview, PresetStatusUnclassified:
		return true
	default:
		return false
	}
}

// MusicPresetClassification stores automatic output and the administrator's
// override independently. Reclassification updates only automatic fields, so
// a rule or analyzer upgrade can never erase a deliberate manual choice.
type MusicPresetClassification struct {
	MusicID uint `json:"music_id" gorm:"primaryKey;autoIncrement:false"`

	PrimaryPreset    string     `json:"primary_preset" gorm:"size:32;index"`
	AutomaticPreset  string     `json:"automatic_preset" gorm:"size:32;index"`
	Confidence       float64    `json:"confidence" gorm:"not null;default:0"`
	Status           string     `json:"status" gorm:"size:24;not null;index"`
	RuleVersion      string     `json:"rule_version" gorm:"size:64;not null;index"`
	MetadataRevision uint64     `json:"metadata_revision" gorm:"not null;default:0"`
	AudioAnalysisID  *uint      `json:"audio_analysis_id,omitempty" gorm:"index:idx_preset_audio_analysis"`
	EvidenceSummary  StringList `json:"evidence_summary" gorm:"type:text;not null"`
	EvaluatedAt      time.Time  `json:"evaluated_at"`

	ManualPreset    *string    `json:"manual_preset,omitempty" gorm:"size:32;index"`
	ManualUpdatedBy *uint      `json:"manual_updated_by,omitempty" gorm:"index"`
	ManualUpdatedAt *time.Time `json:"manual_updated_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`

	Music         Music               `json:"-" gorm:"foreignKey:MusicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	AudioAnalysis *MusicAudioAnalysis `json:"-" gorm:"foreignKey:AudioAnalysisID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Scores        []MusicPresetScore  `json:"scores,omitempty" gorm:"foreignKey:MusicID;references:MusicID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
}

func (*MusicPresetClassification) TableName() string {
	return "music_preset_classifications"
}

type MusicPresetScore struct {
	MusicID  uint               `json:"-" gorm:"primaryKey;autoIncrement:false"`
	PresetID string             `json:"preset_id" gorm:"primaryKey;size:32"`
	Score    float64            `json:"score" gorm:"not null;default:0;index"`
	Evidence PresetEvidenceList `json:"evidence" gorm:"type:text;not null"`
}

func (*MusicPresetScore) TableName() string {
	return "music_preset_scores"
}

type PresetEvidence struct {
	Source string  `json:"source"`
	Key    string  `json:"key"`
	Weight float64 `json:"weight"`
}

type PresetEvidenceList []PresetEvidence

func (values PresetEvidenceList) Value() (driver.Value, error) {
	if values == nil {
		values = PresetEvidenceList{}
	}
	encoded, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("encode preset evidence: %w", err)
	}
	return string(encoded), nil
}

func (values *PresetEvidenceList) Scan(value any) error {
	if value == nil {
		*values = PresetEvidenceList{}
		return nil
	}
	var encoded []byte
	switch typed := value.(type) {
	case []byte:
		encoded = typed
	case string:
		encoded = []byte(typed)
	default:
		return fmt.Errorf("scan preset evidence from %T", value)
	}
	if len(encoded) == 0 {
		*values = PresetEvidenceList{}
		return nil
	}
	if err := json.Unmarshal(encoded, values); err != nil {
		return fmt.Errorf("decode preset evidence: %w", err)
	}
	if *values == nil {
		*values = PresetEvidenceList{}
	}
	return nil
}

func (PresetEvidenceList) GormDataType() string {
	return "text"
}

func (values PresetEvidenceList) MarshalJSON() ([]byte, error) {
	if values == nil {
		return []byte("[]"), nil
	}
	type plainEvidence PresetEvidenceList
	return json.Marshal(plainEvidence(values))
}

type PresetClassificationResponse struct {
	PrimaryPreset    string             `json:"primary_preset"`
	AutomaticPreset  string             `json:"automatic_preset"`
	ManualPreset     *string            `json:"manual_preset,omitempty"`
	EffectivePreset  string             `json:"effective_preset"`
	EffectiveSource  string             `json:"effective_source"`
	Confidence       float64            `json:"confidence"`
	Status           string             `json:"status"`
	RuleVersion      string             `json:"rule_version"`
	MetadataRevision uint64             `json:"metadata_revision"`
	AudioAnalysisID  *uint              `json:"audio_analysis_id,omitempty"`
	EvidenceSummary  StringList         `json:"evidence_summary"`
	Scores           []MusicPresetScore `json:"scores"`
	EvaluatedAt      time.Time          `json:"evaluated_at"`
	ManualUpdatedAt  *time.Time         `json:"manual_updated_at,omitempty"`
}

type PresetSummary struct {
	PresetID         string `json:"preset_id"`
	TrackCount       int64  `json:"track_count"`
	NeedsReviewCount int64  `json:"needs_review_count"`
}

type BatchPresetOverrideRequest struct {
	MusicIDs []uint  `json:"music_ids" binding:"required"`
	Preset   *string `json:"preset"`
}

type BatchPresetOverrideResponse struct {
	Updated         int                             `json:"updated"`
	Classifications []*PresetClassificationResponse `json:"classifications"`
}

func (classification *MusicPresetClassification) ToResponse() *PresetClassificationResponse {
	if classification == nil {
		return nil
	}
	effectivePreset := classification.AutomaticPreset
	effectiveSource := "automatic"
	if classification.ManualPreset != nil {
		effectivePreset = *classification.ManualPreset
		effectiveSource = "manual"
	} else if effectivePreset == "" {
		effectiveSource = "none"
	}
	scores := append([]MusicPresetScore(nil), classification.Scores...)
	sort.Slice(scores, func(left, right int) bool {
		return presetOrder(scores[left].PresetID) < presetOrder(scores[right].PresetID)
	})
	return &PresetClassificationResponse{
		PrimaryPreset: classification.PrimaryPreset, AutomaticPreset: classification.AutomaticPreset,
		ManualPreset: classification.ManualPreset, EffectivePreset: effectivePreset, EffectiveSource: effectiveSource,
		Confidence: classification.Confidence, Status: classification.Status, RuleVersion: classification.RuleVersion,
		MetadataRevision: classification.MetadataRevision, AudioAnalysisID: classification.AudioAnalysisID,
		EvidenceSummary: classification.EvidenceSummary,
		Scores:          scores, EvaluatedAt: classification.EvaluatedAt, ManualUpdatedAt: classification.ManualUpdatedAt,
	}
}

type GenreToken struct {
	Display    string
	Normalized string
	Canonical  string
}

// PresetAudioEvidence is the small, model-independent subset of an analyzer
// artifact understood by the hybrid classifier. Unknown feature keys remain
// stored in the artifact but cannot silently become classification rules.
type PresetAudioEvidence struct {
	AnalysisID      uint
	AnalyzerID      string
	AnalyzerVersion string
	ModelVersion    string
	Features        map[string]float64
	ModelLabels     map[string]float64
	BPMCandidates   []AudioBPMCandidate
}

type AudioBPMCandidate struct {
	BPM        float64 `json:"bpm"`
	Confidence float64 `json:"confidence"`
}

// DecodePresetAudioEvidence validates and extracts only the stable top-level
// feature contract used by hybrid-v1. Nested vendor-specific data is ignored.
func DecodePresetAudioEvidence(analysis *MusicAudioAnalysis) (*PresetAudioEvidence, error) {
	if analysis == nil || analysis.ID == 0 || analysis.Status != AnalysisStatusSucceeded {
		return nil, nil
	}
	featureValues := make(map[string]json.RawMessage)
	if len(analysis.Features) > 0 {
		if err := json.Unmarshal(analysis.Features, &featureValues); err != nil {
			return nil, fmt.Errorf("decode preset audio features: %w", err)
		}
	}
	if featureValues == nil {
		featureValues = make(map[string]json.RawMessage)
	}
	features := make(map[string]float64, len(featureValues))
	var encodedCandidates json.RawMessage
	for key, encoded := range featureValues {
		normalizedKey := normalizeAudioFeatureKey(key)
		if normalizedKey == "bpm_candidates" {
			encodedCandidates = encoded
			continue
		}
		var value float64
		if err := json.Unmarshal(encoded, &value); err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
			continue
		}
		features[normalizedKey] = value
	}
	candidates := decodeBPMCandidates(encodedCandidates, features["bpm_confidence"])
	labels := make(map[string]float64)
	if len(analysis.ModelLabels) > 0 {
		if err := json.Unmarshal(analysis.ModelLabels, &labels); err != nil {
			return nil, fmt.Errorf("decode preset model labels: %w", err)
		}
	}
	if labels == nil {
		labels = make(map[string]float64)
	}
	for label, value := range labels {
		if !unitScore(value) {
			delete(labels, label)
		}
	}
	return &PresetAudioEvidence{
		AnalysisID: analysis.ID, AnalyzerID: analysis.AnalyzerID, AnalyzerVersion: analysis.AnalyzerVersion,
		ModelVersion: analysis.ModelVersion, Features: features, ModelLabels: labels, BPMCandidates: candidates,
	}, nil
}

// decodeBPMCandidates is deliberately tolerant because analyzer artifacts are
// durable data. It accepts the preferred {bpm, confidence} objects and legacy
// numeric candidates, while ignoring malformed optional entries rather than
// making an otherwise successful analysis unusable.
func decodeBPMCandidates(encoded json.RawMessage, fallbackConfidence float64) []AudioBPMCandidate {
	if len(encoded) == 0 {
		return nil
	}
	var values []json.RawMessage
	if err := json.Unmarshal(encoded, &values); err != nil {
		return nil
	}
	result := make([]AudioBPMCandidate, 0, min(len(values), 8))
	for _, value := range values {
		var candidate AudioBPMCandidate
		if err := json.Unmarshal(value, &candidate); err != nil || candidate.BPM == 0 {
			var bpm float64
			if err := json.Unmarshal(value, &bpm); err != nil {
				continue
			}
			candidate = AudioBPMCandidate{BPM: bpm, Confidence: fallbackConfidence}
		} else if candidate.Confidence == 0 && unitScore(fallbackConfidence) {
			candidate.Confidence = fallbackConfidence
		}
		if candidate.BPM < 20 || candidate.BPM > 400 || !unitScore(candidate.Confidence) || candidate.Confidence <= 0 {
			continue
		}
		result = append(result, candidate)
		if len(result) == 8 {
			break
		}
	}
	return result
}

var genreAliases = map[string]string{
	"chill step":        "chillstep",
	"melodic dub step":  "melodic dubstep",
	"drum n bass":       "drum and bass",
	"d and b":           "drum and bass",
	"dnb":               "drum and bass",
	"electro house":     "electro house",
	"progressive house": "progressive house",
	"synth wave":        "synthwave",
	"retro wave":        "retrowave",
	"glitch hop":        "glitch hop",
	"hard style":        "hardstyle",
	"raw style":         "rawstyle",
}

// TokenizeGenres is the only genre tokenizer used by metadata storage,
// filtering and classification. It bounds hostile tag expansion, strips
// controls and treats spacing, case, underscores and common hyphens alike.
func TokenizeGenres(values StringList) []GenreToken {
	result := make([]GenreToken, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		for _, part := range genreFacetSeparator.Split(value, -1) {
			display := sanitizeGenreDisplay(part)
			normalized := NormalizeGenreName(display)
			if normalized == "" || len(normalized) > maxGenreFacetBytes {
				continue
			}
			if _, exists := seen[normalized]; exists {
				continue
			}
			seen[normalized] = struct{}{}
			canonical := normalized
			if alias, exists := genreAliases[normalized]; exists {
				canonical = alias
			}
			result = append(result, GenreToken{Display: display, Normalized: normalized, Canonical: canonical})
			if len(result) >= maxMusicGenreFacets {
				return result
			}
		}
	}
	return result
}

func NormalizeGenreName(value string) string {
	value = sanitizeGenreDisplay(value)
	value = strings.NewReplacer(
		"-", " ", "_", " ", "‐", " ", "‑", " ", "‒", " ", "–", " ", "—", " ", "&", " and ",
	).Replace(value)
	return NormalizeBrowseText(value)
}

func CanonicalGenreName(value string) string {
	normalized := NormalizeGenreName(value)
	if alias, exists := genreAliases[normalized]; exists {
		return alias
	}
	return normalized
}

func sanitizeGenreDisplay(value string) string {
	value = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, value)
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

type presetGenreRule struct {
	preset string
	weight float64
}

var presetGenreRules = map[string]presetGenreRule{
	"chillstep":         {preset: PresetCalmFlow, weight: 0.95},
	"ambient":           {preset: PresetCalmFlow, weight: 0.78},
	"downtempo":         {preset: PresetCalmFlow, weight: 0.78},
	"chillout":          {preset: PresetCalmFlow, weight: 0.74},
	"melodic dubstep":   {preset: PresetKineticPulse, weight: 0.95},
	"drum and bass":     {preset: PresetKineticPulse, weight: 0.92},
	"complextro":        {preset: PresetKineticPulse, weight: 0.92},
	"electro house":     {preset: PresetKineticPulse, weight: 0.86},
	"breakbeat":         {preset: PresetKineticPulse, weight: 0.78},
	"glitch hop":        {preset: PresetKineticPulse, weight: 0.82},
	"trance":            {preset: PresetCosmicDrift, weight: 0.92},
	"progressive house": {preset: PresetCosmicDrift, weight: 0.84},
	"synthwave":         {preset: PresetCosmicDrift, weight: 0.82},
	"retrowave":         {preset: PresetCosmicDrift, weight: 0.82},
	"dubstep":           {preset: PresetBassImpact, weight: 0.78},
	"brostep":           {preset: PresetBassImpact, weight: 0.96},
	"riddim":            {preset: PresetBassImpact, weight: 0.95},
	"tearout":           {preset: PresetBassImpact, weight: 0.96},
	"hardstyle":         {preset: PresetBassImpact, weight: 0.91},
	"rawstyle":          {preset: PresetBassImpact, weight: 0.95},
	"gabber":            {preset: PresetBassImpact, weight: 0.92},
}

// BuildMusicPresetProjection produces four independent scores and may abstain.
// Instrumental is intentionally not a scoring rule: without audio evidence it
// cannot distinguish calm instrumentals from aggressive ones.
func BuildMusicPresetProjection(music *Music, policy PresetRulePolicy) (*MusicPresetClassification, []MusicPresetScore) {
	if music == nil || music.ID == 0 || !policy.Enabled {
		return nil, nil
	}
	genreValues := music.Genres
	if len(genreValues) == 0 && strings.TrimSpace(music.Genre) != "" {
		genreValues = StringList{music.Genre}
	}
	tokens := TokenizeGenres(genreValues)

	scoreValues := make(map[string]float64, len(presetIDs))
	evidence := make(map[string]PresetEvidenceList, len(presetIDs))
	summary := StringList{}
	seenCanonical := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		if _, exists := seenCanonical[token.Canonical]; exists {
			continue
		}
		seenCanonical[token.Canonical] = struct{}{}
		rule, exists := presetGenreRules[token.Canonical]
		if !exists {
			if token.Canonical == "instrumental" {
				summary = append(summary, "instrumental_requires_audio_evidence")
			}
			continue
		}
		weight := clampScore(rule.weight * presetPolicyWeight(policy, rule.preset))
		previous := scoreValues[rule.preset]
		scoreValues[rule.preset] = clampScore(1 - (1-previous)*(1-weight))
		evidence[rule.preset] = append(evidence[rule.preset], PresetEvidence{
			Source: "genre", Key: token.Canonical, Weight: roundScore(weight),
		})
		summary = appendUniqueString(summary, "genre:"+token.Canonical)
	}

	scores := make([]MusicPresetScore, 0, len(presetIDs))
	for _, preset := range presetIDs {
		scores = append(scores, MusicPresetScore{
			MusicID: music.ID, PresetID: preset, Score: roundScore(scoreValues[preset]), Evidence: evidence[preset],
		})
	}
	sorted := append([]MusicPresetScore(nil), scores...)
	sort.SliceStable(sorted, func(left, right int) bool { return sorted[left].Score > sorted[right].Score })
	top := sorted[0]
	second := sorted[1]

	status := PresetStatusUnclassified
	primaryPreset := ""
	automaticPreset := ""
	confidence := top.Score
	if top.Score > 0 {
		primaryPreset = top.PresetID
		status = PresetStatusNeedsReview
		if top.Score >= policy.AutoThreshold && top.Score-second.Score >= policy.ReviewMargin {
			status = PresetStatusClassified
			automaticPreset = top.PresetID
		}
	}

	classification := &MusicPresetClassification{
		MusicID: music.ID, PrimaryPreset: primaryPreset, AutomaticPreset: automaticPreset,
		Confidence: roundScore(confidence), Status: status, RuleVersion: PresetRuleVersion,
		MetadataRevision: music.MetadataRevision, EvidenceSummary: summary, EvaluatedAt: time.Now().UTC(),
	}
	return classification, scores
}

const (
	modelEvidenceScale       = 0.72
	dspEvidenceScale         = 0.55
	metadataAudioEvidenceMax = 0.80
	maxRecordedModelEvidence = 12
)

type scoredAudioEvidence struct {
	preset   string
	key      string
	strength float64
}

// BuildMusicPresetProjectionWithAudio adds bounded model and DSP evidence to
// the metadata baseline. Source scales encode the priority genre > model > DSP;
// corroborating sources may strengthen a score, while conflicts naturally
// reduce the top-two margin and therefore route the track to review.
func BuildMusicPresetProjectionWithAudio(
	music *Music,
	audio *PresetAudioEvidence,
	policy PresetRulePolicy,
) (*MusicPresetClassification, []MusicPresetScore) {
	classification, scores := BuildMusicPresetProjection(music, policy)
	if classification == nil || audio == nil || audio.AnalysisID == 0 {
		return classification, scores
	}
	scoreByPreset := make(map[string]*MusicPresetScore, len(scores))
	for index := range scores {
		scoreByPreset[scores[index].PresetID] = &scores[index]
	}

	modelRaw := make(map[string]float64, len(presetIDs))
	modelEvidence := make([]scoredAudioEvidence, 0, len(audio.ModelLabels))
	labels := make([]string, 0, len(audio.ModelLabels))
	for label := range audio.ModelLabels {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	for _, label := range labels {
		probability := audio.ModelLabels[label]
		if probability <= 0 || !unitScore(probability) {
			continue
		}
		canonical := CanonicalGenreName(label)
		rule, exists := presetGenreRules[canonical]
		if !exists {
			continue
		}
		strength := clampScore(probability * rule.weight)
		modelRaw[rule.preset] = mergeEvidenceScore(modelRaw[rule.preset], strength)
		modelEvidence = append(modelEvidence, scoredAudioEvidence{preset: rule.preset, key: canonical, strength: strength})
	}
	sort.SliceStable(modelEvidence, func(left, right int) bool {
		if modelEvidence[left].strength == modelEvidence[right].strength {
			if modelEvidence[left].preset == modelEvidence[right].preset {
				return modelEvidence[left].key < modelEvidence[right].key
			}
			return presetOrder(modelEvidence[left].preset) < presetOrder(modelEvidence[right].preset)
		}
		return modelEvidence[left].strength > modelEvidence[right].strength
	})
	if len(modelEvidence) > maxRecordedModelEvidence {
		modelEvidence = modelEvidence[:maxRecordedModelEvidence]
	}
	for _, preset := range presetIDs {
		contribution := clampScore(modelRaw[preset] * modelEvidenceScale * presetPolicyWeight(policy, preset))
		if contribution == 0 {
			continue
		}
		mergePresetScore(scoreByPreset[preset], contribution)
		classification.EvidenceSummary = appendUniqueString(classification.EvidenceSummary, "audio_model_evidence")
	}
	for _, item := range modelEvidence {
		score := scoreByPreset[item.preset]
		if score == nil {
			continue
		}
		weight := clampScore(item.strength * modelEvidenceScale * presetPolicyWeight(policy, item.preset))
		score.Evidence = append(score.Evidence, PresetEvidence{Source: "model", Key: item.key, Weight: roundScore(weight)})
	}

	dspRaw := make(map[string]float64, len(presetIDs))
	dspEvidence := make(map[string]PresetEvidenceList, len(presetIDs))
	addDSP := func(preset, key string, value, coefficient float64) {
		if !unitScore(value) || value <= 0 {
			return
		}
		strength := clampScore(value * coefficient)
		dspRaw[preset] = mergeEvidenceScore(dspRaw[preset], strength)
		dspEvidence[preset] = append(dspEvidence[preset], PresetEvidence{Source: "dsp", Key: key, Weight: strength})
	}
	if value, ok := unitAudioFeature(audio, "energy"); ok {
		addDSP(PresetCalmFlow, "low_energy", 1-value, 0.40)
		addDSP(PresetKineticPulse, "energy", value, 0.20)
		addDSP(PresetBassImpact, "energy", value, 0.15)
	}
	if value, ok := unitAudioFeature(audio, "arousal"); ok {
		addDSP(PresetCalmFlow, "low_arousal", 1-value, 0.35)
	}
	if value, ok := unitAudioFeature(audio, "dynamic_smoothness"); ok {
		addDSP(PresetCalmFlow, "dynamic_smoothness", value, 0.25)
		addDSP(PresetCosmicDrift, "dynamic_smoothness", value, 0.15)
	}
	if value, ok := unitAudioFeature(audio, "dynamic_range_normalized"); ok {
		addDSP(PresetCalmFlow, "dynamic_range", value, 0.10)
	}
	if value, ok := unitAudioFeature(audio, "danceability"); ok {
		addDSP(PresetKineticPulse, "danceability", value, 0.45)
		addDSP(PresetCosmicDrift, "danceability", value, 0.10)
	}
	if value, ok := unitAudioFeature(audio, "onset_rate_normalized"); ok {
		addDSP(PresetKineticPulse, "onset_rate", value, 0.25)
	}
	if value, ok := unitAudioFeature(audio, "pulse_clarity"); ok {
		addDSP(PresetKineticPulse, "pulse_clarity", value, 0.35)
		addDSP(PresetCosmicDrift, "pulse_clarity", value, 0.30)
	}
	if value, ok := unitAudioFeature(audio, "spectral_flux"); ok {
		addDSP(PresetCalmFlow, "low_spectral_flux", 1-value, 0.15)
		addDSP(PresetKineticPulse, "spectral_flux", value, 0.15)
	}
	if value, ok := unitAudioFeature(audio, "spectral_centroid_normalized"); ok {
		addDSP(PresetKineticPulse, "spectral_centroid", value, 0.08)
		addDSP(PresetBassImpact, "spectral_centroid", value, 0.06)
	}
	if value, ok := unitAudioFeature(audio, "spectral_flatness"); ok {
		addDSP(PresetBassImpact, "spectral_flatness", value, 0.08)
	}
	if value, ok := unitAudioFeature(audio, "tonal_strength"); ok {
		addDSP(PresetCosmicDrift, "tonal_strength", value, 0.25)
	}
	if value, ok := unitAudioFeature(audio, "harmonicity"); ok {
		addDSP(PresetCosmicDrift, "harmonicity", value, 0.20)
	}
	if value, ok := unitAudioFeature(audio, "spatiality"); ok {
		addDSP(PresetCosmicDrift, "spatiality", value, 0.20)
	}
	if value, ok := unitAudioFeature(audio, "bass_energy_ratio"); ok {
		addDSP(PresetBassImpact, "bass_energy_ratio", value, 0.45)
	}
	if value, ok := unitAudioFeature(audio, "sub_bass_energy_ratio"); ok {
		addDSP(PresetBassImpact, "sub_bass_energy_ratio", value, 0.40)
	}
	if value, ok := unitAudioFeature(audio, "drop_contrast"); ok {
		addDSP(PresetBassImpact, "drop_contrast", value, 0.35)
	}
	if value, ok := unitAudioFeature(audio, "roughness"); ok {
		addDSP(PresetBassImpact, "roughness", value, 0.25)
	}
	if value, ok := unitAudioFeature(audio, "loudness_normalized"); ok {
		addDSP(PresetCalmFlow, "low_loudness", 1-value, 0.08)
		addDSP(PresetBassImpact, "loudness", value, 0.10)
	}
	if value, ok := unitAudioFeature(audio, "high_energy_segment_ratio"); ok {
		addDSP(PresetKineticPulse, "high_energy_segments", value, 0.08)
		addDSP(PresetBassImpact, "high_energy_segments", value, 0.10)
	}
	addTempoEvidence(audio, addDSP)
	for _, preset := range presetIDs {
		policyWeight := presetPolicyWeight(policy, preset)
		contribution := clampScore(dspRaw[preset] * dspEvidenceScale * policyWeight)
		if contribution == 0 {
			continue
		}
		mergePresetScore(scoreByPreset[preset], contribution)
		classification.EvidenceSummary = appendUniqueString(classification.EvidenceSummary, "audio_dsp_evidence")
		for _, item := range dspEvidence[preset] {
			item.Weight = roundScore(item.Weight * dspEvidenceScale * policyWeight)
			scoreByPreset[preset].Evidence = append(scoreByPreset[preset].Evidence, item)
		}
	}

	if instrumentalCalmScore := instrumentalCalmEvidence(music, audio); instrumentalCalmScore > 0 {
		contribution := clampScore(instrumentalCalmScore * metadataAudioEvidenceMax * policy.CalmFlowWeight)
		mergePresetScore(scoreByPreset[PresetCalmFlow], contribution)
		scoreByPreset[PresetCalmFlow].Evidence = append(scoreByPreset[PresetCalmFlow].Evidence, PresetEvidence{
			Source: "metadata_audio", Key: "instrumental_low_energy", Weight: roundScore(contribution),
		})
		classification.EvidenceSummary = appendUniqueString(classification.EvidenceSummary, "instrumental_audio_confirmed")
	}

	classification.RuleVersion = PresetHybridRuleVersion
	analysisID := audio.AnalysisID
	classification.AudioAnalysisID = &analysisID
	classification.EvaluatedAt = time.Now().UTC()
	applyPresetDecision(classification, scores, policy)
	return classification, scores
}

func mergePresetScore(score *MusicPresetScore, contribution float64) {
	if score == nil || contribution <= 0 {
		return
	}
	score.Score = roundScore(mergeEvidenceScore(score.Score, contribution))
}

func mergeEvidenceScore(previous, contribution float64) float64 {
	return clampScore(1 - (1-clampScore(previous))*(1-clampScore(contribution)))
}

func applyPresetDecision(classification *MusicPresetClassification, scores []MusicPresetScore, policy PresetRulePolicy) {
	sorted := append([]MusicPresetScore(nil), scores...)
	sort.SliceStable(sorted, func(left, right int) bool {
		if sorted[left].Score == sorted[right].Score {
			return presetOrder(sorted[left].PresetID) < presetOrder(sorted[right].PresetID)
		}
		return sorted[left].Score > sorted[right].Score
	})
	top, second := sorted[0], sorted[1]
	classification.PrimaryPreset = ""
	classification.AutomaticPreset = ""
	classification.Confidence = roundScore(top.Score)
	classification.Status = PresetStatusUnclassified
	if top.Score <= 0 {
		return
	}
	classification.PrimaryPreset = top.PresetID
	classification.Status = PresetStatusNeedsReview
	if top.Score >= policy.AutoThreshold && top.Score-second.Score >= policy.ReviewMargin {
		classification.AutomaticPreset = top.PresetID
		classification.Status = PresetStatusClassified
	}
}

func unitAudioFeature(audio *PresetAudioEvidence, key string) (float64, bool) {
	if audio == nil {
		return 0, false
	}
	value, exists := audio.Features[key]
	return value, exists && unitScore(value)
}

func addTempoEvidence(audio *PresetAudioEvidence, add func(string, string, float64, float64)) {
	candidates := append([]AudioBPMCandidate(nil), audio.BPMCandidates...)
	if len(candidates) == 0 {
		bpm, bpmOK := audio.Features["bpm"]
		confidence, confidenceOK := unitAudioFeature(audio, "bpm_confidence")
		if bpmOK && confidenceOK && bpm >= 20 && bpm <= 400 {
			candidates = append(candidates, AudioBPMCandidate{BPM: bpm, Confidence: confidence})
		}
	}
	if len(candidates) == 0 {
		return
	}
	sort.SliceStable(candidates, func(left, right int) bool { return candidates[left].Confidence > candidates[right].Confidence })
	best := candidates[0]
	folded := best.BPM
	for folded < 70 {
		folded *= 2
	}
	for folded > 190 {
		folded /= 2
	}
	add(PresetKineticPulse, "bpm_affinity", best.Confidence*tempoAffinity(folded, 160, 90), 0.12)
	add(PresetCosmicDrift, "bpm_affinity", best.Confidence*tempoAffinity(folded, 130, 75), 0.12)
}

func tempoAffinity(bpm, center, width float64) float64 {
	return clampScore(1 - math.Abs(bpm-center)/width)
}

func instrumentalCalmEvidence(music *Music, audio *PresetAudioEvidence) float64 {
	if !musicHasCanonicalGenre(music, "instrumental") {
		return 0
	}
	instrumental, instrumentalOK := unitAudioFeature(audio, "instrumental_probability")
	if !instrumentalOK {
		if vocal, vocalOK := unitAudioFeature(audio, "vocal_probability"); vocalOK {
			instrumental, instrumentalOK = 1-vocal, true
		}
	}
	energy, energyOK := unitAudioFeature(audio, "energy")
	if !instrumentalOK || !energyOK || instrumental < 0.5 || energy > 0.5 {
		return 0
	}
	calmSupport := 1 - energy
	if arousal, ok := unitAudioFeature(audio, "arousal"); ok {
		calmSupport = math.Min(calmSupport, 1-arousal)
	} else if smoothness, ok := unitAudioFeature(audio, "dynamic_smoothness"); ok {
		calmSupport = math.Min(calmSupport, smoothness)
	} else {
		return 0
	}
	return clampScore(math.Min(instrumental, calmSupport))
}

func musicHasCanonicalGenre(music *Music, wanted string) bool {
	if music == nil {
		return false
	}
	values := music.Genres
	if len(values) == 0 && strings.TrimSpace(music.Genre) != "" {
		values = StringList{music.Genre}
	}
	for _, token := range TokenizeGenres(values) {
		if token.Canonical == wanted {
			return true
		}
	}
	return false
}

func normalizeAudioFeatureKey(value string) string {
	return strings.NewReplacer("-", "_", " ", "_", ".", "_").Replace(strings.ToLower(strings.TrimSpace(value)))
}

func unitScore(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= 0 && value <= 1
}

func presetPolicyWeight(policy PresetRulePolicy, preset string) float64 {
	switch preset {
	case PresetCalmFlow:
		return policy.CalmFlowWeight
	case PresetKineticPulse:
		return policy.KineticPulseWeight
	case PresetCosmicDrift:
		return policy.CosmicDriftWeight
	case PresetBassImpact:
		return policy.BassImpactWeight
	default:
		return 1
	}
}

func presetOrder(value string) int {
	for index, preset := range presetIDs {
		if value == preset {
			return index
		}
	}
	return len(presetIDs)
}

func clampScore(value float64) float64 {
	return math.Max(0, math.Min(1, value))
}

func roundScore(value float64) float64 {
	return math.Round(clampScore(value)*10000) / 10000
}

func appendUniqueString(values StringList, value string) StringList {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}
