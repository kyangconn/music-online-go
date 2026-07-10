import { computed, onMounted, onUnmounted, ref } from "vue";

interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: "accepted" | "dismissed"; platform: string }>;
}

export function usePwaInstall() {
  const installPrompt = ref<BeforeInstallPromptEvent | null>(null);
  const isStandalone = ref(false);

  const updateDisplayMode = () => {
    isStandalone.value = window.matchMedia("(display-mode: standalone)").matches;
  };

  const handleBeforeInstallPrompt = (event: Event) => {
    event.preventDefault();
    installPrompt.value = event as BeforeInstallPromptEvent;
  };

  const handleInstalled = () => {
    installPrompt.value = null;
    updateDisplayMode();
  };

  const install = async () => {
    const prompt = installPrompt.value;
    if (!prompt) return false;

    try {
      await prompt.prompt();
      const { outcome } = await prompt.userChoice;
      installPrompt.value = null;
      return outcome === "accepted";
    } catch {
      installPrompt.value = null;
      return false;
    }
  };

  onMounted(() => {
    updateDisplayMode();
    window.addEventListener("beforeinstallprompt", handleBeforeInstallPrompt);
    window.addEventListener("appinstalled", handleInstalled);
  });

  onUnmounted(() => {
    window.removeEventListener("beforeinstallprompt", handleBeforeInstallPrompt);
    window.removeEventListener("appinstalled", handleInstalled);
  });

  return {
    canInstall: computed(() => Boolean(installPrompt.value) && !isStandalone.value),
    install,
  };
}
