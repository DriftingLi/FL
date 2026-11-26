import type { Ref } from 'vue'

export function useFullscreen(elementRef: Ref<HTMLElement | null>) {
  function toggleFullscreen() {
    if (!elementRef.value) return
    if (document.fullscreenElement) {
      document.exitFullscreen()
    } else {
      elementRef.value.requestFullscreen()
    }
  }

  return { toggleFullscreen }
}
