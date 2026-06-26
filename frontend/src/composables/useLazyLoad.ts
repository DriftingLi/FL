import { onMounted, onBeforeUnmount } from 'vue'
import { resolveFileUrl } from '@/utils/fileUrl'

const observerMap = new WeakMap()

function getObserver(rootMargin = '200px') {
  if (typeof IntersectionObserver === 'undefined') return null
  return new IntersectionObserver(
    (entries) => {
      entries.forEach((entry) => {
        if (entry.isIntersecting) {
          const el = entry.target
          const src = el.dataset.lazySrc
          if (src) {
            el.src = resolveFileUrl(src)
            delete el.dataset.lazySrc
          }
          const observer = observerMap.get(el)
          if (observer) {
            observer.unobserve(el)
            observerMap.delete(el)
          }
        }
      })
    },
    { rootMargin }
  )
}

export function useLazyLoad(rootMargin = '200px') {
  let observer = null

  function observe(el) {
    if (!el || !observer) return
    observer.observe(el)
    observerMap.set(el, observer)
  }

  function unobserve(el) {
    if (!el || !observer) return
    observer.unobserve(el)
    observerMap.delete(el)
  }

  onMounted(() => {
    observer = getObserver(rootMargin)
  })

  onBeforeUnmount(() => {
    if (observer) {
      observer.disconnect()
      observer = null
    }
  })

  return { observe, unobserve }
}

export const vLazy = {
  mounted(el, binding) {
    const src = binding.value
    if (!src) return
    el.dataset.lazySrc = src
    const rootMargin = binding.arg || '200px'
    const observer = getObserver(rootMargin)
    if (observer) {
      observer.observe(el)
      observerMap.set(el, observer)
    } else {
      el.src = resolveFileUrl(src)
    }
  },
  updated(el, binding) {
    const src = binding.value
    if (!src) return
    if (el.src === resolveFileUrl(src)) return
    el.dataset.lazySrc = src
    el.src = ''
    const rootMargin = binding.arg || '200px'
    const observer = getObserver(rootMargin)
    if (observer) {
      const oldObserver = observerMap.get(el)
      if (oldObserver) {
        oldObserver.unobserve(el)
      }
      observer.observe(el)
      observerMap.set(el, observer)
    } else {
      el.src = resolveFileUrl(src)
    }
  },
  unmounted(el) {
    const observer = observerMap.get(el)
    if (observer) {
      observer.unobserve(el)
      observerMap.delete(el)
    }
  }
}
