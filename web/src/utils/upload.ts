import type { ICommonTagsResult, IFormat } from "music-metadata"
import { parseBlob } from "music-metadata"
import type { MusicMetadataFields } from "@/types/api"

const META_CACHE_PREFIX = "music-meta:"

/** 根据文件名和大小生成元数据缓存键 */
export const makeMetaCacheKey = (file: File) => `${META_CACHE_PREFIX}${file.name}:${file.size}`

/** 从 localStorage 加载缓存的音频元数据 */
export const loadCachedMeta = (file: File): MusicMetadataFields | null => {
  try {
    const raw = localStorage.getItem(makeMetaCacheKey(file))
    if (raw) return JSON.parse(raw)
  } catch {
    /* corrupted */
  }
  return null
}

/** 将音频元数据序列化后保存到 localStorage */
export const saveCachedMeta = (file: File, meta: MusicMetadataFields) => {
  try {
    localStorage.setItem(makeMetaCacheKey(file), JSON.stringify(meta))
  } catch {
    /* storage full */
  }
}

/** 从 localStorage 移除指定文件的缓存元数据 */
export const removeCachedMeta = (file: File) => {
  try {
    localStorage.removeItem(makeMetaCacheKey(file))
  } catch {
    /* ignore */
  }
}

/** 从 music-metadata 原始解析结果中提取标准化的元数据字段 */
export const extractMetaFields = (common: ICommonTagsResult, format?: IFormat): MusicMetadataFields => ({
  title: common.title || "",
  artist: common.artist || "",
  album: common.album || "",
  year: typeof common.year === "number" ? String(common.year) : "",
  track: common.track && typeof common.track.no === "number" ? String(common.track.no) : "",
  genre: common.genre?.length ? common.genre.join("; ") : "",
  duration: format?.duration ? String(Math.round(format.duration)) : "",
})

/** 解析音频文件并返回标准化的元数据字段 */
export const parseAudioFile = async (file: File) => {
  const metadata = await parseBlob(file)
  return extractMetaFields(metadata.common, metadata.format)
}

/** 将字节数格式化为人类可读的文件大小字符串 */
export const formatFileSize = (bytes: number) => {
  if (bytes === 0) return "0 B"
  const k = 1024
  const sizes = ["B", "KB", "MB", "GB"]
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + " " + sizes[i]
}

/** 将秒数格式化为 mm:ss 时长字符串 */
export const formatDuration = (seconds: number) => {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, "0")}`
}
