<template>
  <div class="note-card">
    <div class="card-header">
      <span class="card-title">笔记</span>
      <el-button link type="primary" size="small" @click="dialogVisible=true">{{ note ? '编辑' : '添加笔记' }}</el-button>
    </div>
    <div v-if="note" class="content">
      {{ note.content }}
      <el-button link type="danger" size="small" @click="handleDelete">删除</el-button>
    </div>
    <div v-else class="empty">暂无笔记</div>

    <el-dialog v-model="dialogVisible" title="笔记" width="480px">
      <el-input v-model="input" type="textarea" :rows="5" placeholder="记录你的笔记" maxlength="2000" show-word-limit />
      <template #footer>
        <el-button @click="dialogVisible=false">取消</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { questionInteractionApi } from '@/api/questionInteraction'

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
.note-card { border:1px solid #ebeef5; border-radius:8px; padding:16px; background:#fff; margin-bottom:12px; }
.card-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:8px; }
.card-title { font-weight:600; color:#303133; }
.content { white-space:pre-wrap; color:#606266; font-size:13px; display:flex; justify-content:space-between; gap:8px; }
.empty { color:#909399; font-size:13px; }
</style>
