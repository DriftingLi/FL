import request from './request'

const DASHBOARD_TIMEOUT = 45000

export const studentApi = {
  getProfile() {
    return request.get('/student/profile', { timeout: DASHBOARD_TIMEOUT })
  },

  getRecords(params) {
    return request.get('/student/records', { params, timeout: DASHBOARD_TIMEOUT })
  },

  // 学习统计（按天分组），days=7|30
  getStudyStats(params?: { days?: number }) {
    return request.get('/student/study-stats', { params, timeout: DASHBOARD_TIMEOUT })
  }
}
