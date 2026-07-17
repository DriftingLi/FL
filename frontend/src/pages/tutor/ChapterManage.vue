<template>
  <div class="chapter-manage-page" v-loading="loading">
    <div class="page-header">
      <el-button text @click="goBack">
        <el-icon><ArrowLeft /></el-icon> 返回课程列表
      </el-button>
      <h2 v-if="courseInfo">{{ courseInfo.name }} - 章节管理</h2>
      <p class="page-desc">点击章节进入编辑，可修改正文、上传与预览多种类型内容</p>
    </div>

    <div v-if="chapters.length === 0 && !loading" class="empty-state">
      <el-empty description="暂无章节" />
    </div>

    <div v-else class="chapter-list">
      <div
        v-for="(chapter, index) in chapters"
        :key="chapter.chapter_id"
        class="chapter-card"
        @click="goToEdit(chapter.chapter_id)"
      >
        <div class="chapter-index">{{ String(index + 1).padStart(2, '0') }}</div>
        <div class="chapter-body">
          <div class="chapter-title">{{ chapter.title }}</div>
          <div class="chapter-meta">
            <el-tag size="small" :type="getContentTypeTagType(chapter.content_type)">
              {{ getContentTypeLabel(chapter.content_type) }}
            </el-tag>
            <span v-if="chapter.duration" class="meta-item">
              <el-icon><Timer /></el-icon> {{ chapter.duration }}分钟
            </span>
            <span v-if="getFileCount(chapter) > 0" class="meta-item">
              <el-icon><Document /></el-icon> {{ getFileCount(chapter) }} 个文件
            </span>
            <span v-if="chapter.content" class="meta-item has-content">
              <el-icon><EditPen /></el-icon> 含正文
            </span>
          </div>
        </div>
        <div class="chapter-arrow">
          <el-icon><ArrowRight /></el-icon>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, ArrowRight, Timer, Document, EditPen } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { tutorApi } from '@/api/tutor'

const route = useRoute()
const router = useRouter()

const loading = ref(false)
const courseInfo = ref<any>(null)
const chapters = ref<any[]>([])

function getContentTypeTagType(contentType: string) {
  const types: Record<string, string> = {
    'text': '',
    'document': '',
    'ppt': 'warning',
    'video': 'danger',
    'image': 'success'
  }
  return types[contentType] || 'info'
}

function getContentTypeLabel(contentType: string) {
  const labels: Record<string, string> = {
    'text': '图文',
    'document': '文档',
    'ppt': 'PPT',
    'video': '视频',
    'image': '图片'
  }
  return labels[contentType] || contentType || '未设置'
}

function getFileCount(chapter: any): number {
  return (chapter.files || []).length
}

async function loadChapters() {
  loading.value = true
  try {
    const courseId = route.params.id
    const res = await tutorApi.getCourseChapters(courseId)
    if (res.code === 200) {
      courseInfo.value = res.data.course
      chapters.value = res.data.chapters || []
    }
  } catch (e) {
    console.error('Failed to load chapters:', e)
    ElMessage.error('加载章节失败')
  } finally {
    loading.value = false
  }
}

function goBack() {
  router.push('/training/tutor/courses')
}

function goToEdit(chapterId: number) {
  const courseId = route.params.id
  router.push(`/training/tutor/course/${courseId}/chapter/${chapterId}`)
}

onMounted(() => {
  loadChapters()
})
</script>

<style scoped>
.chapter-manage-page {
  max-width: 1000px;
  margin: 0 auto;
}

.page-header {
  margin-bottom: 20px;
}

.page-header h2 {
  font-size: 20px;
  color: #303133;
  margin-top: 8px;
  margin-bottom: 4px;
}

.page-desc {
  font-size: 13px;
  color: #909399;
  margin: 0;
}

.chapter-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.chapter-card {
  display: flex;
  align-items: center;
  gap: 16px;
  background: #fff;
  border: 1px solid #ebeef5;
  border-radius: 8px;
  padding: 16px 20px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.chapter-card:hover {
  border-color: #409eff;
  box-shadow: 0 2px 12px rgba(64, 158, 255, 0.1);
  transform: translateX(2px);
}

.chapter-index {
  width: 36px;
  height: 36px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #ecf5ff;
  color: #409eff;
  border-radius: 50%;
  font-size: 13px;
  font-weight: 600;
  flex-shrink: 0;
}

.chapter-body {
  flex: 1;
  min-width: 0;
}

.chapter-title {
  font-size: 15px;
  font-weight: 500;
  color: #303133;
  margin-bottom: 8px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chapter-meta {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}

.meta-item {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  color: #909399;
}

.meta-item.has-content {
  color: #67c23a;
}

.chapter-arrow {
  color: #c0c4cc;
  flex-shrink: 0;
}

.chapter-card:hover .chapter-arrow {
  color: #409eff;
}

@media screen and (max-width: 768px) {
  .chapter-card {
    flex-wrap: wrap;
  }
  .chapter-arrow {
    display: none;
  }
}
</style>
