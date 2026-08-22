<template>
  <div class="knowledge-card">
    <div class="card-header"><span class="card-title">考点</span></div>
    <div class="content">
      <template v-if="tags.length>0">
        <el-tag v-for="t in tags" :key="t.id" size="small" style="margin:4px 6px 0 0">{{ t.name }}</el-tag>
      </template>
      <span v-else class="empty">暂无</span>
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{ tags: any[] }>()
</script>

<style scoped>
.knowledge-card { border:1px solid #ebeef5; border-radius:8px; padding:16px; background:#fff; margin-bottom:12px; }
.card-header { margin-bottom:8px; }
.card-title { font-weight:600; color:#303133; }
.content { min-height:24px; }
.empty { color:#909399; font-size:13px; }
</style>
