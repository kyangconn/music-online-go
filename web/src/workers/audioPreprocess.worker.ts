import { parseBlob } from "music-metadata";
import type { AudioPreprocessRequest, AudioPreprocessResponse } from "@/types/audioWorker";
import { extractMetaFields } from "@/utils/upload";

const toHex = (buffer: ArrayBuffer) =>
  Array.from(new Uint8Array(buffer), (byte) => byte.toString(16).padStart(2, "0")).join("");

const hashFile = async (file: File) => {
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return toHex(digest);
};

self.onmessage = async (event: MessageEvent<AudioPreprocessRequest>) => {
  const { id, file, parseMetadata } = event.data;

  try {
    const parsed = parseMetadata ? await parseBlob(file) : null;
    const response: AudioPreprocessResponse = {
      id,
      ok: true,
      hash: await hashFile(file),
      metadata: parsed ? extractMetaFields(parsed.common, parsed.format) : null,
    };
    self.postMessage(response);
  } catch (error) {
    const response: AudioPreprocessResponse = {
      id,
      ok: false,
      error: error instanceof Error ? error.message : "Audio preprocessing failed",
    };
    self.postMessage(response);
  }
};
