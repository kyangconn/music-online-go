import { computed, ref } from "vue";
import { defineStore } from "pinia";
import type { InstanceCapabilities } from "@/types/api";
import request from "@/utils/request";

const defaultCapabilities = (): InstanceCapabilities => ({
  library_mode: "public",
  registration_mode: "open",
  registration_open: true,
  musicbee_submit_enabled: false,
});

export const useInstanceStore = defineStore("instance", () => {
  const capabilities = ref<InstanceCapabilities>(defaultCapabilities());
  const loaded = ref(false);
  let pending: Promise<InstanceCapabilities> | null = null;

  const registrationOpen = computed(() => capabilities.value.registration_open);
  const libraryRequiresAuth = computed(() => capabilities.value.library_mode === "authenticated");

  const load = async (force = false) => {
    if (loaded.value && !force) return capabilities.value;
    if (pending && !force) return pending;

    pending = request
      .get<InstanceCapabilities>("/instance")
      .then((response) => {
        capabilities.value = response.data;
        loaded.value = true;
        return response.data;
      })
      .catch(() => {
        loaded.value = true;
        return capabilities.value;
      })
      .finally(() => {
        pending = null;
      });
    return pending;
  };

  return { capabilities, loaded, registrationOpen, libraryRequiresAuth, load };
});
