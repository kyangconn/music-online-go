import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Music } from "@/types/api";
import { fetchMusicCollection, formatDuration } from "./library";

const requestMock = vi.hoisted(() => ({ get: vi.fn() }));
vi.mock("@/utils/request", () => ({ default: requestMock }));

const music = (id: number) => ({ id }) as Music;

describe("library utilities", () => {
  beforeEach(() => requestMock.get.mockReset());

  it("loads every server page in order", async () => {
    requestMock.get
      .mockResolvedValueOnce({ data: { items: [music(1), music(2)], total: 3 } })
      .mockResolvedValueOnce({ data: { items: [music(3)], total: 3 } });

    await expect(fetchMusicCollection({ album_key: "text_key" })).resolves.toEqual([music(1), music(2), music(3)]);

    expect(requestMock.get).toHaveBeenNthCalledWith(1, "/musics", {
      params: { album_key: "text_key", page: 1, page_size: 100 },
    });
    expect(requestMock.get).toHaveBeenNthCalledWith(2, "/musics", {
      params: { album_key: "text_key", page: 2, page_size: 100 },
    });
  });

  it("formats unknown, minute and hour durations", () => {
    expect(formatDuration(0)).toBe("—");
    expect(formatDuration(65)).toBe("1:05");
    expect(formatDuration(3661)).toBe("1:01:01");
  });
});
