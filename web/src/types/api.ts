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
  token: string;
  user: UserInfo;
}

export interface UpdateUserProfileData {
  full_name?: string;
  email?: string;
  nickname?: string;
  bio?: string;
}

// ---- 音乐相关 ----

export type MusicType = "single" | "album";

export interface Music {
  id: number;
  title: string;
  artist: string;
  album: string;
  year: number;
  track_number: number;
  genre: string;
  duration: number;
  album_id: number | null;
  intro: string;
  img: string;
  type: MusicType;
  issuing_date: string;
  path: string;
  user_id: number;
  created_at: string;
  updated_at: string;
  like_count?: number;
  is_liked?: boolean;
  cover_url?: string;
}

export interface CreateMusicRequest {
  title: string;
  artist: string;
  album?: string;
  year?: number;
  track_number?: number;
  genre?: string;
  duration?: number;
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
  album?: string;
  year?: number;
  track_number?: number;
  genre?: string;
  duration?: number;
  intro?: string;
  type?: MusicType;
  issuing_date?: string;
  album_id?: number | null;
}

export interface MusicFilterOptions {
  artists: string[];
  years: number[];
  types: MusicType[];
}

export interface MusicMetadataData {
  title: string;
  artist: string;
  album: string;
  year: number;
  track_number: number;
  genre: string;
  duration: number;
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
  album: string;
  year: string;
  track: string;
  genre: string;
  duration: string;
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
