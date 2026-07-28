package domain

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

const (
	AnalysisJobKindMetadataRules = "metadata_rules"
	AnalysisJobKindAudio         = "audio_analysis"

	AnalysisStatusPending   = "pending"
	AnalysisStatusRunning   = "running"
	AnalysisStatusSucceeded = "succeeded"
	AnalysisStatusFailed    = "failed"
	AnalysisStatusStale     = "stale"
	AnalysisStatusCancelled = "cancelled"
)

var analysisJobStatuses = [...]string{
	AnalysisStatusPending,
	AnalysisStatusRunning,
	AnalysisStatusSucceeded,
	AnalysisStatusFailed,
	AnalysisStatusStale,
	AnalysisStatusCancelled,
}

func IsAnalysisJobStatus(value string) bool {
	for _, status := range analysisJobStatuses {
		if value == status {
			return true
		}
	}
	return false
}

func IsAnalysisJobKind(value string) bool {
	return value == AnalysisJobKindMetadataRules || value == AnalysisJobKindAudio
}

// JSONDocument stores validated JSON as text so SQLite and PostgreSQL use the
// same schema and API representation. Analyzer responses are bounded before
// they reach this type.
type JSONDocument []byte

func NewJSONDocument(value any) (JSONDocument, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("encode JSON document: %w", err)
	}
	return JSONDocument(encoded), nil
}

func (document JSONDocument) Value() (driver.Value, error) {
	if len(document) == 0 {
		return "{}", nil
	}
	if !json.Valid(document) {
		return nil, fmt.Errorf("invalid JSON document")
	}
	return string(document), nil
}

func (document *JSONDocument) Scan(value any) error {
	if value == nil {
		*document = JSONDocument("{}")
		return nil
	}
	var encoded []byte
	switch typed := value.(type) {
	case []byte:
		encoded = append([]byte(nil), typed...)
	case string:
		encoded = []byte(typed)
	default:
		return fmt.Errorf("scan JSON document from %T", value)
	}
	if len(encoded) == 0 {
		encoded = []byte("{}")
	}
	if !json.Valid(encoded) {
		return fmt.Errorf("scan invalid JSON document")
	}
	*document = JSONDocument(encoded)
	return nil
}

func (JSONDocument) GormDataType() string {
	return "text"
}

func (document JSONDocument) MarshalJSON() ([]byte, error) {
	if len(document) == 0 {
		return []byte("{}"), nil
	}
	if !json.Valid(document) {
		return nil, fmt.Errorf("marshal invalid JSON document")
	}
	return document, nil
}

// MusicAudioAnalysis is a reusable artifact keyed by content and analyzer
// versions. Jobs for byte-identical tracks can reference one successful row.
type MusicAudioAnalysis struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	FileHash        string       `json:"file_hash" gorm:"size:64;not null;uniqueIndex:idx_audio_analysis_cache,priority:1"`
	AnalyzerID      string       `json:"analyzer_id" gorm:"size:100;not null;uniqueIndex:idx_audio_analysis_cache,priority:2"`
	AnalyzerVersion string       `json:"analyzer_version" gorm:"size:100;not null;uniqueIndex:idx_audio_analysis_cache,priority:3"`
	ModelVersion    string       `json:"model_version" gorm:"size:100;not null;uniqueIndex:idx_audio_analysis_cache,priority:4"`
	Status          string       `json:"status" gorm:"size:24;not null;index"`
	Features        JSONDocument `json:"features" gorm:"type:text;not null"`
	ModelLabels     JSONDocument `json:"model_labels" gorm:"type:text;not null"`
	DurationMS      int64        `json:"duration_ms" gorm:"not null;default:0"`
	ProcessingMS    int64        `json:"processing_ms" gorm:"not null;default:0"`
	ErrorCode       string       `json:"error_code" gorm:"size:64"`
	ErrorSummary    string       `json:"error_summary" gorm:"size:500"`
	CompletedAt     *time.Time   `json:"completed_at,omitempty"`
}

func (*MusicAudioAnalysis) TableName() string {
	return "music_audio_analyses"
}

// MusicAnalysisJob is the durable coordination record. Lease owner and
// generation make claiming safe across future PostgreSQL multi-instance use;
// SQLite remains single-writer by default.
type MusicAnalysisJob struct {
	ID        uint      `json:"id" gorm:"primarykey;index:idx_analysis_music_kind_id,priority:3;index:idx_analysis_claim,priority:4"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	Kind           string `json:"kind" gorm:"size:32;not null;index;index:idx_analysis_music_kind_id,priority:2"`
	IdempotencyKey string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	MusicID        uint   `json:"music_id" gorm:"not null;index;index:idx_analysis_music_kind_id,priority:1"`
	MediaFileID    *uint  `json:"media_file_id,omitempty" gorm:"index"`
	AnalysisID     *uint  `json:"analysis_id,omitempty" gorm:"index"`
	RequestedBy    uint   `json:"requested_by" gorm:"not null;index"`

	FileHash         string `json:"file_hash" gorm:"size:64;index"`
	ObservedFileHash string `json:"observed_file_hash" gorm:"size:64"`
	ContentRevision  uint64 `json:"content_revision" gorm:"not null;default:0"`
	MetadataRevision uint64 `json:"metadata_revision" gorm:"not null;default:0"`
	RuleVersion      string `json:"rule_version" gorm:"size:64"`
	AnalyzerID       string `json:"analyzer_id" gorm:"size:100"`
	AnalyzerVersion  string `json:"analyzer_version" gorm:"size:100"`
	ModelVersion     string `json:"model_version" gorm:"size:100"`

	Status          string     `json:"status" gorm:"size:24;not null;index;index:idx_analysis_claim,priority:1"`
	Attempt         int        `json:"attempt" gorm:"not null;default:0"`
	MaxAttempts     int        `json:"max_attempts" gorm:"not null;default:1"`
	AvailableAt     *time.Time `json:"available_at,omitempty" gorm:"index;index:idx_analysis_claim,priority:3"`
	CancelRequested bool       `json:"cancel_requested" gorm:"not null;default:false;index:idx_analysis_claim,priority:2"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	HeartbeatAt     *time.Time `json:"heartbeat_at,omitempty"`
	FinishedAt      *time.Time `json:"finished_at,omitempty"`
	LeaseOwner      string     `json:"-" gorm:"size:100;index"`
	LeaseExpiresAt  *time.Time `json:"-" gorm:"index"`
	LeaseGeneration uint64     `json:"-" gorm:"not null;default:0"`
	ErrorCode       string     `json:"error_code" gorm:"size:64;index"`
	ErrorSummary    string     `json:"error_summary" gorm:"size:500"`
	ProcessingMS    int64      `json:"processing_ms" gorm:"not null;default:0"`

	Music     Music               `json:"-" gorm:"foreignKey:MusicID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	MediaFile *MediaFile          `json:"-" gorm:"foreignKey:MediaFileID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
	Analysis  *MusicAudioAnalysis `json:"analysis,omitempty" gorm:"foreignKey:AnalysisID;references:ID;constraint:OnUpdate:CASCADE,OnDelete:SET NULL"`
}

func (*MusicAnalysisJob) TableName() string {
	return "music_analysis_jobs"
}

type AnalysisJobListParams struct {
	MusicID  *uint
	Kind     string
	Status   string
	Page     int
	PageSize int
}

type AnalysisQueueMetrics struct {
	Statuses          map[string]int64 `json:"statuses"`
	QueueLength       int64            `json:"queue_length"`
	AverageProcessing int64            `json:"average_processing_ms"`
	FailureRate       float64          `json:"failure_rate"`
}

// MusicAnalysisSummary is the bounded projection exposed with music rows.
// Full analyzer output remains on the administrator-only job detail endpoint.
type MusicAnalysisSummary struct {
	JobID           uint       `json:"job_id"`
	Status          string     `json:"status"`
	AnalyzerID      string     `json:"analyzer_id"`
	AnalyzerVersion string     `json:"analyzer_version"`
	ModelVersion    string     `json:"model_version"`
	Attempt         int        `json:"attempt"`
	ErrorCode       string     `json:"error_code"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
}

func (job *MusicAnalysisJob) ToSummary() *MusicAnalysisSummary {
	if job == nil {
		return nil
	}
	return &MusicAnalysisSummary{
		JobID: job.ID, Status: job.Status, AnalyzerID: job.AnalyzerID,
		AnalyzerVersion: job.AnalyzerVersion, ModelVersion: job.ModelVersion,
		Attempt: job.Attempt, ErrorCode: job.ErrorCode, CompletedAt: job.FinishedAt,
	}
}

// AnalysisMusicCandidate is an internal snapshot used to build idempotent
// jobs. It contains no path: the worker resolves a currently controlled source
// only when execution starts.
type AnalysisMusicCandidate struct {
	Music           *Music
	MediaFileID     *uint
	FileHash        string
	ContentRevision uint64
}

type AnalysisBackfillRequest struct {
	IncludeAudio bool `json:"include_audio"`
}

type AnalysisEnqueueRequest struct {
	IncludeAudio bool `json:"include_audio"`
	Force        bool `json:"force"`
}

type AnalysisScheduleResponse struct {
	MetadataJob *MusicAnalysisJob `json:"metadata_job,omitempty"`
	AudioJob    *MusicAnalysisJob `json:"audio_job,omitempty"`
	Reused      int64             `json:"reused"`
	Skipped     int64             `json:"skipped"`
}

type AnalysisBackfillResponse struct {
	Visited       int64 `json:"visited"`
	RulesQueued   int64 `json:"rules_queued"`
	AudioQueued   int64 `json:"audio_queued"`
	Reused        int64 `json:"reused"`
	Skipped       int64 `json:"skipped"`
	QueueRejected int64 `json:"queue_rejected"`
}
