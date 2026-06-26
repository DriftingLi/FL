import { defineStore } from 'pinia'
import { ref } from 'vue'
import { studentApi } from '@/api/student'

export const useUserStore = defineStore('user', () => {
  const profile = ref({})
  const studyStats = ref({})
  const courseProgress = ref([])
  const studyRecords = ref([])
  const recordsPagination = ref({})

  async function fetchProfile() {
    try {
      const res = await studentApi.getProfile()
      if (res.code === 200 && res.data) {
        profile.value = res.data.student_info || {}
        studyStats.value = res.data.study_stats || {}
        courseProgress.value = res.data.course_progress || []
      }
      return res
    } catch (e) {
      console.error('Failed to fetch profile:', e)
      throw e
    }
  }

  async function fetchRecords(params) {
    try {
      const res = await studentApi.getRecords(params)
      if (res.code === 200 && res.data) {
        studyRecords.value = res.data.records || []
        recordsPagination.value = {
          total: res.data.total,
          page: res.data.page,
          pages: res.data.pages
        }
      }
      return res
    } catch (e) {
      console.error('Failed to fetch records:', e)
      throw e
    }
  }

  function clearData() {
    profile.value = {}
    studyStats.value = {}
    courseProgress.value = []
    studyRecords.value = []
    recordsPagination.value = {}
  }

  return {
    profile,
    studyStats,
    courseProgress,
    studyRecords,
    recordsPagination,
    fetchProfile,
    fetchRecords,
    clearData
  }
})
