import { defineStore } from "pinia";
import { ref, computed } from "vue";
import type { UserInfo, UpdateUserProfileData, TOTPSetupData } from "@/types/api";
import request from "@/utils/request";

/**
 * 用户状态管理存储
 * 管理用户认证状态、用户信息和相关操作
 */
export const useUserStore = defineStore("user", () => {
  /** 用户认证令牌 */
  const token = ref(localStorage.getItem("token") || "");
  /** 用户信息对象 */
  const user = ref<UserInfo | null>(JSON.parse(localStorage.getItem("user") || "null"));

  /** 用户是否已登录 */
  const isLoggedIn = computed(() => !!token.value);
  /** 用户是否为管理员 */
  const isAdmin = computed(() => user.value?.role === "admin");

  /**
   * 设置用户认证令牌
   * @param newToken - 新的认证令牌
   */
  function setToken(newToken: string) {
    token.value = newToken;
    try {
      localStorage.setItem("token", newToken);
    } catch (error) {
      console.error("保存令牌失败:", error);
      // 即使存储失败，也更新内存中的状态
    }
  }

  /**
   * 设置用户信息
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

  /**
   * 用户登出
   * 清除用户认证状态和本地存储的用户信息
   */
  function logout() {
    token.value = "";
    user.value = null;
    try {
      localStorage.removeItem("token");
      localStorage.removeItem("user");
    } catch (error) {
      console.error("清除本地存储失败:", error);
      // 即使清除存储失败，也清除内存中的状态
    }
  }

  return {
    token,
    user,
    isLoggedIn,
    isAdmin,
    setToken,
    setUser,
    updateUser,
    changePassword,
    setupTOTP,
    enableTOTP,
    disableTOTP,
    logout,
  };
});
