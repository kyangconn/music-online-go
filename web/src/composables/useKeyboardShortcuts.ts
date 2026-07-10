import { onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { usePlayerStore } from "@/store/player";

interface KeyboardShortcutOptions {
  focusSearch: () => void;
}

const isInteractiveTarget = (target: EventTarget | null) => {
  if (!(target instanceof HTMLElement)) return false;
  return (
    target.isContentEditable ||
    Boolean(target.closest("input, textarea, select, button, a, [contenteditable], [role='slider']"))
  );
};

export function useKeyboardShortcuts({ focusSearch }: KeyboardShortcutOptions) {
  const router = useRouter();
  const playerStore = usePlayerStore();

  const handleKeydown = (event: KeyboardEvent) => {
    if (
      event.defaultPrevented ||
      event.ctrlKey ||
      event.metaKey ||
      event.altKey ||
      event.shiftKey ||
      isInteractiveTarget(event.target)
    ) {
      return;
    }

    if (event.code === "Space" && playerStore.hasTrack) {
      event.preventDefault();
      void playerStore.togglePlayback();
      return;
    }

    if (event.key === "/") {
      event.preventDefault();
      focusSearch();
      return;
    }

    if (event.key === "ArrowLeft" && playerStore.hasTrack) {
      event.preventDefault();
      playerStore.seek(playerStore.currentTime - 5);
      return;
    }

    if (event.key === "ArrowRight" && playerStore.hasTrack) {
      event.preventDefault();
      playerStore.seek(playerStore.currentTime + 5);
      return;
    }

    if (event.key.toLowerCase() === "u") {
      event.preventDefault();
      void router.push("/music/add");
    }
  };

  onMounted(() => window.addEventListener("keydown", handleKeydown));
  onUnmounted(() => window.removeEventListener("keydown", handleKeydown));
}
