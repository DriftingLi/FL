import request, { aiRequest } from './request'

export const aiApi = {
  generateText(data) {
    return aiRequest.post('/ai/generate/text', data)
  },

  getHistory(params) {
    return request.get('/ai/history', { params })
  },

  getCozeToken() {
    return request.get('/ai/coze/token')
  }
}
