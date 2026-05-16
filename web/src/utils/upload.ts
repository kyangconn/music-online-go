import { parseBlob } from 'music-metadata'

const META_CACHE_PREFIX = 'music-meta:'

export const makeMetaCacheKey = (file: File) =>
  `${META_CACHE_PREFIX}${file.name}:${file.size}`

export const loadCachedMeta = (file: File): Record<string, string> | null => {
  try {
    const raw = localStorage.getItem(makeMetaCacheKey(file))
    if (raw) return JSON.parse(raw)
  } catch {
    /* corrupted */
  }
  return null
}

export const saveCachedMeta = (file: File, meta: Record<string, string>) => {
  try {
    localStorage.setItem(makeMetaCacheKey(file), JSON.stringify(meta))
  } catch {
    /* storage full */
  }
}

export const removeCachedMeta = (file: File) => {
  try {
    localStorage.removeItem(makeMetaCacheKey(file))
  } catch {
    /* ignore */
  }
}

export const extractMetaFields = (common: any, format?: any) => ({
  title: common.title || '',
  artist: common.artist || '',
  album: common.album || '',
  year: typeof common.year === 'number' ? String(common.year) : '',
  track: common.track && typeof common.track.no === 'number' ? String(common.track.no) : '',
  genre: common.genre?.length ? common.genre.join('; ') : '',
  duration: format?.duration ? String(Math.round(format.duration)) : '',
})

export const parseAudioFile = async (file: File) => {
  const metadata = await parseBlob(file)
  return extractMetaFields(metadata.common, metadata.format)
}

export const formatFileSize = (bytes: number) => {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

export const formatDuration = (seconds: number) => {
  const mins = Math.floor(seconds / 60)
  const secs = Math.floor(seconds % 60)
  return `${mins}:${secs.toString().padStart(2, '0')}`
}
