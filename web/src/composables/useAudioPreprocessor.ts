import { onScopeDispose } from "vue";
import type { AudioPreprocessResponse, AudioPreprocessResult } from "@/types/audioWorker";
import {
  loadCachedMeta,
  parseAudioFile,
  saveCachedMeta,
} from "@/utils/upload";

interface PendingRequest {
  resolve: (result: AudioPreprocessResult) => void;
  reject: (error: Error) => void;
  cachedMetadata: AudioPreprocessResult["metadata"] | null;
}

const toHex = (buffer: ArrayBuffer) =>
  Array.from(new Uint8Array(buffer), (byte) => byte.toString(16).padStart(2, "0")).join("");

const preprocessWithoutWorker = async (file: File): Promise<AudioPreprocessResult> => {
  const metadata = loadCachedMeta(file) ?? (await parseAudioFile(file));
  saveCachedMeta(file, metadata);
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return { metadata, hash: toHex(digest) };
};

export function useAudioPreprocessor() {
  const pending = new Map<number, PendingRequest>();
  let worker: Worker | null = null;
  let sequence = 0;
  let workerUnavailable = false;

  const rejectPending = (message: string) => {
    for (const request of pending.values()) request.reject(new Error(message));
    pending.clear();
  };

  const getWorker = () => {
    if (workerUnavailable) return null;
    if (worker) return worker;

    try {
      worker = new Worker(new URL("../workers/audioPreprocess.worker.ts", import.meta.url), { type: "module" });
      worker.onmessage = (event: MessageEvent<AudioPreprocessResponse>) => {
        const response = event.data;
        const request = pending.get(response.id);
        if (!request) return;
        pending.delete(response.id);

        if (!response.ok) {
          request.reject(new Error(response.error));
          return;
        }

        const metadata = response.metadata ?? request.cachedMetadata;
        if (!metadata) {
          request.reject(new Error("Worker returned no metadata"));
          return;
        }
        request.resolve({ metadata, hash: response.hash });
      };
      worker.onerror = () => {
        workerUnavailable = true;
        worker?.terminate();
        worker = null;
        rejectPending("Audio preprocessing worker stopped unexpectedly");
      };
      return worker;
    } catch {
      workerUnavailable = true;
      return null;
    }
  };

  const preprocess = (file: File): Promise<AudioPreprocessResult> => {
    const activeWorker = getWorker();
    if (!activeWorker) return preprocessWithoutWorker(file);

    const cachedMetadata = loadCachedMeta(file);
    const id = ++sequence;
    return new Promise<AudioPreprocessResult>((resolve, reject) => {
      pending.set(id, { resolve, reject, cachedMetadata });
      try {
        activeWorker.postMessage({ id, file, parseMetadata: !cachedMetadata });
      } catch (error) {
        pending.delete(id);
        reject(error instanceof Error ? error : new Error("Unable to start audio preprocessing"));
      }
    }).then((result) => {
      saveCachedMeta(file, result.metadata);
      return result;
    });
  };

  onScopeDispose(() => {
    worker?.terminate();
    rejectPending("Audio preprocessing was cancelled");
  });

  return { preprocess };
}
