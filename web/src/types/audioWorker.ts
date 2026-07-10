import type { MusicMetadataFields } from "@/types/api";

export interface AudioPreprocessRequest {
  id: number;
  file: File;
  parseMetadata: boolean;
}

export interface AudioPreprocessSuccess {
  id: number;
  ok: true;
  hash: string;
  metadata: MusicMetadataFields | null;
}

export interface AudioPreprocessFailure {
  id: number;
  ok: false;
  error: string;
}

export type AudioPreprocessResponse = AudioPreprocessSuccess | AudioPreprocessFailure;

export interface AudioPreprocessResult {
  hash: string;
  metadata: MusicMetadataFields;
}
