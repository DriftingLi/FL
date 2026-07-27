import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Ref } from 'vue'
import { studentApi } from '@/api/student'

interface StudentInfo {
  student_id?: number
  username?: string
  name?: string
  status?: number
  created_at?: string
  [key: string]: unknown
}

interface StudyStats {
  total_courses?: number
  total_study_duration?: number
  completed_courses?: number
  learning_courses?: number
  latest_study_time?: string
  exam_count?: number
  avg_score?: number
  [key: string]: unknown
}

interface CourseProgressItem {
  course_id: number
  course_name: string
  progress: number
  study_duration: number
  total_chapters?: number
  study_date?: string
  [key: string]: unknown
}

interface StudyRecordItem {
  record_id: number
  course_id: number
  course_name?: string
  chapter_id?: number | null
  chapter_title?: string | null
  study_duration: number
  progress: number
  study_date: string
  [key: string]: unknown
}

interface RecordsPagination {
  total?: number
  page?: number
  pages?: number
}

interface FetchRecordsParams {
  page?: number
  page_size?: number
  start_date?: string
  end_date?: string
}

export const useUserStore = defineStore('user', () => {
  const profile: Ref<StudentInfo> = ref({})
  const studyStats: Ref<StudyStats> = ref({})
  const courseProgress: Ref<CourseProgressItem[]> = ref([])
  const studyRecords: Ref<StudyRecordItem[]> = ref([])
  const recordsPagination: Ref<RecordsPagination> = ref({})

  async function fetchProfile() {
    try {
      const res = await studentApi.getProfile()
      if (res.code === 200 && res.data) {
        const data = res.data as {
          student_info?: StudentInfo
          study_stats?: StudyStats
          course_progress?: CourseProgressItem[]
        }
        profile.value = data.student_info || {}
        studyStats.value = data.study_stats || {}
        courseProgress.value = data.course_progress || []
      }
      return res
    } catch (e) {
      console.error('Failed to fetch profile:', e)
      throw e
    }
  }

  async function fetchRecords(params: FetchRecordsParams) {
    try {
      const res = await studentApi.getRecords(params)
      if (res.code === 200 && res.data) {
        const data = res.data as {
          records?: StudyRecordItem[]
          total?: number
          page?: number
          pages?: number
        }
        studyRecords.value = data.records || []
        recordsPagination.value = {
          total: data.total,
          page: data.page,
          pages: data.pages
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
