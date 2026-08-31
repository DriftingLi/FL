<template>
  <UiCard class="note-card" padding="base">
    <template #header><span class="card-title">笔记</span></template>
    <template #actions>
      <UiButton variant="primary" link size="small" @click="dialogVisible=true">{{ note ? '编辑' : '添加笔记' }}</UiButton>
    </template>
    <div v-if="note" class="content">
      {{ note.content }}
      <UiButton variant="danger" link size="small" @click="handleDelete">删除</UiButton>
    </div>
    <div v-else class="empty">暂无笔记</div>

    <el-dialog v-model="dialogVisible" title="笔记" width="480px">
      <el-input v-model="input" type="textarea" :rows="5" placeholder="记录你的笔记" maxlength="2000" show-word-limit />
      <template #footer>
        <UiButton @click="dialogVisible=false">取消</UiButton>
        <UiButton variant="primary" :loading="saving" @click="handleSave">保存</UiButton>
      </template>
    </el-dialog>
  </UiCard>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { questionInteractionApi } from '@/api/questionInteraction'
import UiButton from '@/components/ui/UiButton.vue'
import UiCard from '@/components/ui/UiCard.vue'

const props = defineProps<{ questionId: number }>()
const note = ref<any>(null)
const input = ref('')
const saving = ref(false)
const dialogVisible = ref(false)

async function load(){
  if(!props.questionId) return
  try{
    const res = await questionInteractionApi.getNote(props.questionId)
    note.value = res || null
    input.value = note.value?.content || ''
  } catch {}
}
async function handleSave(){
  if(!input.value.trim()) return
  saving.value=true
  try{
    await questionInteractionApi.upsertNote(props.questionId, { content: input.value.trim() })
    await load()
    dialogVisible.value=false
  } catch {}
  saving.value=false
}
async function handleDelete(){
  try{
    await questionInteractionApi.deleteNote(props.questionId)
    note.value=null
    input.value=''
  } catch {}
}
watch(()=>props.questionId, load)
onMounted(load)
</script>

<style scoped>
/* 容器样式（描边/圆角/内距/底色）与卡片页眉布局已由 UiCard 承担，此处只留外边距 */
.note-card { margin-bottom:12px; }
.card-title { font-weight: var(--font-semibold); color: var(--color-text-primary); }
.content { white-space:pre-wrap; color: var(--color-text-secondary); font-size:13px; display:flex; justify-content:space-between; gap:8px; }
.empty { color: var(--color-text-tertiary); font-size:13px; }
</style>
