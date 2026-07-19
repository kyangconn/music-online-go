package domain

import (
	"time"

	"gorm.io/gorm"
)

const (
	ManagedMediaRootID  uint = 0
	ManagedMediaRootKey      = "managed"

	MediaScanStatusPending   = "pending"
	MediaScanStatusRunning   = "running"
	MediaScanStatusRetryWait = "retry_wait"
	MediaScanStatusSucceeded = "succeeded"
	MediaScanStatusFailed    = "failed"
	MediaScanStatusCancelled = "cancelled"

	MediaStorageKindManaged = "managed"
	MediaStorageKindAuto    = "auto"
	MediaStorageKindLocal   = "local"
	MediaStorageKindNFS     = "nfs"
	MediaStorageKindSMB     = "smb"

	MediaPathSemanticsAuto            = "auto"
	MediaPathSemanticsCaseSensitive   = "case_sensitive"
	MediaPathSemanticsCaseInsensitive = "case_insensitive"

	MediaRootHealthUnknown  = "unknown"
	MediaRootHealthOnline   = "online"
	MediaRootHealthDegraded = "degraded"
	MediaRootHealthOffline  = "offline"

	MediaFileAvailabilityUnknown = "unknown"
	MediaFileAvailabilityOnline  = "online"
	MediaFileAvailabilityMissing = "missing"
	MediaFileAvailabilityOffline = "offline"
	MediaFileAvailabilityChanged = "changed"
)

// MediaLibraryRoot is an administrator-registered, read-only source directory.
// The managed upload directory is configuration-owned and is represented by
// ManagedMediaRootID in API responses rather than by a row in this table.
type MediaLibraryRoot struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Key is stable across path changes and is suitable for future M5 analysis
	// ownership. The numeric ID remains the database relation key.
	Key                string `json:"key" gorm:"size:64;index"`
	Name               string `json:"name" gorm:"size:100;not null"`
	Path               string `json:"path" gorm:"size:1000;not null"`
	StorageKind        string `json:"storage_kind" gorm:"size:16;not null;default:'auto'"`
	ExpectedFilesystem string `json:"expected_filesystem" gorm:"size:64"`
	ProbeFile          string `json:"probe_file" gorm:"size:500"`
	PathSemantics      string `json:"path_semantics" gorm:"size:32;not null;default:'auto'"`
	Enabled            bool   `json:"enabled" gorm:"not null;default:true;index"`
	ReadOnly           bool   `json:"read_only" gorm:"not null;default:true"`
	CreatedBy          uint   `json:"created_by" gorm:"not null;index"`
}

func (*MediaLibraryRoot) TableName() string {
	return "media_library_roots"
}

// MediaLibraryRootState keeps operational health separate from administrator
// intent. A transient NFS outage must not mutate or disable the registered root.
type MediaLibraryRootState struct {
	RootID        uint       `json:"root_id" gorm:"primaryKey;autoIncrement:false"`
	Status        string     `json:"status" gorm:"size:16;not null;default:'unknown';index"`
	Code          string     `json:"code" gorm:"size:64"`
	Message       string     `json:"message" gorm:"size:500"`
	Filesystem    string     `json:"filesystem" gorm:"size:64"`
	MountSource   string     `json:"mount_source" gorm:"size:500"`
	Retryable     bool       `json:"retryable" gorm:"not null;default:false"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	LastOnlineAt  *time.Time `json:"last_online_at,omitempty"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (*MediaLibraryRootState) TableName() string {
	return "media_library_root_states"
}

// MediaFile is the physical-source layer beneath Music. Multiple rows may
// point to one logical track, allowing a future analyzer (M5) to key artifacts
// by content hash while playback chooses an available local/NFS/SMB copy.
type MediaFile struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	MusicID      uint   `json:"music_id" gorm:"not null;index"`
	RootID       uint   `json:"root_id" gorm:"not null;index"`
	RelativePath string `json:"relative_path" gorm:"type:text;not null"`
	SourceKey    string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	FileHash     string `json:"file_hash" gorm:"size:64;index"`
	// ObservedFileHash diverges from FileHash when bytes at a known path are
	// replaced. M5 can mark old analysis stale without silently relinking the
	// logical track before an administrator accepts the replacement.
	ObservedFileHash string     `json:"observed_file_hash" gorm:"size:64;index"`
	FileSize         int64      `json:"file_size" gorm:"not null;default:0"`
	FileModTime      *time.Time `json:"file_mod_time,omitempty"`
	// ReadOnly has no GORM default because managed uploads intentionally store
	// false. A default:true tag makes GORM replace that zero value during Create.
	ReadOnly        bool       `json:"read_only" gorm:"not null"`
	Availability    string     `json:"availability" gorm:"size:16;not null;default:'unknown';index"`
	ContentRevision uint64     `json:"content_revision" gorm:"not null;default:1"`
	LastSeenAt      *time.Time `json:"last_seen_at,omitempty"`
	LastVerifiedAt  *time.Time `json:"last_verified_at,omitempty"`
}

func (*MediaFile) TableName() string {
	return "media_files"
}

// MediaScanJob persists scan progress so an administrator can inspect the
// outcome after the worker or browser has gone away. Jobs are append-only;
// cancelling a job changes its state but does not remove its history.
type MediaScanJob struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	RootID          uint   `json:"root_id" gorm:"not null;index"`
	RootName        string `json:"root_name" gorm:"size:100;not null"`
	RequestedBy     uint   `json:"requested_by" gorm:"not null;index"`
	Status          string `json:"status" gorm:"size:32;not null;index"`
	CancelRequested bool   `json:"cancel_requested" gorm:"not null;default:false"`

	DiscoveredCount int64 `json:"discovered_count" gorm:"not null;default:0"`
	ProcessedCount  int64 `json:"processed_count" gorm:"not null;default:0"`
	ImportedCount   int64 `json:"imported_count" gorm:"not null;default:0"`
	ExistingCount   int64 `json:"existing_count" gorm:"not null;default:0"`
	DuplicateCount  int64 `json:"duplicate_count" gorm:"not null;default:0"`
	SkippedCount    int64 `json:"skipped_count" gorm:"not null;default:0"`
	WarningCount    int64 `json:"warning_count" gorm:"not null;default:0"`
	FailedCount     int64 `json:"failed_count" gorm:"not null;default:0"`

	StartedAt    *time.Time `json:"started_at,omitempty"`
	HeartbeatAt  *time.Time `json:"heartbeat_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	ErrorSummary string     `json:"error_summary" gorm:"size:500"`

	Attempt          int        `json:"attempt" gorm:"not null;default:0"`
	NextAttemptAt    *time.Time `json:"next_attempt_at,omitempty" gorm:"index"`
	FailureCode      string     `json:"failure_code" gorm:"size:64"`
	FailureRetryable bool       `json:"failure_retryable" gorm:"not null;default:false"`
	LeaseOwner       string     `json:"-" gorm:"size:64;index"`
	LeaseExpiresAt   *time.Time `json:"-" gorm:"index"`
	// LeaseGeneration is a fencing token: a stale worker cannot overwrite a
	// job after another process has reclaimed an expired lease.
	LeaseGeneration uint64 `json:"-" gorm:"not null;default:0"`
}

func (*MediaScanJob) TableName() string {
	return "media_scan_jobs"
}

type MediaScanIssue struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CreatedAt time.Time `json:"created_at"`

	JobID        uint   `json:"job_id" gorm:"not null;index"`
	RelativePath string `json:"relative_path" gorm:"size:1000;not null"`
	Severity     string `json:"severity" gorm:"size:16;not null"`
	Code         string `json:"code" gorm:"size:64;not null;index"`
	Message      string `json:"message" gorm:"size:500;not null"`
}

func (*MediaScanIssue) TableName() string {
	return "media_scan_issues"
}

type CreateMediaLibraryRootRequest struct {
	Name               string `json:"name" binding:"required,max=100"`
	Path               string `json:"path" binding:"required,max=1000"`
	StorageKind        string `json:"storage_kind" binding:"omitempty,oneof=auto local nfs smb"`
	ExpectedFilesystem string `json:"expected_filesystem" binding:"omitempty,max=64"`
	ProbeFile          string `json:"probe_file" binding:"omitempty,max=500"`
	PathSemantics      string `json:"path_semantics" binding:"omitempty,oneof=auto case_sensitive case_insensitive"`
}

type UpdateMediaLibraryRootRequest struct {
	Name               *string `json:"name" binding:"omitempty,max=100"`
	Path               *string `json:"path" binding:"omitempty,max=1000"`
	StorageKind        *string `json:"storage_kind" binding:"omitempty,oneof=auto local nfs smb"`
	ExpectedFilesystem *string `json:"expected_filesystem" binding:"omitempty,max=64"`
	ProbeFile          *string `json:"probe_file" binding:"omitempty,max=500"`
	PathSemantics      *string `json:"path_semantics" binding:"omitempty,oneof=auto case_sensitive case_insensitive"`
	Enabled            *bool   `json:"enabled"`
}

type MediaLibraryRootResponse struct {
	ID                 uint                           `json:"id"`
	CreatedAt          time.Time                      `json:"created_at"`
	UpdatedAt          time.Time                      `json:"updated_at"`
	Name               string                         `json:"name"`
	Path               string                         `json:"path"`
	Kind               string                         `json:"kind"`
	Key                string                         `json:"key"`
	StorageKind        string                         `json:"storage_kind"`
	ExpectedFilesystem string                         `json:"expected_filesystem"`
	ProbeFile          string                         `json:"probe_file"`
	PathSemantics      string                         `json:"path_semantics"`
	Enabled            bool                           `json:"enabled"`
	ReadOnly           bool                           `json:"read_only"`
	CreatedBy          uint                           `json:"created_by"`
	Health             MediaLibraryRootHealthResponse `json:"health"`
}

type MediaLibraryRootHealthResponse struct {
	Status        string     `json:"status"`
	Code          string     `json:"code"`
	Message       string     `json:"message"`
	Filesystem    string     `json:"filesystem"`
	Retryable     bool       `json:"retryable"`
	LastCheckedAt *time.Time `json:"last_checked_at,omitempty"`
	LastOnlineAt  *time.Time `json:"last_online_at,omitempty"`
}

type MediaScanJobDetail struct {
	MediaScanJob
	Issues []*MediaScanIssue `json:"issues"`
}
