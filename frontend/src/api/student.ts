import request from './request'

export const studentApi = {
  getProfile() {
    return request.get('/student/profile')
  },

  getRecords(params) {
    return request.get('/student/records', { params })
  }
}
