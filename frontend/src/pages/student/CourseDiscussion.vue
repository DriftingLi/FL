<template>
  <div>
    <ForumPage :course-id="courseId" :course-name="courseName" />
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import ForumPage from '@/pages/student/ForumPage.vue'
import { courseApi } from '@/api/course'

const route = useRoute()
const courseId = ref<number | null>(null)
const courseName = ref('')

onMounted(async () => {
  courseId.value = Number(route.params.courseId) || null
  try {
    const res = await courseApi.getCourseDetail(Number(route.params.courseId))
    if (res.code === 200 && res.data) {
      courseName.value = res.data.course_info?.name || ''
    }
  } catch (e) {
    console.error('加载课程信息失败:', e)
  }
})
</script>
