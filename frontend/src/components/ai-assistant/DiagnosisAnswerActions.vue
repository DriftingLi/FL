<template>
  <!-- 助手回答下方的动作行（票 2241 第 14 行 → 2280 缺陷五补落）。
       「复制」是这一页唯一一个「离开本站还有用」的动作：答案要贴进工单、发到班组群。
       对方 app.js 的答案下方同样只有复制，不做点赞/重新生成这类无消费方的按钮。 -->
  <div v-if="content" class="answer-actions mt-2 flex items-center justify-end gap-1">
    <UiButton variant="text" size="small" :icon="'copy'" @click="copy">复制</UiButton>
  </div>
</template>

<script setup lang="ts">
// 复制通道：`navigator.clipboard` 在非安全上下文（内网 http 访问）不可用，
// 失败必须显式告知并让用户手抄，不能静默吞掉 —— 这里沿用 recruit/CompanyContactInfo 的兜底口径。
import { ElMessage } from 'element-plus'
import UiButton from '@/components/ui/UiButton.vue'

const props = defineProps<{
  /** 助手回答正文（原样复制，不做任何清理：用户要的就是看到的那段话） */
  content: string
}>()

async function copy() {
  const text = props.content
  if (!text) return
  try {
    await navigator.clipboard.writeText(text)
    ElMessage.success('已复制')
  } catch {
    ElMessage.warning('复制失败，请手动选择文字复制')
  }
}
</script>
