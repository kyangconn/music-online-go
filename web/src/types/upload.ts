export type BatchUploadStatus = "pending" | "uploading" | "success" | "failed" | "skipped";
export type BatchUploadStage = "preflight" | "upload";

export interface BatchUploadResultItem {
  path: string;
  name: string;
  status: BatchUploadStatus;
  stage: BatchUploadStage;
  reason: string;
  musicId?: number;
  attempts: number;
}
