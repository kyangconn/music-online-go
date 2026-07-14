import { flushPromises, mount, type VueWrapper } from "@vue/test-utils";
import { defineComponent, reactive } from "vue";
import { beforeEach, describe, expect, it, vi } from "vitest";
import SecuritySettings from "../SecuritySettings.vue";

const messageMock = vi.hoisted(() => ({ error: vi.fn(), success: vi.fn(), warning: vi.fn() }));
const qrCodeMock = vi.hoisted(() => ({ toDataURL: vi.fn() }));
const storeControl = vi.hoisted(() => ({ current: undefined as unknown }));

vi.mock("element-plus", () => ({ ElMessage: messageMock }));
vi.mock("qrcode", () => ({ default: qrCodeMock }));
vi.mock("vue-i18n", () => ({ useI18n: () => ({ t: (key: string) => key }) }));
vi.mock("@/store/user", () => ({ useUserStore: () => storeControl.current }));

const ButtonStub = defineComponent({
  emits: ["click"],
  template: '<button type="button" @click="$emit(\'click\')"><slot /></button>',
});

const InputStub = defineComponent({
  emits: ["update:modelValue"],
  props: {
    modelValue: { default: "", type: String },
    placeholder: { default: "", type: String },
  },
  template:
    '<input :value="modelValue" :placeholder="placeholder" @input="$emit(\'update:modelValue\', $event.target.value)" />',
});

const DialogStub = defineComponent({
  emits: ["update:modelValue"],
  props: {
    modelValue: Boolean,
    title: { default: "", type: String },
  },
  template: '<section v-if="modelValue"><h4>{{ title }}</h4><slot /><slot name="footer" /></section>',
});

const TagStub = defineComponent({ template: "<span><slot /></span>" });

interface SecurityStoreStub {
  disableTOTP: ReturnType<typeof vi.fn>;
  enableTOTP: ReturnType<typeof vi.fn>;
  setupTOTP: ReturnType<typeof vi.fn>;
  user: { totp_enabled: boolean };
}

const makeStore = (enabled = false): SecurityStoreStub =>
  reactive({
    disableTOTP: vi.fn(),
    enableTOTP: vi.fn(),
    setupTOTP: vi.fn(),
    user: { totp_enabled: enabled },
  });

const mountSettings = () =>
  mount(SecuritySettings, {
    global: {
      mocks: { $t: (key: string) => key },
      stubs: {
        ElButton: ButtonStub,
        ElDialog: DialogStub,
        ElInput: InputStub,
        ElTag: TagStub,
      },
    },
  });

const buttonByText = (wrapper: VueWrapper, text: string) => {
  const button = wrapper.findAll("button").find((candidate) => candidate.text() === text);
  if (!button) throw new Error(`Button not found: ${text}`);
  return button;
};

describe("SecuritySettings", () => {
  beforeEach(() => {
    storeControl.current = makeStore();
    qrCodeMock.toDataURL.mockResolvedValue("data:image/png;base64,qr");
  });

  it("opens setup with the generated QR code and manual secret", async () => {
    const store = storeControl.current as SecurityStoreStub;
    store.setupTOTP.mockResolvedValue({ qr_code_url: "otpauth://totp/account", secret: "SECRET42" });
    const wrapper = mountSettings();

    await buttonByText(wrapper, "settings.totp_setup").trigger("click");
    await flushPromises();

    expect(store.setupTOTP).toHaveBeenCalledOnce();
    expect(qrCodeMock.toDataURL).toHaveBeenCalledWith("otpauth://totp/account");
    expect(wrapper.text()).toContain("SECRET42");
    expect(wrapper.get("img").attributes()).toMatchObject({
      alt: "settings.totp_qr_alt",
      src: "data:image/png;base64,qr",
    });
  });

  it("uses localized feedback when setup fails", async () => {
    const store = storeControl.current as SecurityStoreStub;
    store.setupTOTP.mockRejectedValue(new Error("offline"));
    const wrapper = mountSettings();

    await buttonByText(wrapper, "settings.totp_setup").trigger("click");
    await flushPromises();

    expect(messageMock.error).toHaveBeenCalledWith("settings.totp_setup_failed");
    expect(wrapper.find(".totp-setup-panel").exists()).toBe(false);
  });

  it("validates the code before enabling TOTP and updates the UI after success", async () => {
    const store = storeControl.current as SecurityStoreStub;
    store.setupTOTP.mockResolvedValue({ qr_code_url: "otpauth://totp/account", secret: "SECRET42" });
    store.enableTOTP.mockImplementation(async () => {
      store.user.totp_enabled = true;
    });
    const wrapper = mountSettings();
    await buttonByText(wrapper, "settings.totp_setup").trigger("click");
    await flushPromises();
    const input = wrapper.get('input[placeholder="settings.totp_code_placeholder"]');

    await input.setValue("123");
    await buttonByText(wrapper, "settings.totp_verify_enable").trigger("click");
    expect(messageMock.warning).toHaveBeenCalledWith("settings.totp_code_required");
    expect(store.enableTOTP).not.toHaveBeenCalled();

    await input.setValue("123456");
    await buttonByText(wrapper, "settings.totp_verify_enable").trigger("click");
    await flushPromises();

    expect(store.enableTOTP).toHaveBeenCalledWith("123456");
    expect(messageMock.success).toHaveBeenCalledWith("settings.totp_enabled_success");
    expect(wrapper.find(".totp-setup-panel").exists()).toBe(false);
    expect(wrapper.text()).toContain("settings.totp_enabled");
  });

  it("requires verification before disabling TOTP", async () => {
    const store = makeStore(true);
    store.disableTOTP.mockImplementation(async () => {
      store.user.totp_enabled = false;
    });
    storeControl.current = store;
    const wrapper = mountSettings();

    await buttonByText(wrapper, "settings.totp_disable").trigger("click");
    const input = wrapper.get('input[placeholder="settings.totp_code_placeholder"]');
    await input.setValue("654321");
    await buttonByText(wrapper, "settings.totp_confirm_disable").trigger("click");
    await flushPromises();

    expect(store.disableTOTP).toHaveBeenCalledWith("654321");
    expect(messageMock.success).toHaveBeenCalledWith("settings.totp_disabled_success");
    expect(wrapper.text()).toContain("settings.totp_setup");
  });
});
