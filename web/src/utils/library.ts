import type { Music, PaginatedData } from "@/types/api";
import request from "@/utils/request";

const COLLECTION_PAGE_SIZE = 100;
const MAX_COLLECTION_TRACKS = 500;

// Fetches a complete album-sized collection through the ordinary paginated
// endpoint, preserving server ordering and its access/media URL policies.
export const fetchMusicCollection = async (params: Record<string, unknown>) => {
  const tracks: Music[] = [];
  let page = 1;
  let total = 0;
  do {
    const response = await request.get<PaginatedData<Music>>("/musics", {
      params: { ...params, page, page_size: COLLECTION_PAGE_SIZE },
    });
    const items = response.data.items || [];
    tracks.push(...items.slice(0, MAX_COLLECTION_TRACKS - tracks.length));
    total = response.data.total ?? tracks.length;
    if (items.length === 0) break;
    page += 1;
  } while (tracks.length < total && tracks.length < MAX_COLLECTION_TRACKS);
  return tracks;
};

export const formatDuration = (seconds: number) => {
  if (!Number.isFinite(seconds) || seconds <= 0) return "—";
  const rounded = Math.round(seconds);
  const hours = Math.floor(rounded / 3600);
  const minutes = Math.floor((rounded % 3600) / 60);
  const remaining = rounded % 60;
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, "0")}:${String(remaining).padStart(2, "0")}`
    : `${minutes}:${String(remaining).padStart(2, "0")}`;
};
