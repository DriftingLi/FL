<script setup lang="ts">
/**
 * 导航项的图标适配层（ADR-0079 的唯一例外）。
 *
 * 绝大多数条目是 `IconName`（字符串 → 注册表）；**章节序号徽标**是运行时生成的组件
 * （`createChapterIcon(n)` 用 SVG `<text>` 画数字），它不是图标数据，也不该进注册表。
 * 这层只做二选一，让侧栏两种形态都只写一处。
 */
import type { Component } from 'vue'
import type { IconName } from '@/icons/registry'
import UiIcon from '@/components/ui/UiIcon.vue'

defineProps<{ icon?: IconName | Component }>()
</script>

<template>
  <UiIcon v-if="typeof icon === 'string'" :name="icon" />
  <component :is="icon" v-else-if="icon" />
</template>
