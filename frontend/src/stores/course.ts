import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useCourseStore = defineStore('course', () => {
  const courses = ref([])
  const currentCourse = ref(null)
  const chapters = ref([])

  function setCourses(data) {
    courses.value = data
  }

  function setCurrentCourse(data) {
    currentCourse.value = data
  }

  function setChapters(data) {
    chapters.value = data
  }

  return {
    courses,
    currentCourse,
    chapters,
    setCourses,
    setCurrentCourse,
    setChapters
  }
})
