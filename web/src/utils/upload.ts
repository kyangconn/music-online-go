import type { ICommonTagsResult, IFormat } from "music-metadata";
import { parseBlob } from "music-metadata";
import type { Music, MusicMetadataData, MusicMetadataFields } from "@/types/api";

const META_CACHE_PREFIX = "music-meta:v2:";
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

export const emptyMusicMetadataFields = (): MusicMetadataFields => ({
  title: "",
  artist: "",
  artists: [],
  album: "",
  album_artist: "",
  album_artists: [],
  year: "",
  track: "",
  track_total: "",
  disc: "",
  disc_total: "",
  release_date: "",
  original_release_date: "",
  genre: "",
  genres: [],
  comment: "",
  isrcs: [],
  duration: "",
  musicbrainz_recording_id: "",
  musicbrainz_track_id: "",
  musicbrainz_release_id: "",
  musicbrainz_release_group_id: "",
  musicbrainz_artist_ids: [],
  musicbrainz_album_artist_ids: [],
});

/** 根据文件名、大小和修改时间生成元数据缓存键 */
export const makeMetaCacheKey = (file: File) => `${META_CACHE_PREFIX}${file.name}:${file.size}:${file.lastModified}`;

/** 从 localStorage 加载缓存的音频元数据 */
export const loadCachedMeta = (file: File): MusicMetadataFields | null => {
  try {
    const raw = localStorage.getItem(makeMetaCacheKey(file));
    if (raw) {
      const parsed: unknown = JSON.parse(raw);
      if (typeof parsed !== "object" || parsed === null) return null;
      const metadata = emptyMusicMetadataFields();
      for (const key of Object.keys(metadata) as (keyof MusicMetadataFields)[]) {
        const value = (parsed as Record<string, unknown>)[key];
        if (Array.isArray(metadata[key])) {
          if (Array.isArray(value) && value.every((item) => typeof item === "string")) {
            Object.assign(metadata, { [key]: [...value] });
          }
        } else if (typeof value === "string") {
          Object.assign(metadata, { [key]: value });
        }
      }
      return metadata;
    }
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
export const extractMetaFields = (common: ICommonTagsResult, format?: IFormat): MusicMetadataFields => {
  const genres = common.genre?.filter(Boolean) || [];
  const releaseDate = common.releasedate || common.date || "";
  const originalReleaseDate = common.originaldate || (common.originalyear ? String(common.originalyear) : "");
  const year = common.year || Number.parseInt(releaseDate.slice(0, 4), 10);

  return {
    ...emptyMusicMetadataFields(),
    title: common.title || "",
    artist: common.artist || common.artists?.join("; ") || "",
    artists: common.artists || [],
    album: common.album || "",
    album_artist: common.albumartist || common.albumartists?.join("; ") || "",
    album_artists: common.albumartists || [],
    year: Number.isInteger(year) ? String(year) : "",
    track: typeof common.track?.no === "number" ? String(common.track.no) : "",
    track_total: typeof common.track?.of === "number" ? String(common.track.of) : common.totaltracks || "",
    disc: typeof common.disk?.no === "number" ? String(common.disk.no) : "",
    disc_total: typeof common.disk?.of === "number" ? String(common.disk.of) : common.totaldiscs || "",
    release_date: releaseDate,
    original_release_date: originalReleaseDate,
    genre: genres.join("; "),
    genres,
    comment: common.comment?.map((item) => item.text).filter(Boolean).join("\n") || "",
    isrcs: common.isrc || [],
    duration: format?.duration ? formatDuration(format.duration) : "",
    musicbrainz_recording_id: common.musicbrainz_recordingid || "",
    musicbrainz_track_id: common.musicbrainz_trackid || "",
    musicbrainz_release_id: common.musicbrainz_albumid || "",
    musicbrainz_release_group_id: common.musicbrainz_releasegroupid || "",
    musicbrainz_artist_ids: common.musicbrainz_artistid || [],
    musicbrainz_album_artist_ids: common.musicbrainz_albumartistid || [],
  };
};

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
  const raw = value.trim();
  if (!raw) return 0;
  const segments = raw.split(":");
  if (segments.length > 3 || segments.some((part) => !/^\d+$/.test(part))) return -1;
  const parts = segments.map((part) => Number(part));
  if (parts.slice(1).some((part) => part >= 60)) return -1;
  const seconds = parts.reduce((total, part) => total * 60 + part, 0);
  return Number.isSafeInteger(seconds) ? seconds : -1;
};

const parseOptionalMetadataInteger = (value: string) => {
  const raw = value.trim();
  if (!raw) return 0;
  if (!/^\d+$/.test(raw)) return -1;
  const parsed = Number(raw);
  return Number.isSafeInteger(parsed) ? parsed : -1;
};

export const musicToMetadataFields = (music: Music): MusicMetadataFields => ({
  title: music.title,
  artist: music.artist,
  artists: [...music.artists],
  album: music.album,
  album_artist: music.album_artist,
  album_artists: [...music.album_artists],
  year: music.year > 0 ? String(music.year) : "",
  track: music.track_number > 0 ? String(music.track_number) : "",
  track_total: music.track_total > 0 ? String(music.track_total) : "",
  disc: music.disc_number > 0 ? String(music.disc_number) : "",
  disc_total: music.disc_total > 0 ? String(music.disc_total) : "",
  release_date: music.release_date,
  original_release_date: music.original_release_date,
  genre: music.genre,
  genres: [...music.genres],
  comment: music.comment,
  isrcs: [...music.isrcs],
  duration: music.duration > 0 ? formatDuration(music.duration) : "",
  musicbrainz_recording_id: music.musicbrainz_recording_id,
  musicbrainz_track_id: music.musicbrainz_track_id,
  musicbrainz_release_id: music.musicbrainz_release_id,
  musicbrainz_release_group_id: music.musicbrainz_release_group_id,
  musicbrainz_artist_ids: [...music.musicbrainz_artist_ids],
  musicbrainz_album_artist_ids: [...music.musicbrainz_album_artist_ids],
});

export const metadataToData = (
  metadata: MusicMetadataFields,
  fallbackTitle = "",
  fallbackArtist = "",
): MusicMetadataData => {
  const year = parseOptionalMetadataInteger(metadata.year);
  const trackNumber = parseOptionalMetadataInteger(metadata.track);
  const trackTotal = parseOptionalMetadataInteger(metadata.track_total);
  const discNumber = parseOptionalMetadataInteger(metadata.disc);
  const discTotal = parseOptionalMetadataInteger(metadata.disc_total);
  return {
    title: metadata.title.trim() || fallbackTitle,
    artist: metadata.artist.trim() || fallbackArtist,
    artists: metadata.artists.map((item) => item.trim()).filter(Boolean),
    album: metadata.album.trim(),
    album_artist: metadata.album_artist.trim(),
    album_artists: metadata.album_artists.map((item) => item.trim()).filter(Boolean),
    year,
    track_number: trackNumber,
    track_total: trackTotal,
    disc_number: discNumber,
    disc_total: discTotal,
    release_date: metadata.release_date.trim(),
    original_release_date: metadata.original_release_date.trim(),
    genre: metadata.genre.trim(),
    genres: metadata.genres.map((item) => item.trim()).filter(Boolean),
    comment: metadata.comment.trim(),
    isrcs: metadata.isrcs.map((item) => item.trim()).filter(Boolean),
    duration: parseDurationSeconds(metadata.duration),
    musicbrainz_recording_id: metadata.musicbrainz_recording_id.trim(),
    musicbrainz_track_id: metadata.musicbrainz_track_id.trim(),
    musicbrainz_release_id: metadata.musicbrainz_release_id.trim(),
    musicbrainz_release_group_id: metadata.musicbrainz_release_group_id.trim(),
    musicbrainz_artist_ids: metadata.musicbrainz_artist_ids.map((item) => item.trim()).filter(Boolean),
    musicbrainz_album_artist_ids: metadata.musicbrainz_album_artist_ids.map((item) => item.trim()).filter(Boolean),
  };
};

export const applyMetadataSuggestion = (target: MusicMetadataFields, suggestion: MusicMetadataData) => {
  if (!target.title && suggestion.title) target.title = suggestion.title;
  if (!target.artist && suggestion.artist) target.artist = suggestion.artist;
  if (!target.artists.length && suggestion.artists.length) target.artists = [...suggestion.artists];
  if (!target.album && suggestion.album) target.album = suggestion.album;
  if (!target.album_artist && suggestion.album_artist) target.album_artist = suggestion.album_artist;
  if (!target.album_artists.length && suggestion.album_artists.length) target.album_artists = [...suggestion.album_artists];
  if (!target.year && suggestion.year > 0) target.year = String(suggestion.year);
  if (!target.track && suggestion.track_number > 0) target.track = String(suggestion.track_number);
  if (!target.track_total && suggestion.track_total > 0) target.track_total = String(suggestion.track_total);
  if (!target.disc && suggestion.disc_number > 0) target.disc = String(suggestion.disc_number);
  if (!target.disc_total && suggestion.disc_total > 0) target.disc_total = String(suggestion.disc_total);
  if (!target.release_date && suggestion.release_date) target.release_date = suggestion.release_date;
  if (!target.original_release_date && suggestion.original_release_date) {
    target.original_release_date = suggestion.original_release_date;
  }
  if (!target.genre && suggestion.genre) target.genre = suggestion.genre;
  if (!target.genres.length && suggestion.genres.length) target.genres = [...suggestion.genres];
  if (!target.comment && suggestion.comment) target.comment = suggestion.comment;
  if (!target.isrcs.length && suggestion.isrcs.length) target.isrcs = [...suggestion.isrcs];
  if (!target.duration && suggestion.duration > 0) target.duration = formatDuration(suggestion.duration);
  if (!target.musicbrainz_recording_id && suggestion.musicbrainz_recording_id) {
    target.musicbrainz_recording_id = suggestion.musicbrainz_recording_id;
  }
  if (!target.musicbrainz_track_id && suggestion.musicbrainz_track_id) {
    target.musicbrainz_track_id = suggestion.musicbrainz_track_id;
  }
  if (!target.musicbrainz_release_id && suggestion.musicbrainz_release_id) {
    target.musicbrainz_release_id = suggestion.musicbrainz_release_id;
  }
  if (!target.musicbrainz_release_group_id && suggestion.musicbrainz_release_group_id) {
    target.musicbrainz_release_group_id = suggestion.musicbrainz_release_group_id;
  }
  if (!target.musicbrainz_artist_ids.length && suggestion.musicbrainz_artist_ids.length) {
    target.musicbrainz_artist_ids = [...suggestion.musicbrainz_artist_ids];
  }
  if (!target.musicbrainz_album_artist_ids.length && suggestion.musicbrainz_album_artist_ids.length) {
    target.musicbrainz_album_artist_ids = [...suggestion.musicbrainz_album_artist_ids];
  }
};
