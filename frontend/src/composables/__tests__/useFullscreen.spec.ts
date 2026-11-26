import { describe, expect, it, vi } from 'vitest'
import { ref } from 'vue'
import { useFullscreen } from '../useFullscreen'

describe('useFullscreen', () => {
  it('无元素时不动，有元素时进入或退出全屏', () => {
    const elementRef = ref<HTMLElement | null>(null)
    const { toggleFullscreen } = useFullscreen(elementRef)
    const requestFullscreen = vi.fn()
    const exitFullscreen = vi.fn()

    toggleFullscreen()
    expect(requestFullscreen).not.toHaveBeenCalled()

    elementRef.value = { requestFullscreen } as unknown as HTMLElement
    Object.defineProperty(document, 'fullscreenElement', { configurable: true, value: null })
    Object.defineProperty(document, 'exitFullscreen', { configurable: true, value: exitFullscreen })
    toggleFullscreen()
    expect(requestFullscreen).toHaveBeenCalledTimes(1)

    Object.defineProperty(document, 'fullscreenElement', { configurable: true, value: elementRef.value })
    toggleFullscreen()
    expect(exitFullscreen).toHaveBeenCalledTimes(1)
  })
})
