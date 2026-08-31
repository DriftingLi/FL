<template>
  <div class="q-options" :class="{ 'q-options--compact': compact }">
    <div
      v-for="(label, key) in options"
      :key="key"
      class="q-option"
      :class="optionClass(key)"
      @click="!disabled && $emit('select', key)"
    >
      <span class="opt-label">{{ key }}</span>
      <span>{{ label }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
// 答题选项渲染/选择组件：渲染选项、选中态、交卷后的正确/错误高亮。
// 判断题由页面通过 buildQuestionOptions 传入对/错模板；多选 toggle 由父级接管。
const props = defineProps<{
  options: Record<string, string>
  selectedKeys: (string | number)[]
  multiChoice?: boolean
  disabled?: boolean
  /** 提交后传入正确答案（string 或数组），启用正确/错误高亮 */
  correctAnswer?: unknown
  /** 提交后的用户作答，用于标记选中态 */
  userAnswer?: unknown
  /** 紧凑样式（错题重做列表） */
  compact?: boolean
}>()

defineEmits<{ (e: 'select', key: string): void }>()

function optionClass(key: string): Record<string, boolean> {
  const cls: Record<string, boolean> = { selected: props.selectedKeys.includes(key) }
  if (props.correctAnswer !== undefined) {
    const correctArr = Array.isArray(props.correctAnswer)
      ? props.correctAnswer.map(String)
      : String(props.correctAnswer ?? '').split(',')
    cls['opt-correct'] = correctArr.includes(key)
    cls['opt-wrong'] = cls.selected && !correctArr.includes(key)
  }
  return cls
}
</script>

<style scoped>
.q-options { display: flex; flex-direction: column; gap: 8px; }
.q-option { display: flex; align-items: center; padding: 10px 15px; border: 1px solid #dcdfe6; border-radius: 8px; cursor: pointer; transition: all var(--duration-base) var(--ease-default); }
.q-option:hover { border-color: #409eff; }
.q-option.selected { border-color: #409eff; background: #ecf5ff; }
.q-option.opt-correct { border-color: #67c23a; background: #f0f9eb; }
.q-option.opt-wrong { border-color: #f56c6c; background: #fef0f0; }
.opt-label { width: 28px; height: 28px; line-height: 28px; text-align: center; border-radius: 50%; background: #f5f7fa; margin-right: 10px; font-weight: bold; flex-shrink: 0; }
.q-options--compact { gap: 6px; margin-bottom: 10px; }
.q-options--compact .q-option { padding: 8px 12px; border-radius: 6px; }
.q-options--compact .opt-label { width: 24px; height: 24px; line-height: 24px; margin-right: 8px; font-size: 12px; }
</style>
