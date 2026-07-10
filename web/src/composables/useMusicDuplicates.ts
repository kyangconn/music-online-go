import { ref } from "vue";
import type {
  MusicDuplicateCheckData,
  MusicMetadataFields,
} from "@/types/api";
import request from "@/utils/request";
import { metadataToData } from "@/utils/upload";

export function useMusicDuplicates() {
  const checking = ref(false);

  const checkDuplicate = async (metadata: MusicMetadataFields, fileHash = "") => {
    checking.value = true;
    try {
      const payload = { ...metadataToData(metadata), file_hash: fileHash };
      const response = await request.post<MusicDuplicateCheckData>("/musics/duplicate-check", payload);
      return response.data;
    } finally {
      checking.value = false;
    }
  };

  const enrichExactMatch = async (result: MusicDuplicateCheckData) => {
    if (!result.exact_match || !result.enrichment) return false;
    await request.put(`/musics/${result.exact_match.id}`, result.enrichment);
    return true;
  };

  return { checking, checkDuplicate, enrichExactMatch };
}
