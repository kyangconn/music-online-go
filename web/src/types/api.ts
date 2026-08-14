/**
 * API 响应类型定义
 * 基于 Go 后端 handler.Response 结构体
 */

/** 后端统一响应包装 */
export interface ApiResponse<T = unknown> {
  code: number;
  message: string;
  data: T;
  error?: string;
}

/** 分页响应 */
export interface PaginatedData<T> {
  items: T[];
  total: number;
  page?: number;
  size?: number;
}

// ---- 用户相关 ----

export interface UserInfo {
  id: number;
  username: string;
  email: string;
  full_name: string;
  nickname: string;
  bio: string;
  role: "user" | "admin";
  is_active: boolean;
  totp_enabled: boolean;
  created_at: string;
  updated_at: string;
}

export interface LoginData {
  access_token: string;
  expires_in: number;
  user: UserInfo;
}

/** /users/refresh 响应：仅包含新的短期 access token 与有效期 */
export interface RefreshData {
  access_token: string;
  expires_in: number;
}

export interface UpdateUserProfileData {
  full_name?: string;
  email?: string;
  nickname?: string;
  bio?: string;
}

// ---- 音乐相关 ----

export type MusicType = "single" | "album";
export type PresetID = "calm_flow" | "kinetic_pulse" | "cosmic_drift" | "bass_impact";
export type PresetStatus = "classified" | "needs_review" | "unclassified";
export type AnalysisStatus = "pending" | "running" | "succeeded" | "failed" | "stale" | "cancelled";
export type AnalysisJobKind = "metadata_rules" | "audio_analysis";

export interface PresetEvidence {
  source: "genre" | "model" | "dsp" | "metadata_audio" | "external";
  key: string;
  weight: number;
}

export interface PresetScore {
  preset_id: PresetID;
  score: number;
  evidence: PresetEvidence[];
}

export interface PresetClassification {
  primary_preset: PresetID | "";
  automatic_preset: PresetID | "";
  manual_preset?: PresetID;
  effective_preset: PresetID | "";
  effective_source: "automatic" | "manual" | "none";
  confidence: number;
  status: PresetStatus;
  rule_version: string;
  metadata_revision: number;
  audio_analysis_id?: number;
  evidence_summary: string[];
  scores: PresetScore[];
  evaluated_at: string;
  manual_updated_at?: string;
}

export interface PresetSummary {
  preset_id: PresetID;
  track_count: number;
  needs_review_count: number;
}

export interface BatchPresetOverrideResponse {
  updated: number;
  classifications: PresetClassification[];
}

export interface MusicAnalysisSummary {
  job_id: number;
  status: AnalysisStatus;
  analyzer_id: string;
  analyzer_version: string;
  model_version: string;
  attempt: number;
  error_code: string;
  completed_at?: string;
}

export interface MusicAudioAnalysis {
  id: number;
  file_hash: string;
  analyzer_id: string;
  analyzer_version: string;
  model_version: string;
  status: AnalysisStatus;
  features: Record<string, unknown>;
  model_labels: Record<string, number>;
  duration_ms: number;
  processing_ms: number;
  error_code: string;
  error_summary: string;
  completed_at?: string;
}

export interface MusicAnalysisJob {
  id: number;
  created_at: string;
  updated_at: string;
  kind: AnalysisJobKind;
  music_id: number;
  media_file_id?: number;
  analysis_id?: number;
  requested_by: number;
  file_hash: string;
  observed_file_hash: string;
  content_revision: number;
  metadata_revision: number;
  rule_version: string;
  analyzer_id: string;
  analyzer_version: string;
  model_version: string;
  status: AnalysisStatus;
  attempt: number;
  max_attempts: number;
  available_at?: string;
  cancel_requested: boolean;
  started_at?: string;
  heartbeat_at?: string;
  finished_at?: string;
  error_code: string;
  error_summary: string;
  processing_ms: number;
  analysis?: MusicAudioAnalysis;
}

export interface AnalysisScheduleResponse {
  metadata_job?: MusicAnalysisJob;
  audio_job?: MusicAnalysisJob;
  reused: number;
  skipped: number;
}

export interface AnalysisBackfillResponse {
  visited: number;
  rules_queued: number;
  audio_queued: number;
  reused: number;
  skipped: number;
  queue_rejected: number;
}

export interface AnalysisQueueMetrics {
  statuses: Partial<Record<AnalysisStatus, number>>;
  queue_length: number;
  average_processing_ms: number;
  failure_rate: number;
}

export interface Music {
  id: number;
  title: string;
  artist: string;
  artists: string[];
  album: string;
  album_artist: string;
  album_artists: string[];
  year: number;
  track_number: number;
  track_total: number;
  disc_number: number;
  disc_total: number;
  release_date: string;
  original_release_date: string;
  genre: string;
  genres: string[];
  genre_tokens: string[];
  comment: string;
  isrcs: string[];
  duration: number;
  musicbrainz_recording_id: string;
  musicbrainz_track_id: string;
  musicbrainz_release_id: string;
  musicbrainz_release_group_id: string;
  musicbrainz_artist_ids: string[];
  musicbrainz_album_artist_ids: string[];
  metadata_revision: number;
  artist_keys: string[];
  album_key?: string;
  album_artist_key?: string;
  preset_classification?: PresetClassification;
  audio_analysis?: MusicAnalysisSummary;
  album_id: number | null;
  intro: string;
  img: string;
  type: MusicType;
  issuing_date: string;
  path: string;
  user_id: number;
  source_read_only: boolean;
  created_at: string;
  updated_at: string;
  like_count?: number;
  is_liked?: boolean;
  cover_url?: string;
  media_url_expires_at?: string;
}

export interface CreateMusicRequest {
  title: string;
  artist: string;
  artists?: string[];
  album?: string;
  album_artist?: string;
  album_artists?: string[];
  year?: number;
  track_number?: number;
  track_total?: number;
  disc_number?: number;
  disc_total?: number;
  release_date?: string;
  original_release_date?: string;
  genre?: string;
  genres?: string[];
  comment?: string;
  isrcs?: string[];
  duration?: number;
  musicbrainz_recording_id?: string;
  musicbrainz_track_id?: string;
  musicbrainz_release_id?: string;
  musicbrainz_release_group_id?: string;
  musicbrainz_artist_ids?: string[];
  musicbrainz_album_artist_ids?: string[];
  intro?: string;
  img?: string;
  path?: string;
  type?: MusicType;
  issuing_date?: string;
  album_id?: number | null;
}

export type CreateMusicData = Music;

export interface UpdateMusicRequest {
  title?: string;
  artist?: string;
  artists?: string[];
  album?: string;
  album_artist?: string;
  album_artists?: string[];
  year?: number;
  track_number?: number;
  track_total?: number;
  disc_number?: number;
  disc_total?: number;
  release_date?: string;
  original_release_date?: string;
  genre?: string;
  genres?: string[];
  comment?: string;
  isrcs?: string[];
  duration?: number;
  musicbrainz_recording_id?: string;
  musicbrainz_track_id?: string;
  musicbrainz_release_id?: string;
  musicbrainz_release_group_id?: string;
  musicbrainz_artist_ids?: string[];
  musicbrainz_album_artist_ids?: string[];
  intro?: string;
  type?: MusicType;
  issuing_date?: string;
  album_id?: number | null;
}

export interface MusicFilterOptions {
  artists: string[];
  albums: string[];
  album_artists: string[];
  genres: string[];
  years: number[];
  types: MusicType[];
}

export interface ArtistSummary {
  key: string;
  name: string;
  musicbrainz_artist_id: string;
  track_count: number;
  album_count: number;
  cover_url?: string;
  cover_url_expires_at?: string;
}

export interface AlbumSummary {
  key: string;
  title: string;
  album_artist: string;
  album_artist_key?: string;
  musicbrainz_release_id: string;
  musicbrainz_release_group_id: string;
  year: number;
  track_count: number;
  total_duration: number;
  disc_count: number;
  cover_url?: string;
  cover_url_expires_at?: string;
}

export interface Playlist {
  id: number;
  created_at: string;
  updated_at: string;
  user_id: number;
  name: string;
  description: string;
  revision: number;
  item_count: number;
}

export interface PlaylistItem {
  position: number;
  music: Music;
}

export interface PlaylistDetail extends Playlist {
  items: PlaylistItem[];
}

export interface MusicMetadataData {
  title: string;
  artist: string;
  artists: string[];
  album: string;
  album_artist: string;
  album_artists: string[];
  year: number;
  track_number: number;
  track_total: number;
  disc_number: number;
  disc_total: number;
  release_date: string;
  original_release_date: string;
  genre: string;
  genres: string[];
  comment: string;
  isrcs: string[];
  duration: number;
  musicbrainz_recording_id: string;
  musicbrainz_track_id: string;
  musicbrainz_release_id: string;
  musicbrainz_release_group_id: string;
  musicbrainz_artist_ids: string[];
  musicbrainz_album_artist_ids: string[];
}

export interface MusicDuplicateCheckData {
  exact_match?: Music;
  metadata_matches: Music[];
  suggested_metadata: MusicMetadataData;
  enrichment?: UpdateMusicRequest;
}

// ---- TOTP 相关 ----

export interface TOTPSetupData {
  secret: string;
  qr_code_url: string;
}

// ---- 管理面板：系统信息 ----

export interface SystemInfoData {
  // 服务器
  host: string;
  server_mode: string;
  server_port: string;
  app_version: string;
  app_commit: string;
  app_built: string;
  app_time: string;
  uptime: string;
  // Go 运行时
  go_version: string;
  num_cpu: number;
  goroutines: number;
  // 内存
  memory_alloc: string;
  memory_total_alloc: string;
  memory_sys: string;
  heap_alloc: string;
  heap_sys: string;
  heap_idle: string;
  heap_inuse: string;
  heap_released: string;
  heap_objects: number;
  stack_inuse: string;
  stack_sys: string;
  // GC
  num_gc: number;
  pause_total: string;
  last_gc_time: string;
  gc_cpu_fraction: string;
  // 数据库
  db_max_open_conns: number;
  db_open_conns: number;
  db_in_use: number;
  db_idle: number;
  db_wait_count: number;
  db_wait_duration: string;
  db_type: string;
  db_name: string;
  // 统计
  total_users: number;
  total_music: number;
  total_music_tags: number;
}

// ---- 批量上传 ----

export interface MusicMetadataFields {
  title: string;
  artist: string;
  artists: string[];
  album: string;
  album_artist: string;
  album_artists: string[];
  year: string;
  track: string;
  track_total: string;
  disc: string;
  disc_total: string;
  release_date: string;
  original_release_date: string;
  genre: string;
  genres: string[];
  comment: string;
  isrcs: string[];
  duration: string;
  musicbrainz_recording_id: string;
  musicbrainz_track_id: string;
  musicbrainz_release_id: string;
  musicbrainz_release_group_id: string;
  musicbrainz_artist_ids: string[];
  musicbrainz_album_artist_ids: string[];
}

export interface InstanceCapabilities {
  library_mode: "public" | "authenticated";
  registration_mode: "open" | "admin";
  registration_open: boolean;
  musicbee_submit_enabled: boolean;
  classification_enabled: boolean;
  audio_analyzer_enabled: boolean;
  analyze_on_upload: boolean;
}

export interface AdminCreateUserRequest {
  username: string;
  email: string;
  password: string;
  full_name: string;
  role: "user" | "admin";
}

export interface MediaLibraryRoot {
  id: number;
  created_at: string;
  updated_at: string;
  name: string;
  path: string;
  kind: "managed" | "read_only";
  key: string;
  storage_kind: "managed" | "auto" | "local" | "nfs" | "smb";
  expected_filesystem: string;
  probe_file: string;
  path_semantics: "auto" | "case_sensitive" | "case_insensitive";
  enabled: boolean;
  read_only: boolean;
  created_by: number;
  health: MediaLibraryRootHealth;
}

export interface MediaLibraryRootHealth {
  status: "unknown" | "online" | "degraded" | "offline";
  code: string;
  message: string;
  filesystem: string;
  retryable: boolean;
  last_checked_at?: string;
  last_online_at?: string;
}

export type MediaScanStatus = "pending" | "running" | "retry_wait" | "succeeded" | "failed" | "cancelled";

export interface MediaScanJob {
  id: number;
  created_at: string;
  updated_at: string;
  root_id: number;
  root_name: string;
  requested_by: number;
  status: MediaScanStatus;
  cancel_requested: boolean;
  discovered_count: number;
  processed_count: number;
  imported_count: number;
  existing_count: number;
  duplicate_count: number;
  skipped_count: number;
  warning_count: number;
  failed_count: number;
  started_at?: string;
  heartbeat_at?: string;
  finished_at?: string;
  error_summary: string;
  attempt: number;
  next_attempt_at?: string;
  failure_code: string;
  failure_retryable: boolean;
}

export interface MediaScanIssue {
  id: number;
  created_at: string;
  job_id: number;
  relative_path: string;
  severity: "warning" | "error";
  code: string;
  message: string;
}

export interface MediaScanJobDetail extends MediaScanJob {
  issues: MediaScanIssue[];
}

export interface ScannedFileItem {
  handle?: FileSystemFileHandle;
  file: File;
  name: string;
  path: string;
  size: number;
  type: string;
  metadata: MusicMetadataFields | null;
  loading: boolean;
  hash?: string;
  duplicateOf?: string;
  exactMatch?: Music;
  metadataMatches?: Music[];
  enrichment?: UpdateMusicRequest;
  processingError?: boolean;
}
