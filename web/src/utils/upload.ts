import type { ICommonTagsResult, IFormat } from "music-metadata";
import { parseBlob } from "music-metadata";
import type { MusicMetadataData, MusicMetadataFields } from "@/types/api";

const META_CACHE_PREFIX = "music-meta:";
const MB = 1024 * 1024;

export const MAX_AUDIO_SIZE_BYTES = 200 * MB;
export const MAX_COVER_SIZE_BYTES = 10 * MB;

const AUDIO_EXTENSIONS = new Set([".aac", ".aif", ".aiff", ".ape", ".flac", ".m4a", ".mp3", ".ogg", ".opus", ".wav", ".wma"]);
const COVER_EXTENSIONS = new Set([".gif", ".jpeg", ".jpg", ".png", ".webp"]);

type UploadFileKind = "audio" | "cover";
type UploadValidationReason = "empty" | "extension" | "mime" | "too_large";

export interface UploadValidationResult {
  valid: boolean;
  reason?: UploadValidationReason;
  ext?: string;
  maxSize?: string;
  mime?: string;
}

export interface UploadPolicy {
  max_audio_size_bytes: number;
  max_cover_size_bytes: number;
  max_audio_size_mb: number;
  max_cover_size_mb: number;
  audio_extensions: string[];
  cover_extensions: string[];
}

export const DEFAULT_UPLOAD_POLICY: UploadPolicy = {
  max_audio_size_bytes: MAX_AUDIO_SIZE_BYTES,
  max_cover_size_bytes: MAX_COVER_SIZE_BYTES,
  max_audio_size_mb: 200,
  max_cover_size_mb: 10,
  audio_extensions: [...AUDIO_EXTENSIONS],
  cover_extensions: [...COVER_EXTENSIONS],
};

/** 根据文件名、大小和修改时间生成元数据缓存键 */
export const makeMetaCacheKey = (file: File) => `${META_CACHE_PREFIX}${file.name}:${file.size}:${file.lastModified}`;

/** 从 localStorage 加载缓存的音频元数据 */
export const loadCachedMeta = (file: File): MusicMetadataFields | null => {
  try {
    const raw = localStorage.getItem(makeMetaCacheKey(file));
    if (raw) return JSON.parse(raw);
  } catch {
    /* corrupted */
  }
  return null;
};

/** 将音频元数据序列化后保存到 localStorage */
export const saveCachedMeta = (file: File, meta: MusicMetadataFields) => {
  try {
    localStorage.setItem(makeMetaCacheKey(file), JSON.stringify(meta));
  } catch {
    /* storage full */
  }
};

/** 从 localStorage 移除指定文件的缓存元数据 */
export const removeCachedMeta = (file: File) => {
  try {
    localStorage.removeItem(makeMetaCacheKey(file));
  } catch {
    /* ignore */
  }
};

const getFileExtension = (name: string) => {
  const dot = name.lastIndexOf(".");
  return dot >= 0 ? name.slice(dot).toLowerCase() : "";
};

export const isSupportedAudioFileName = (name: string, policy: UploadPolicy = DEFAULT_UPLOAD_POLICY) =>
  new Set(policy.audio_extensions.map((ext) => ext.toLowerCase())).has(getFileExtension(name));

export const validateUploadFile = (
  file: File,
  kind: UploadFileKind,
  policy: UploadPolicy = DEFAULT_UPLOAD_POLICY,
): UploadValidationResult => {
  if (file.size <= 0) return { valid: false, reason: "empty" };

  const ext = getFileExtension(file.name);
  const allowedExts = new Set((kind === "audio" ? policy.audio_extensions : policy.cover_extensions).map((item) => item.toLowerCase()));
  if (!allowedExts.has(ext)) {
    return { valid: false, reason: "extension", ext: ext || "(none)" };
  }

  const maxBytes = kind === "audio" ? policy.max_audio_size_bytes : policy.max_cover_size_bytes;
  if (file.size > maxBytes) {
    return { valid: false, reason: "too_large", maxSize: formatFileSize(maxBytes) };
  }

  const mime = file.type.toLowerCase();
  const expectedPrefix = kind === "audio" ? "audio/" : "image/";
  if (mime && !mime.startsWith(expectedPrefix)) {
    return { valid: false, reason: "mime", mime };
  }

  return { valid: true };
};

export const getUploadValidationMessage = (
  file: File,
  kind: UploadFileKind,
  result: UploadValidationResult,
  t: (key: string, params?: Record<string, unknown>) => string,
) => {
  const label = kind === "audio" ? t("add.audio") : t("add.cover");
  switch (result.reason) {
    case "empty":
      return t("add.invalid_file_empty", { name: file.name, label });
    case "extension":
      return t("add.invalid_file_extension", { name: file.name, label, ext: result.ext });
    case "mime":
      return t("add.invalid_file_mime", { name: file.name, label, mime: result.mime });
    case "too_large":
      return t("add.invalid_file_too_large", { name: file.name, label, max: result.maxSize });
    default:
      return t("add.upload_failed");
  }
};

/** 从 music-metadata 原始解析结果中提取标准化的元数据字段 */
export const extractMetaFields = (common: ICommonTagsResult, format?: IFormat): MusicMetadataFields => ({
  title: common.title || "",
  artist: common.artist || "",
  album: common.album || "",
  year: typeof common.year === "number" ? String(common.year) : "",
  track: common.track && typeof common.track.no === "number" ? String(common.track.no) : "",
  genre: common.genre?.length ? common.genre.join("; ") : "",
  duration: format?.duration ? formatDuration(format.duration) : "",
});

/** 解析音频文件并返回标准化的元数据字段 */
export const parseAudioFile = async (file: File) => {
  const metadata = await parseBlob(file);
  return extractMetaFields(metadata.common, metadata.format);
};

/** 将字节数格式化为人类可读的文件大小字符串 */
export const formatFileSize = (bytes: number) => {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i];
};

/** 将秒数格式化为 mm:ss 时长字符串 */
export const formatDuration = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds < 0) return "0:00";
  const mins = Math.floor(seconds / 60);
  const secs = Math.floor(seconds % 60);
  return `${mins}:${secs.toString().padStart(2, "0")}`;
};

export const parseDurationSeconds = (value: string) => {
  const parts = value
    .trim()
    .split(":")
    .map((part) => Number(part));
  if (parts.length === 0 || parts.length > 3 || parts.some((part) => !Number.isFinite(part) || part < 0)) return 0;
  return Math.round(parts.reduce((total, part) => total * 60 + part, 0));
};

export const metadataToData = (
  metadata: MusicMetadataFields,
  fallbackTitle = "",
  fallbackArtist = "",
): MusicMetadataData => {
  const year = Number.parseInt(metadata.year, 10);
  const trackNumber = Number.parseInt(metadata.track, 10);
  return {
    title: metadata.title.trim() || fallbackTitle,
    artist: metadata.artist.trim() || fallbackArtist,
    album: metadata.album.trim(),
    year: year >= 1000 && year <= 9999 ? year : 0,
    track_number: trackNumber > 0 ? trackNumber : 0,
    genre: metadata.genre.trim(),
    duration: parseDurationSeconds(metadata.duration),
  };
};

export const applyMetadataSuggestion = (target: MusicMetadataFields, suggestion: MusicMetadataData) => {
  if (!target.title && suggestion.title) target.title = suggestion.title;
  if (!target.artist && suggestion.artist) target.artist = suggestion.artist;
  if (!target.album && suggestion.album) target.album = suggestion.album;
  if (!target.year && suggestion.year > 0) target.year = String(suggestion.year);
  if (!target.track && suggestion.track_number > 0) target.track = String(suggestion.track_number);
  if (!target.genre && suggestion.genre) target.genre = suggestion.genre;
  if (!target.duration && suggestion.duration > 0) target.duration = formatDuration(suggestion.duration);
};
