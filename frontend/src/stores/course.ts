import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { courseApi } from '@/api/course'
import type { CourseSummary, CourseChapter } from '@/api/course'

export const useCourseStore = defineStore('course', () => {
  const courses: Ref<CourseSummary[]> = ref([])
  const currentCourse: Ref<CourseSummary | null> = ref(null)
  const chapters: Ref<CourseChapter[]> = ref([])

  // 当前已加载的课程详情缓存（供侧栏章节模式与章节页共享，避免重复请求）
  const currentCourseId: Ref<number | null> = ref(null)
  const courseInfo: Ref<CourseSummary | null> = ref(null)
  // 进行中的加载请求，用于并发去重
  let loadPromise: Promise<void> | null = null

  function setCourses(data: CourseSummary[]) {
    courses.value = data
  }

  function setCurrentCourse(data: CourseSummary | null) {
    currentCourse.value = data
  }

  function setChapters(data: CourseChapter[]) {
    chapters.value = data
  }

  // 加载课程详情（含章节列表）。同一 courseId 已缓存则直接返回；
  // 切换到不同 courseId 时重新加载；并发调用复用同一 Promise 避免重复请求。
  async function loadCourse(courseId: number | string) {
    const numericId = Number(courseId)
    if (!Number.isFinite(numericId)) return
    if (currentCourseId.value === numericId && courseInfo.value) return
    if (loadPromise) return loadPromise
    loadPromise = (async () => {
      try {
        // 拦截器已解包信封：course_info/chapters 即业务负载
        const data = await courseApi.getCourseDetail(numericId)
        currentCourseId.value = numericId
        courseInfo.value = data.course_info || null
        chapters.value = data.chapters || []
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
