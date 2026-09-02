<template>
  <div v-if="approved" class="mt-3 rounded-card border border-line bg-panel p-3 text-sm">
    <div class="text-xs font-semibold text-ink-3 mb-1">企业联系方式（已授权）</div>
    <div class="grid gap-1 text-ink">
      <div><span class="text-ink-3 mr-2">企业</span>{{ companyName || '-' }}</div>
      <div><span class="text-ink-3 mr-2">联系人</span>{{ contactName || '-' }}</div>
      <div v-if="phone" class="flex items-center gap-2">
        <span class="text-ink-3 mr-2">电话</span><span>{{ phone }}</span>
        <UiButton variant="text" size="small" @click="copyPhone">复制</UiButton>
      </div>
      <div v-if="email"><span class="text-ink-3 mr-2">邮箱</span>{{ email }}</div>
      <div v-if="wechat"><span class="text-ink-3 mr-2">微信</span>{{ wechat }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ElMessage } from 'element-plus'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps<{
  approved: boolean
  companyName?: string
  contactName?: string
  phone?: string
  email?: string
  wechat?: string
}>()

async function copyPhone() {
  if (!props.phone) return
  try {
    await navigator.clipboard.writeText(props.phone)
    ElMessage.success('电话已复制')
  } catch {
    ElMessage.warning('复制失败，请手动复制')
  }
}
</script>
