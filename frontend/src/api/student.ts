import request from './request'

export const studentApi = {
  getProfile() {
    return request.get('/student/profile')
  },

  getRecords(params) {
    return request.get('/student/records', { params })
  },

  // 学习统计（按天分组），days=7|30
  getStudyStats(params?: { days?: number }) {
    return request.get('/student/study-stats', { params })
  }
}
