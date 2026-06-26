<template>
  <div class="chapter-nav" :class="{ 'compact-mode': compact }">
    <div class="nav-header" v-if="!compact">
      <h3>课程章节</h3>
      <span class="chapter-count">{{ chapters.length }} 章</span>
    </div>
    <div class="nav-list" ref="navListRef">
      <div
        v-for="(chapter, index) in chapters"
        :key="chapter.chapter_id"
        class="nav-item"
        :class="{ 'is-active': activeChapterId === chapter.chapter_id }"
        @click="$emit('select', chapter)"
      >
        <div class="item-index">{{ String(index + 1).padStart(2, '0') }}</div>
        <div class="item-body" v-if="!compact">
          <div class="item-title">{{ chapter.title }}</div>
          <div class="item-meta">
            <el-tag
              v-if="chapter.content_type"
              size="small"
              :type="getContentTypeTagType(chapter.content_type)"
            >
              {{ getContentTypeLabel(chapter.content_type) }}
            </el-tag>
            <span
              v-if="getChapterStatus(chapter) === 'completed'"
              class="status-badge completed"
            >
              <el-icon><CircleCheck /></el-icon>
            </span>
            <span
              v-else-if="getChapterStatus(chapter) === 'studying'"
              class="status-badge studying"
            >
              <span class="dot"></span>
            </span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { CircleCheck } from '@element-plus/icons-vue'

const props = defineProps({
  chapters: { type: Array, default: () => [] },
  courseId: { type: [Number, String], required: true },
  activeChapterId: { type: [Number, String], default: null },
  compact: { type: Boolean, default: false }
})

defineEmits(['select'])

const navListRef = ref(null)

function getContentTypeTagType(contentType) {
  const types = {
    'document': '',
    'ppt': 'warning',
    'video': 'danger',
    'image': 'success',
    'spreadsheet': 'success'
  }
  return types[contentType] || 'info'
}

function getContentTypeLabel(contentType) {
  const labels = {
    'document': '文档',
    'ppt': 'PPT',
    'video': '视频',
    'image': '图片',
    'spreadsheet': '表格'
  }
  return labels[contentType] || contentType
}

function getChapterStatus(chapter) {
  if (chapter.study_status === 'completed') return 'completed'
  if (chapter.study_status === 'studying') return 'studying'
  return null
}

watch(() => props.activeChapterId, () => {
  nextTick(() => {
    if (!navListRef.value) return
    const activeEl = navListRef.value.querySelector('.is-active')
    if (activeEl) {
      activeEl.scrollIntoView({ behavior: 'smooth', block: 'nearest' })
    }
  })
})
</script>

<style scoped>
.chapter-nav {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.nav-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px 16px 12px;
  border-bottom: 1px solid #ebeef5;
}

.nav-header h3 {
  font-size: 15px;
  font-weight: 600;
  color: #303133;
  margin: 0;
}

.chapter-count {
  font-size: 12px;
  color: #909399;
}

.nav-list {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}

.nav-item {
  display: flex;
  align-items: center;
  padding: 10px 12px;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s ease;
  border-left: 3px solid transparent;
  margin-bottom: 4px;
}

.nav-item:hover {
  background: #f5f7fa;
}

.nav-item.is-active {
  background: #ecf5ff;
  border-left-color: #409eff;
}

.item-index {
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #e4e7ed;
  color: #606266;
  border-radius: 50%;
  font-size: 12px;
  font-weight: 600;
  flex-shrink: 0;
  margin-right: 10px;
}

.nav-item.is-active .item-index {
  background: #409eff;
  color: #fff;
}

.item-body {
  flex: 1;
  min-width: 0;
}

.item-title {
  font-size: 13px;
  color: #303133;
  line-height: 1.4;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  margin-bottom: 4px;
}

.nav-item.is-active .item-title {
  color: #409eff;
  font-weight: 500;
}

.item-meta {
  display: flex;
  align-items: center;
  gap: 6px;
}

.status-badge {
  display: flex;
  align-items: center;
}

.status-badge.completed {
  color: #67c23a;
  font-size: 14px;
}

.status-badge.studying .dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #409eff;
  display: inline-block;
}

.compact-mode .nav-item {
  justify-content: center;
  padding: 8px 4px;
  border-left: none;
  border-radius: 6px;
}

.compact-mode .item-index {
  margin-right: 0;
  width: 36px;
  height: 36px;
  font-size: 11px;
}

.compact-mode .nav-item.is-active {
  background: #409eff;
}

.compact-mode .nav-item.is-active .item-index {
  background: transparent;
  color: #fff;
}
</style>
