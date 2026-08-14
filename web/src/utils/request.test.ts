import type { AxiosError, AxiosRequestConfig, InternalAxiosRequestConfig } from "axios";
import { afterAll, beforeEach, describe, expect, it, vi } from "vitest";

/** 模块级 mock：axios.create 返回可调用的 mockService，拦截器注册被捕获 */
const mocks = vi.hoisted(() => {
  type MockFn = ReturnType<typeof vi.fn>;
  type MockService = MockFn & {
    get: MockFn;
    post: MockFn;
    put: MockFn;
    patch: MockFn;
    delete: MockFn;
    interceptors: {
      request: { use: MockFn };
      response: { use: MockFn };
    };
    defaults: Record<string, unknown>;
  };
  const service = vi.fn() as unknown as MockService;
  service.get = vi.fn();
  service.post = vi.fn();
  service.put = vi.fn();
  service.patch = vi.fn();
  service.delete = vi.fn();
  service.interceptors = {
    request: { use: vi.fn() },
    response: { use: vi.fn() },
  };
  service.defaults = {};
  return {
    service,
    messageWarning: vi.fn(),
    requestUse: service.interceptors.request.use,
    responseUse: service.interceptors.response.use,
  };
});

vi.mock("axios", () => ({
  default: { create: vi.fn(() => mocks.service) },
}));

vi.mock("element-plus", () => ({ ElMessage: { warning: mocks.messageWarning } }));
vi.mock("@/i18n", () => ({ default: { global: { t: (key: string) => key } } }));

import request, { clearAccessToken, setAccessToken } from "./request";

const requestInterceptor = mocks.requestUse.mock.calls[0][0];
const responseOnFulfilled = mocks.responseUse.mock.calls[0][0];
const responseErrorHandler = mocks.responseUse.mock.calls[0][1];

/** 构造一个 401 风格的 Axios 错误 */
function makeError(status: number, url: string, errorMessage?: string, extra?: Partial<AxiosRequestConfig>): AxiosError {
  return {
    isAxiosError: true,
    name: "AxiosError",
    message: `Request failed with status code ${status}`,
    response: {
      status,
      data: { code: status, message: errorMessage ?? "", error: errorMessage },
      statusText: "",
      headers: {},
      config: { url } as AxiosRequestConfig,
    },
    config: { url, ...extra } as AxiosRequestConfig,
    toJSON: () => ({}),
  } as unknown as AxiosError;
}

/** refresh 成功响应（拦截器解包后的形状：ApiResponse<RefreshData>） */
const refreshSuccess = {
  code: 200,
  message: "success",
  data: { access_token: "refreshed-token", expires_in: 900 },
};

const originalLocation = window.location;

beforeEach(() => {
  vi.clearAllMocks();
  clearAccessToken();
  // jsdom 不支持真实导航：用可写对象替换 location
  Object.defineProperty(window, "location", {
    value: { pathname: "/music/1", search: "", hash: "", href: "" },
    writable: true,
    configurable: true,
  });
});

afterAll(() => {
  Object.defineProperty(window, "location", { value: originalLocation, configurable: true });
});

describe("request interceptor", () => {
  it("attaches the in-memory access token as a Bearer header", () => {
    setAccessToken("mem-token");
    const config = { headers: {} } as InternalAxiosRequestConfig;

    const result = requestInterceptor(config);

    expect(result.headers["Authorization"]).toBe("Bearer mem-token");
  });

  it("omits the Authorization header when no token is present", () => {
    clearAccessToken();
    const config = { headers: {} } as InternalAxiosRequestConfig;

    requestInterceptor(config);

    expect(config.headers["Authorization"]).toBeUndefined();
  });
});

describe("response interceptor", () => {
  it("unwraps the API response body", () => {
    const response = { data: { code: 200, data: { id: 1 }, message: "success" } };

    expect(responseOnFulfilled(response)).toEqual({ code: 200, data: { id: 1 }, message: "success" });
  });

  it("refreshes once, replays the original request with the new token", async () => {
    setAccessToken("old-token");
    mocks.service.post.mockResolvedValueOnce(refreshSuccess);
    mocks.service.mockResolvedValueOnce({ code: 200, data: { items: [] }, message: "success" });
    const error = makeError(401, "/musics", "Token has expired", { method: "get", headers: {} });

    const result = await responseErrorHandler(error);

    expect(mocks.service.post).toHaveBeenCalledWith("/users/refresh", undefined, { skipAuthRefresh: true });
    expect(mocks.service).toHaveBeenCalledTimes(1);
    const replayConfig = mocks.service.mock.calls[0][0];
    expect(replayConfig.url).toBe("/musics");
    expect(replayConfig.skipAuthRefresh).toBe(true);
    expect(replayConfig.headers["Authorization"]).toBe("Bearer refreshed-token");
    expect(result).toEqual({ code: 200, data: { items: [] }, message: "success" });
    // 成功刷新后仍可继续使用新 token
    const next = requestInterceptor({ headers: {} } as InternalAxiosRequestConfig);
    expect(next.headers["Authorization"]).toBe("Bearer refreshed-token");
  });

  it("deduplicates concurrent 401s into a single refresh", async () => {
    setAccessToken("old-token");
    let resolveRefresh!: (value: unknown) => void;
    mocks.service.post.mockReturnValueOnce(
      new Promise((resolve) => {
        resolveRefresh = resolve;
      }),
    );
    mocks.service.mockResolvedValue({ code: 200, data: {}, message: "success" });

    const first = responseErrorHandler(makeError(401, "/playlists", "Token has expired", { headers: {} }));
    const second = responseErrorHandler(makeError(401, "/profile", "Token has expired", { headers: {} }));
    // 两个 401 都进入刷新流程后才放行 refresh 响应
    await Promise.resolve();
    await Promise.resolve();

    expect(mocks.service.post).toHaveBeenCalledTimes(1);

    resolveRefresh(refreshSuccess);
    await first;
    await second;
    expect(mocks.service).toHaveBeenCalledTimes(2); // 两个原请求都被重放
  });

  it("retries once when the refresh raced with a concurrent rotation", async () => {
    setAccessToken("old-token");
    mocks.service.post.mockRejectedValueOnce(
      makeError(401, "/users/refresh", "Session refreshed concurrently, please retry"),
    );
    mocks.service.post.mockResolvedValueOnce(refreshSuccess);
    mocks.service.mockResolvedValueOnce({ code: 200, data: {}, message: "success" });

    const result = await responseErrorHandler(makeError(401, "/musics", "Token has expired", { headers: {} }));

    expect(mocks.service.post).toHaveBeenCalledTimes(2);
    expect(result).toEqual({ code: 200, data: {}, message: "success" });
  });

  it("clears the token and redirects to login when refresh is rejected", async () => {
    setAccessToken("old-token");
    mocks.service.post.mockRejectedValueOnce(makeError(401, "/users/refresh", "Session has been revoked"));

    await expect(
      responseErrorHandler(makeError(401, "/musics", "Token has expired", { headers: {} })),
    ).rejects.toBeDefined();

    expect(mocks.messageWarning).toHaveBeenCalledWith("common.session_expired");
    expect(window.location.href).toBe("/login?redirect=%2Fmusic%2F1");
    // token 已清除：下一个请求不再携带 Authorization
    const config = { headers: {} } as InternalAxiosRequestConfig;
    requestInterceptor(config);
    expect(config.headers["Authorization"]).toBeUndefined();
  });

  it("does not refresh for login/refresh endpoints themselves", async () => {
    for (const url of ["/users/login", "/users/refresh"]) {
      await expect(responseErrorHandler(makeError(401, url, "Bad credentials"))).rejects.toBeDefined();
    }
    expect(mocks.service.post).not.toHaveBeenCalled();
  });

  it("leaves non-401 errors untouched", async () => {
    const error = makeError(500, "/musics", "boom");
    await expect(responseErrorHandler(error)).rejects.toBe(error);
    expect(mocks.service.post).not.toHaveBeenCalled();
  });
});

describe("request helpers", () => {
  it("exposes the typed HTTP methods bound to the axios instance", () => {
    mocks.service.get.mockResolvedValueOnce({ code: 200, data: {}, message: "success" });
    mocks.service.delete.mockResolvedValueOnce({ code: 200, data: {}, message: "success" });

    void request.get("/musics", { params: { page: 1 } });
    void request.delete("/musics/1");

    expect(mocks.service.get).toHaveBeenCalledWith("/musics", { params: { page: 1 } });
    expect(mocks.service.delete).toHaveBeenCalledWith("/musics/1", undefined);
  });
});
