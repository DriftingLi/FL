<template>
  <div class="breadcrumb-nav">
    <el-breadcrumb separator="/">
      <el-breadcrumb-item :to="{ path: '/training' }">课程中心</el-breadcrumb-item>
      <el-breadcrumb-item :to="{ path: `/course/${courseId}` }">{{ courseName }}</el-breadcrumb-item>
      <el-breadcrumb-item v-if="chapterName">{{ chapterName }}</el-breadcrumb-item>
    </el-breadcrumb>
  </div>
</template>

<script setup lang="ts">
defineProps({
  courseName: { type: String, default: '' },
  courseId: { type: [Number, String], required: true },
  chapterName: { type: String, default: '' }
})
</script>

<style scoped>
.breadcrumb-nav {
  margin-bottom: 16px;
}

.breadcrumb-nav :deep(.el-breadcrumb) {
  font-size: 13px;
}

.breadcrumb-nav :deep(.el-breadcrumb__inner) {
  color: #909399;
}

.breadcrumb-nav :deep(.el-breadcrumb__inner.is-link) {
  color: #606266;
  font-weight: normal;
}

.breadcrumb-nav :deep(.el-breadcrumb__inner.is-link:hover) {
  color: #409eff;
}

.breadcrumb-nav :deep(.el-breadcrumb__item:last-child .el-breadcrumb__inner) {
  color: #303133;
}
</style>
