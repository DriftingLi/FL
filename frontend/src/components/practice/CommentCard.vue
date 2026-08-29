<template>
  <div class="comment-card">
    <div class="card-header">
      <span class="card-title">评论</span>
      <el-button v-if="total>3" link type="primary" size="small" @click="dialogVisible=true">查看所有评论</el-button>
    </div>
    <div v-if="comments.length===0" class="empty">暂无评论</div>
    <div v-else class="list">
      <div v-for="c in displayComments" :key="c.id" class="comment-item">
        <div class="comment-main">
          <div class="comment-author">
            <el-avatar :size="24" :src="c.avatar_url || undefined">{{ (c.username||'?').charAt(0) }}</el-avatar>
            <span class="username">{{ c.username || '用户' }}</span>
            <span class="time">{{ c.created_at }}</span>
          </div>
          <span class="content">{{ c.content }}</span>
        </div>
        <el-button v-if="c.user_id===currentUserId" link type="danger" size="small" @click="handleDelete(c.id)">删除</el-button>
      </div>
    </div>
    <div class="input-row">
      <el-input v-model="input" placeholder="写下你的评论" size="small" style="flex:1" @keyup.enter="handleSubmit" />
      <el-button type="primary" size="small" :loading="submitting" @click="handleSubmit">发布</el-button>
    </div>

    <el-dialog v-model="dialogVisible" title="所有评论" width="520px">
      <div v-for="c in comments" :key="c.id" class="comment-item">
        <div class="comment-main">
          <div class="comment-author">
            <el-avatar :size="24" :src="c.avatar_url || undefined">{{ (c.username||'?').charAt(0) }}</el-avatar>
            <span class="username">{{ c.username || '用户' }}</span>
            <span class="time">{{ c.created_at }}</span>
          </div>
          <span class="content">{{ c.content }}</span>
        </div>
        <el-button v-if="c.user_id===currentUserId" link type="danger" size="small" @click="handleDelete(c.id)">删除</el-button>
      </div>
      <div v-if="comments.length===0" class="empty">暂无评论</div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, watch } from 'vue'
import { useAuthStore } from '@/stores/auth'
import { questionInteractionApi } from '@/api/questionInteraction'

const props = defineProps<{ questionId: number }>()
const authStore = useAuthStore()
const currentUserId = computed(()=> (authStore.userInfo as any)?.user_id || (authStore.userInfo as any)?.id || 0)
const comments = ref<any[]>([])
const total = ref(0)
const input = ref('')
const submitting = ref(false)
const dialogVisible = ref(false)

const displayComments = computed(()=> comments.value.slice(0,3))

async function load(){
  if(!props.questionId) return
  try{
    const res = await questionInteractionApi.listComments(props.questionId, { page:1, page_size:20 })
    comments.value = res?.items || []
    total.value = res?.total || comments.value.length
  } catch {}
}
async function handleSubmit(){
  if(!input.value.trim()) return
  submitting.value=true
  try{
    await questionInteractionApi.createComment(props.questionId, { content: input.value.trim() })
    input.value=''
    await load()
  } catch {}
  submitting.value=false
}
async function handleDelete(id:number){
  try{
    await questionInteractionApi.deleteComment(id)
    await load()
  } catch {}
}

watch(()=>props.questionId, load)
onMounted(load)
</script>

<style scoped>
.comment-card { border:1px solid #ebeef5; border-radius:8px; padding:16px; background:#fff; margin-bottom:12px; }
.card-header { display:flex; justify-content:space-between; align-items:center; margin-bottom:8px; }
.card-title { font-weight:600; color:#303133; }
.empty { color:#909399; font-size:13px; }
.list { display:flex; flex-direction:column; gap:8px; margin-bottom:10px; }
.comment-item { display:flex; justify-content:space-between; align-items:flex-start; gap:8px; padding:8px 0; border-bottom:1px solid #f2f3f5; }
.comment-main { flex:1; display:flex; flex-direction:column; gap:4px; }
.comment-author { display:flex; align-items:center; gap:6px; }
.username { font-size:12px; color:#303133; font-weight:500; }
.comment-item .content { flex:1; font-size:13px; color:#606266; word-break:break-word; }
.input-row { display:flex; gap:8px; margin-top:8px; }
.time { font-size:11px; color:#c0c4cc; margin-left:4px; }
</style>
