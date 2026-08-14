import { defineStore } from "pinia";
import { ref, computed } from "vue";
import type { UserInfo, UpdateUserProfileData, TOTPSetupData, RefreshData } from "@/types/api";
import request, { setAccessToken, clearAccessToken } from "@/utils/request";

/**
 * 用户状态管理存储
 *
 * 认证模型：短期 access token 只保存在内存（刷新页面后通过 /users/refresh
 * 的 httpOnly cookie 静默恢复），refresh token 由服务端 cookie 持有，
 * JavaScript 永远无法读取。用户信息属于非敏感展示数据，保留在 localStorage。
 */
export const useUserStore = defineStore("user", () => {
  /** 短期访问令牌（内存态，不落 localStorage） */
  const token = ref("");

  /** 用户信息对象 */
  let storedUser: UserInfo | null = null;
  try {
    const raw = localStorage.getItem("user");
    if (raw) storedUser = JSON.parse(raw);
  } catch {
    localStorage.removeItem("user"); // 清理损坏的数据
  }
  const user = ref<UserInfo | null>(storedUser);

  /** 用户是否已登录 */
  const isLoggedIn = computed(() => !!token.value);
  /** 用户是否为管理员 */
  const isAdmin = computed(() => user.value?.role === "admin");

  /**
   * 设置短期访问令牌（仅内存），并同步给 axios 请求层
   * @param newToken - 新的认证令牌
   */
  function setToken(newToken: string) {
    token.value = newToken;
    setAccessToken(newToken);
  }

  /**
   * 设置用户信息（localStorage 保存非敏感展示数据）
   * @param newUser - 新的用户信息对象
   */
  function setUser(newUser: UserInfo | null) {
    user.value = newUser;
    try {
      localStorage.setItem("user", JSON.stringify(newUser));
    } catch (error) {
      console.error("保存用户信息失败:", error);
      // 即使存储失败，也更新内存中的状态
    }
  }

  /**
   * 用 refresh cookie 恢复会话：刷新页面后 access token 丢失，路由守卫
   * 在进入受保护页面前调用本方法。失败（会话过期/撤销）返回 false。
   */
  async function refreshSession(): Promise<boolean> {
    try {
      const res = await request.post<RefreshData>("/users/refresh");
      setToken(res.data.access_token);
      return true;
    } catch {
      clearLocalSession();
      return false;
    }
  }

  /**
   * 更新用户信息
   * @param data - 要更新的用户数据
   * @returns 更新后的用户信息
   * @throws 当API请求失败时抛出错误
   */
  async function updateUser(data: UpdateUserProfileData) {
    try {
      const response = await request.put<UserInfo>("/users/profile", data);
      setUser(response.data);
      return response.data;
    } catch (error) {
      console.error("更新用户信息失败:", error);
      throw error;
    }
  }

  /**
   * 修改当前用户的登录密码
   * @param oldPassword - 当前旧密码
   * @param newPassword - 想要设置的新密码
   */
  async function changePassword(oldPassword: string, newPassword: string) {
    try {
      await request.post("/users/change-password", {
        old_password: oldPassword,
        new_password: newPassword,
      });
    } catch (error) {
      console.error("修改密码失败:", error);
      throw error;
    }
  }

  async function deleteAccount(currentPassword: string) {
    await request.delete("/users/profile", { data: { password: currentPassword } });
    await logout();
  }

  /**
   * 向服务端请求生成 TOTP 两步验证的密钥和二维码
   * @returns 包含 TOTP secret 和二维码 URL 的数据
   */
  async function setupTOTP() {
    const res = await request.post<TOTPSetupData>("/users/totp/setup");
    return res.data;
  }

  /**
   * 使用用户提供的验证码启用 TOTP 两步验证
   * @param code - 用户从验证器应用获取的 6 位验证码
   */
  async function enableTOTP(code: string) {
    await request.post("/users/totp/enable", { code });
    if (user.value) user.value.totp_enabled = true;
    setUser(user.value);
  }

  /**
   * 使用用户提供的验证码禁用 TOTP 两步验证
   * @param code - 用户从验证器应用获取的 6 位验证码
   */
  async function disableTOTP(code: string) {
    await request.post("/users/totp/disable", { code });
    if (user.value) user.value.totp_enabled = false;
    setUser(user.value);
  }

  /** 清除本地内存与 localStorage 中的会话状态 */
  function clearLocalSession() {
    token.value = "";
    user.value = null;
    clearAccessToken();
    try {
      localStorage.removeItem("user");
    } catch (error) {
      console.error("清除本地存储失败:", error);
    }
  }

  /**
   * 用户登出：先撤销服务端当前会话，再清除本地状态。
   * 请求失败（断网等）时仍然清除本地状态。
   */
  async function logout() {
    try {
      await request.post("/users/logout", undefined, { skipAuthRefresh: true });
    } catch (error) {
      console.warn("服务端登出失败（本地会话仍会被清除）:", error);
    } finally {
      clearLocalSession();
    }
  }

  return {
    token,
    user,
    isLoggedIn,
    isAdmin,
    setToken,
    setUser,
    refreshSession,
    updateUser,
    changePassword,
    deleteAccount,
    setupTOTP,
    enableTOTP,
    disableTOTP,
    logout,
  };
});
