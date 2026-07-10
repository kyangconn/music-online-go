import type { Ref } from "vue";
import { watch } from "vue";
import type { MusicMetadataFields } from "@/types/api";

const DRAFT_STORAGE_KEY = "music-upload-draft:v1";

interface UploadDraft extends MusicMetadataFields {
  description: string;
}

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === "object" && value !== null;

export function useUploadDraft(
  form: MusicMetadataFields,
  touched: Record<keyof MusicMetadataFields, boolean>,
  description: Ref<string>,
) {
  try {
    const raw = localStorage.getItem(DRAFT_STORAGE_KEY);
    const parsed: unknown = raw ? JSON.parse(raw) : null;
    if (isRecord(parsed)) {
      for (const key of Object.keys(form) as (keyof MusicMetadataFields)[]) {
        const value = parsed[key];
        if (typeof value !== "string") continue;
        form[key] = value;
        touched[key] = value.length > 0;
      }
      if (typeof parsed.description === "string") description.value = parsed.description;
    }
  } catch {
    // A damaged draft should not block the upload form.
  }

  watch(
    [() => ({ ...form }), description],
    ([metadata, intro]) => {
      const draft: UploadDraft = { ...metadata, description: intro };
      try {
        localStorage.setItem(DRAFT_STORAGE_KEY, JSON.stringify(draft));
      } catch {
        // The form remains usable when browser storage is unavailable.
      }
    },
    { deep: true },
  );

  const clearDraft = () => {
    try {
      localStorage.removeItem(DRAFT_STORAGE_KEY);
    } catch {
      // Ignore storage failures after a successful upload.
    }
  };

  return { clearDraft };
}
