import request from './request'

export const practiceApi = {
  saveRecord(data) {
    return request.post('/practice/record', data)
  },
  getRecords(params) {
    return request.get('/practice/records', { params })
  },
  getRecordDetail(recordId) {
    return request.get(`/practice/record/${recordId}`)
  },
  getStats() {
    return request.get('/practice/stats')
  },
  getAdminStats() {
    return request.get('/practice/admin/stats')
  },
  getAdminRecords(params) {
    return request.get('/practice/admin/records', { params })
  }
}
