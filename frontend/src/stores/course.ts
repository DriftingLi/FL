import { defineStore } from 'pinia'
import { ref } from 'vue'
import { courseApi } from '@/api/course'

export const useCourseStore = defineStore('course', () => {
  const courses = ref([])
  const currentCourse = ref(null)
  const chapters = ref([])

  // 当前已加载的课程详情缓存（供侧栏章节模式与章节页共享，避免重复请求）
  const currentCourseId = ref<string | number | null>(null)
  const courseInfo = ref<any>(null)
  // 进行中的加载请求，用于并发去重
  let loadPromise: Promise<void> | null = null

  function setCourses(data) {
    courses.value = data
  }

  function setCurrentCourse(data) {
    currentCourse.value = data
  }

  function setChapters(data) {
    chapters.value = data
  }

  // 加载课程详情（含章节列表）。同一 courseId 已缓存则直接返回；
  // 切换到不同 courseId 时重新加载；并发调用复用同一 Promise 避免重复请求。
  async function loadCourse(courseId: string | number) {
    if (currentCourseId.value === courseId && courseInfo.value) return
    if (loadPromise) return loadPromise
    loadPromise = (async () => {
      try {
        const res = await courseApi.getCourseDetail(courseId)
        if (res.code === 200) {
          currentCourseId.value = courseId
          courseInfo.value = res.data.course_info || null
          chapters.value = res.data.chapters || []
        }
      } finally {
        loadPromise = null
      }
    })()
    return loadPromise
  }

  function clearCourse() {
    currentCourseId.value = null
    courseInfo.value = null
    chapters.value = []
  }

  return {
    courses,
    currentCourse,
    chapters,
    currentCourseId,
    courseInfo,
    setCourses,
    setCurrentCourse,
    setChapters,
    loadCourse,
    clearCourse
  }
})
