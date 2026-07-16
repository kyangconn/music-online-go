import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useMusicUpload } from "../useMusicUpload";

const requestMock = vi.hoisted(() => ({ post: vi.fn() }));
const messageMock = vi.hoisted(() => ({ error: vi.fn() }));
const uploadPolicyMock = vi.hoisted(() => ({
  loadPolicy: vi.fn().mockResolvedValue(undefined),
  policy: {
    value: {
      audio_extensions: [".mp3", ".flac"],
      cover_extensions: [".jpg", ".png"],
      max_audio_size_bytes: 1024,
      max_audio_size_mb: 1,
      max_cover_size_bytes: 1024,
      max_cover_size_mb: 1,
    },
  },
}));

vi.mock("@/utils/request", () => ({ default: requestMock }));
vi.mock("element-plus", () => ({ ElMessage: messageMock }));
vi.mock("vue-i18n", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/composables/useUploadPolicy", () => ({ useUploadPolicy: () => uploadPolicyMock }));
vi.mock("@/composables/useApiError", () => ({
  useApiError: () => ({
    getErrorMessage: (error: unknown, fallback: string) => (error instanceof Error ? error.message : fallback),
  }),
}));

const makeAudio = () => new File([new Uint8Array([1, 2, 3])], "track.mp3", { type: "audio/mpeg" });

describe("useMusicUpload", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    requestMock.post.mockReset();
    uploadPolicyMock.loadPolicy.mockReset().mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("rejects an invalid audio file before creating a music record", async () => {
    const { loading, uploadOne } = useMusicUpload();
    const invalid = new File(["not audio"], "notes.txt", { type: "text/plain" });

    const result = await uploadOne({ artist: "Artist", audio: invalid, title: "Title" });

    expect(result).toMatchObject({ success: false, uploaded: false });
    expect(result.errorMessage).toBe("add.invalid_file_extension");
    expect(requestMock.post).not.toHaveBeenCalled();
    expect(messageMock.error).toHaveBeenCalledWith("add.invalid_file_extension");
    expect(loading.value).toBe(false);
  });

  it("creates a metadata-only record without starting file upload", async () => {
    requestMock.post.mockResolvedValue({ code: 200, data: { id: 17 }, message: "success" });
    const { uploadOne } = useMusicUpload();

    const result = await uploadOne({
      artist: "Artist",
      metadata: { album: "Album", artist: "Artist", duration: "3:20", genre: "Rock", title: "Title", track: "2", year: "2026" },
      title: "Title",
    });

    expect(result).toEqual({ musicId: 17, success: true, uploaded: false });
    expect(requestMock.post).toHaveBeenCalledWith(
      "/musics",
      expect.objectContaining({ album: "Album", duration: 200, track_number: 2, year: 2026 }),
    );
  });

  it("returns the created ID when file upload fails so retry can reuse it", async () => {
    requestMock.post
      .mockResolvedValueOnce({ code: 200, data: { id: 23 }, message: "success" })
      .mockRejectedValueOnce(new Error("upload interrupted"));
    const { uploadOne } = useMusicUpload();

    const result = await uploadOne({ artist: "Artist", audio: makeAudio(), silent: true, title: "Title" });

    expect(result).toEqual({
      errorMessage: "upload interrupted",
      musicId: 23,
      success: false,
      uploaded: false,
    });
    expect(messageMock.error).not.toHaveBeenCalled();
  });

  it("retries an existing record without creating a second record", async () => {
    requestMock.post.mockImplementation(async (_url, _body, config) => {
      config.onUploadProgress({ loaded: 1, total: 2 });
      return { code: 200, data: {}, message: "success" };
    });
    const { uploadOne, uploadPercent } = useMusicUpload();
    const audio = makeAudio();

    const result = await uploadOne({ artist: "Artist", audio, existingMusicId: 23, title: "Title" });

    expect(result).toEqual({ musicId: 23, success: true, uploaded: true });
    expect(requestMock.post).toHaveBeenCalledOnce();
    expect(requestMock.post.mock.calls[0]?.[0]).toBe("/musics/23/upload");
    expect(requestMock.post.mock.calls[0]?.[1]).toBeInstanceOf(FormData);
    expect((requestMock.post.mock.calls[0]?.[1] as FormData).get("file")).toBe(audio);
    expect(requestMock.post.mock.calls[0]?.[2]?.timeout).toBe(0);
    expect(uploadPercent.value).toBe(100);

    vi.advanceTimersByTime(800);
    expect(uploadPercent.value).toBe(0);
  });

  it("reports a missing ID returned by the create endpoint", async () => {
    requestMock.post.mockResolvedValue({ code: 200, data: {}, message: "success" });
    const { uploadOne } = useMusicUpload();

    const result = await uploadOne({ artist: "Artist", title: "Title" });

    expect(result).toEqual({ errorMessage: "add.create_record_failed", success: false, uploaded: false });
    expect(messageMock.error).toHaveBeenCalledWith("add.create_record_failed");
  });
});
