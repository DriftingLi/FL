<script setup lang="ts">
import { ref } from 'vue'
import { vLazy } from '@/composables/useLazyLoad'

const props = withDefaults(
  defineProps<{
    name: string
    description?: string
    coverImage?: string
    specialtyId?: number | null
  }>(),
  { description: '', coverImage: '', specialtyId: null }
)

defineEmits<{ click: [] }>()

const coverClassMap: Record<number, string> = {
  1: 'cc-cover-operation',
  2: 'cc-cover-maintenance',
  3: 'cc-cover-safety',
  4: 'cc-cover-battery'
}

const imgFailed = ref(false)

function onImgError() {
  imgFailed.value = true
}

function coverClass() {
  return props.specialtyId ? coverClassMap[props.specialtyId] || 'cc-cover-default' : 'cc-cover-default'
}
</script>

<template>
  <div class="cc-card" @click="$emit('click')">
    <div class="cc-cover" :class="coverClass()">
      <img v-if="coverImage && !imgFailed" v-lazy="coverImage" :alt="name" @error="onImgError" />
      <div v-else class="cc-cover-placeholder">
        <span>{{ name.charAt(0) }}</span>
      </div>
      <slot name="cover" />
    </div>
    <div class="cc-body">
      <slot name="tags" />
      <h3 class="cc-name">{{ name }}</h3>
      <p class="cc-desc">{{ description || '暂无简介' }}</p>
      <slot name="meta" />
    </div>
  </div>
</template>

<style scoped>
.cc-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-xl);
  overflow: hidden;
  cursor: pointer;
  box-shadow: var(--shadow-xs);
  border: 1px solid var(--color-border-light);
  transition: all var(--duration-normal) var(--ease-default);
}

.cc-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-4px);
}

.cc-cover {
  position: relative;
  height: 120px;
  overflow: hidden;
}

.cc-cover img {
  width: 100%;
  height: 100%;
  object-fit: cover;
  transition: transform var(--duration-slow) var(--ease-default);
}

.cc-card:hover .cc-cover img {
  transform: scale(1.05);
}

.cc-cover-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.cc-cover-placeholder span {
  font-size: 40px;
  color: rgba(255, 255, 255, 0.8);
  font-weight: var(--font-bold);
  font-family: var(--font-display);
}

.cc-cover-operation .cc-cover-placeholder,
.cc-cover-operation:not(:has(img)) {
  background: linear-gradient(135deg, #2563eb 0%, #7c3aed 100%);
}

.cc-cover-maintenance .cc-cover-placeholder,
.cc-cover-maintenance:not(:has(img)) {
  background: linear-gradient(135deg, #0f766e 0%, #14b8a6 100%);
}

.cc-cover-safety .cc-cover-placeholder,
.cc-cover-safety:not(:has(img)) {
  background: linear-gradient(135deg, #b45309 0%, #f59e0b 100%);
}

.cc-cover-battery .cc-cover-placeholder,
.cc-cover-battery:not(:has(img)) {
  background: linear-gradient(135deg, #dc2626 0%, #f97316 100%);
}

.cc-cover-default .cc-cover-placeholder,
.cc-cover-default:not(:has(img)) {
  background: linear-gradient(135deg, #6b7280 0%, #9ca3af 100%);
}

.cc-body {
  padding: var(--space-3) var(--space-4) var(--space-4);
}

.cc-name {
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  color: var(--color-text-primary);
  margin-bottom: var(--space-1);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.cc-desc {
  font-size: var(--text-xs);
  color: var(--color-text-secondary);
  margin-bottom: var(--space-2);
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
  overflow: hidden;
  min-height: 34px;
}
</style>
