import { createPinia, setActivePinia } from "pinia";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { UserInfo } from "@/types/api";
import { useUserStore } from "../user";

const requestMock = vi.hoisted(() => ({
  delete: vi.fn(),
  post: vi.fn(),
  put: vi.fn(),
}));

vi.mock("@/utils/request", () => ({ default: requestMock }));

const makeUser = (overrides: Partial<UserInfo> = {}): UserInfo => ({
  id: 1,
  username: "testuser",
  email: "test@example.com",
  full_name: "Test User",
  nickname: "",
  bio: "",
  role: "user",
  is_active: true,
  totp_enabled: false,
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
  ...overrides,
});

describe("useUserStore", () => {
  beforeEach(() => {
    localStorage.clear();
    requestMock.delete.mockReset();
    requestMock.post.mockReset();
    requestMock.put.mockReset();
    setActivePinia(createPinia());
  });

  it("recovers from corrupted cached user data", () => {
    localStorage.setItem("user", "{not valid json");
    setActivePinia(createPinia());

    const store = useUserStore();

    expect(store.user).toBeNull();
    expect(localStorage.getItem("user")).toBeNull();
  });

  it("restores the token and user from localStorage", () => {
    const user = makeUser({ username: "cached-user" });
    localStorage.setItem("token", "pre-existing-token");
    localStorage.setItem("user", JSON.stringify(user));
    setActivePinia(createPinia());

    const store = useUserStore();

    expect(store.token).toBe("pre-existing-token");
    expect(store.user?.username).toBe("cached-user");
    expect(store.isLoggedIn).toBe(true);
  });

  it("persists token and user updates", () => {
    const store = useUserStore();
    const user = makeUser();

    store.setToken("my-jwt-token");
    store.setUser(user);

    expect(store.token).toBe("my-jwt-token");
    expect(localStorage.getItem("token")).toBe("my-jwt-token");
    expect(store.user).toEqual(user);
    expect(JSON.parse(localStorage.getItem("user") || "null")).toEqual(user);
  });

  it("derives the administrator state from the current user", () => {
    const store = useUserStore();

    store.setUser(makeUser({ role: "admin" }));
    expect(store.isAdmin).toBe(true);

    store.setUser(makeUser({ role: "user" }));
    expect(store.isAdmin).toBe(false);
  });

  it("clears memory and persistent authentication on logout", () => {
    const store = useUserStore();
    store.setToken("some-token");
    store.setUser(makeUser());

    store.logout();

    expect(store.token).toBe("");
    expect(store.user).toBeNull();
    expect(store.isLoggedIn).toBe(false);
    expect(localStorage.getItem("token")).toBeNull();
    expect(localStorage.getItem("user")).toBeNull();
  });

  it("updates and persists the profile returned by the API", async () => {
    const store = useUserStore();
    const updated = makeUser({ nickname: "New name" });
    requestMock.put.mockResolvedValue({ code: 200, data: updated, message: "success" });

    await expect(store.updateUser({ nickname: "New name" })).resolves.toEqual(updated);

    expect(requestMock.put).toHaveBeenCalledWith("/users/profile", { nickname: "New name" });
    expect(store.user).toEqual(updated);
    expect(JSON.parse(localStorage.getItem("user") || "null")).toEqual(updated);
  });

  it("preserves profile state and rethrows when an update fails", async () => {
    const store = useUserStore();
    const original = makeUser();
    const error = new Error("network down");
    store.setUser(original);
    requestMock.put.mockRejectedValue(error);
    vi.spyOn(console, "error").mockImplementation(() => undefined);

    await expect(store.updateUser({ nickname: "Lost update" })).rejects.toBe(error);

    expect(store.user).toEqual(original);
  });

  it("sends the current and replacement password", async () => {
    const store = useUserStore();
    requestMock.post.mockResolvedValue({ code: 200, data: {}, message: "success" });

    await store.changePassword("old-password", "new-password");

    expect(requestMock.post).toHaveBeenCalledWith("/users/change-password", {
      new_password: "new-password",
      old_password: "old-password",
    });
  });

  it("logs out only after account deletion succeeds", async () => {
    const store = useUserStore();
    store.setToken("delete-token");
    store.setUser(makeUser());
    requestMock.delete.mockResolvedValue({ code: 200, data: {}, message: "success" });

    await store.deleteAccount("current-password");

    expect(requestMock.delete).toHaveBeenCalledWith("/users/profile", { data: { password: "current-password" } });
    expect(store.isLoggedIn).toBe(false);
    expect(store.user).toBeNull();
  });

  it("keeps the session when account deletion is rejected", async () => {
    const store = useUserStore();
    const error = new Error("incorrect password");
    store.setToken("keep-token");
    store.setUser(makeUser());
    requestMock.delete.mockRejectedValue(error);

    await expect(store.deleteAccount("wrong-password")).rejects.toBe(error);

    expect(store.token).toBe("keep-token");
    expect(store.user).not.toBeNull();
  });

  it("returns TOTP setup data without storing the secret", async () => {
    const store = useUserStore();
    const setup = { qr_code_url: "otpauth://totp/test", secret: "SECRET" };
    requestMock.post.mockResolvedValue({ code: 200, data: setup, message: "success" });

    await expect(store.setupTOTP()).resolves.toEqual(setup);

    expect(requestMock.post).toHaveBeenCalledWith("/users/totp/setup");
    expect(localStorage.getItem("SECRET")).toBeNull();
  });

  it("persists the enabled TOTP state only after a successful request", async () => {
    const store = useUserStore();
    store.setUser(makeUser());
    requestMock.post.mockResolvedValue({ code: 200, data: {}, message: "success" });

    await store.enableTOTP("123456");

    expect(requestMock.post).toHaveBeenCalledWith("/users/totp/enable", { code: "123456" });
    expect(store.user?.totp_enabled).toBe(true);
    expect(JSON.parse(localStorage.getItem("user") || "null").totp_enabled).toBe(true);
  });

  it("does not change the TOTP state when enabling fails", async () => {
    const store = useUserStore();
    const error = new Error("invalid code");
    store.setUser(makeUser());
    requestMock.post.mockRejectedValue(error);

    await expect(store.enableTOTP("000000")).rejects.toBe(error);

    expect(store.user?.totp_enabled).toBe(false);
  });

  it("persists the disabled TOTP state after verification", async () => {
    const store = useUserStore();
    store.setUser(makeUser({ totp_enabled: true }));
    requestMock.post.mockResolvedValue({ code: 200, data: {}, message: "success" });

    await store.disableTOTP("654321");

    expect(requestMock.post).toHaveBeenCalledWith("/users/totp/disable", { code: "654321" });
    expect(store.user?.totp_enabled).toBe(false);
  });
});
