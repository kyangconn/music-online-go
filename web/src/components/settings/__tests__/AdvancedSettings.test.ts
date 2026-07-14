import { flushPromises, mount } from "@vue/test-utils";
import { defineComponent } from "vue";
import { beforeEach, describe, expect, it, vi } from "vitest";
import AdvancedSettings from "../AdvancedSettings.vue";

const messageMock = vi.hoisted(() => ({ info: vi.fn(), success: vi.fn() }));
const messageBoxMock = vi.hoisted(() => ({ prompt: vi.fn() }));
const userStoreMock = vi.hoisted(() => ({ deleteAccount: vi.fn() }));
const routerMock = vi.hoisted(() => ({ replace: vi.fn() }));
const apiErrorMock = vi.hoisted(() => ({ handleError: vi.fn() }));

vi.mock("element-plus", () => ({ ElMessage: messageMock, ElMessageBox: messageBoxMock }));
vi.mock("vue-i18n", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("vue-router", () => ({ useRouter: () => routerMock }));
vi.mock("@/store/user", () => ({ useUserStore: () => userStoreMock }));
vi.mock("@/composables/useApiError", () => ({ useApiError: () => apiErrorMock }));

const ButtonStub = defineComponent({
  emits: ["click"],
  template: '<button type="button" @click="$emit(\'click\')"><slot /></button>',
});

const mountSettings = () =>
  mount(AdvancedSettings, {
    global: {
      mocks: { $t: (key: string) => key },
      stubs: { ElButton: ButtonStub },
    },
  });

describe("AdvancedSettings account deletion", () => {
  beforeEach(() => {
    messageBoxMock.prompt.mockResolvedValue({ value: "current-password" });
    userStoreMock.deleteAccount.mockResolvedValue(undefined);
    routerMock.replace.mockResolvedValue(undefined);
  });

  it("deletes the account only after password confirmation", async () => {
    const wrapper = mountSettings();
    const deleteButton = wrapper.findAll("button").find((button) => button.text() === "settings.delete_account");
    if (!deleteButton) throw new Error("delete account button not found");

    await deleteButton.trigger("click");
    await flushPromises();

    expect(messageBoxMock.prompt).toHaveBeenCalledWith(
      "settings.delete_account_warning",
      "settings.delete_account",
      expect.objectContaining({ inputType: "password", type: "warning" }),
    );
    expect(userStoreMock.deleteAccount).toHaveBeenCalledWith("current-password");
    expect(messageMock.success).toHaveBeenCalledWith("settings.delete_account_success");
    expect(routerMock.replace).toHaveBeenCalledWith("/login");
  });

  it("does nothing when confirmation is cancelled", async () => {
    messageBoxMock.prompt.mockRejectedValue("cancel");
    const wrapper = mountSettings();
    const deleteButton = wrapper.findAll("button").find((button) => button.text() === "settings.delete_account");
    if (!deleteButton) throw new Error("delete account button not found");

    await deleteButton.trigger("click");
    await flushPromises();

    expect(userStoreMock.deleteAccount).not.toHaveBeenCalled();
    expect(apiErrorMock.handleError).not.toHaveBeenCalled();
  });

  it("keeps the user on the page and reports a failed deletion", async () => {
    const error = new Error("incorrect password");
    userStoreMock.deleteAccount.mockRejectedValue(error);
    const wrapper = mountSettings();
    const deleteButton = wrapper.findAll("button").find((button) => button.text() === "settings.delete_account");
    if (!deleteButton) throw new Error("delete account button not found");

    await deleteButton.trigger("click");
    await flushPromises();

    expect(apiErrorMock.handleError).toHaveBeenCalledWith(error, "settings.delete_account_failed");
    expect(routerMock.replace).not.toHaveBeenCalled();
  });
});
