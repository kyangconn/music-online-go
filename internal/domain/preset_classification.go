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

	PresetRuleVersion = "metadata-v1"
)

var presetIDs = [...]string{
	PresetCalmFlow,
	PresetKineticPulse,
	PresetCosmicDrift,
	PresetBassImpact,
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

func (policy PresetRulePolicy) normalized() PresetRulePolicy {
	defaults := DefaultPresetRulePolicy()
	if policy.AutoThreshold <= 0 || policy.AutoThreshold > 1 {
		policy.AutoThreshold = defaults.AutoThreshold
	}
	if policy.ReviewMargin < 0 || policy.ReviewMargin > 1 {
		policy.ReviewMargin = defaults.ReviewMargin
	}
	if policy.CalmFlowWeight <= 0 {
		policy.CalmFlowWeight = defaults.CalmFlowWeight
	}
	if policy.KineticPulseWeight <= 0 {
		policy.KineticPulseWeight = defaults.KineticPulseWeight
	}
	if policy.CosmicDriftWeight <= 0 {
		policy.CosmicDriftWeight = defaults.CosmicDriftWeight
	}
	if policy.BassImpactWeight <= 0 {
		policy.BassImpactWeight = defaults.BassImpactWeight
	}
	return policy
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
	EvidenceSummary  StringList `json:"evidence_summary" gorm:"type:text;not null"`
	EvaluatedAt      time.Time  `json:"evaluated_at"`

	ManualPreset    *string    `json:"manual_preset,omitempty" gorm:"size:32;index"`
	ManualUpdatedBy *uint      `json:"manual_updated_by,omitempty" gorm:"index"`
	ManualUpdatedAt *time.Time `json:"manual_updated_at,omitempty"`
	UpdatedAt       time.Time  `json:"updated_at"`

	Music  Music              `json:"-" gorm:"foreignKey:MusicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Scores []MusicPresetScore `json:"scores,omitempty" gorm:"foreignKey:MusicID;references:MusicID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
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
	EvidenceSummary  StringList         `json:"evidence_summary"`
	Scores           []MusicPresetScore `json:"scores"`
	EvaluatedAt      time.Time          `json:"evaluated_at"`
	ManualUpdatedAt  *time.Time         `json:"manual_updated_at,omitempty"`
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
		MetadataRevision: classification.MetadataRevision, EvidenceSummary: classification.EvidenceSummary,
		Scores: scores, EvaluatedAt: classification.EvaluatedAt, ManualUpdatedAt: classification.ManualUpdatedAt,
	}
}

type GenreToken struct {
	Display    string
	Normalized string
	Canonical  string
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
	policy = policy.normalized()
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
