<!-- PROTOTYPE — throwaway UI switcher. Hidden in production. -->
<template>
  <div v-if="!isProd" class="prototype-switcher" role="toolbar" aria-label="Prototype variant switcher">
    <button class="switcher-arrow" aria-label="Previous variant" @click="goPrev">
      <el-icon><ArrowLeft /></el-icon>
    </button>
    <span class="switcher-label">{{ currentLabel }}</span>
    <button class="switcher-arrow" aria-label="Next variant" @click="goNext">
      <el-icon><ArrowRight /></el-icon>
    </button>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight } from '@element-plus/icons-vue'

const props = defineProps<{
  variants: Array<{ key: string; label: string }>
  current: string
}>()

const route = useRoute()
const router = useRouter()
const isProd = import.meta.env.PROD

const currentLabel = computed(() => {
  const found = props.variants.find(v => v.key === props.current)
  return found ? `${found.key.toUpperCase()} — ${found.label}` : props.current
})

function navigateTo(key: string) {
  router.replace({ query: { ...route.query, variant: key } })
}

function goPrev() {
  const idx = props.variants.findIndex(v => v.key === props.current)
  const prev = idx <= 0 ? props.variants[props.variants.length - 1] : props.variants[idx - 1]
  navigateTo(prev.key)
}

function goNext() {
  const idx = props.variants.findIndex(v => v.key === props.current)
  const next = idx >= props.variants.length - 1 ? props.variants[0] : props.variants[idx + 1]
  navigateTo(next.key)
}

function onKeydown(e: KeyboardEvent) {
  const target = e.target as HTMLElement | null
  if (target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)) return
  if (e.key === 'ArrowLeft') {
    e.preventDefault()
    goPrev()
  } else if (e.key === 'ArrowRight') {
    e.preventDefault()
    goNext()
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.prototype-switcher {
  position: fixed;
  left: 50%;
  bottom: 24px;
  transform: translateX(-50%);
  display: inline-flex;
  align-items: center;
  gap: 12px;
  padding: 8px 16px;
  background: #0f172a;
  color: #f1f5f9;
  border-radius: 9999px;
  box-shadow: 0 10px 15px -3px rgba(0, 0, 0, 0.2), 0 4px 6px -2px rgba(0, 0, 0, 0.1);
  z-index: 1070;
  font-size: 13px;
  user-select: none;
}
.switcher-arrow {
  width: 28px;
  height: 28px;
  border-radius: 9999px;
  border: none;
  background: rgba(255, 255, 255, 0.12);
  color: #f1f5f9;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: background 150ms ease;
}
.switcher-arrow:hover {
  background: rgba(255, 255, 255, 0.22);
}
.switcher-label {
  font-weight: 600;
  letter-spacing: 0.02em;
  white-space: nowrap;
}
</style>
