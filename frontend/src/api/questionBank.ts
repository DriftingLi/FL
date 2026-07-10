import request from './request'

export const questionBankApi = {
  getQuestions(params) {
    return request.get('/question-bank/questions', { params })
  },
  createQuestion(data) {
    return request.post('/question-bank/questions', data)
  },
  getQuestion(id) {
    return request.get(`/question-bank/questions/${id}`)
  },
  updateQuestion(id, data) {
    return request.put(`/question-bank/questions/${id}`, data)
  },
  deleteQuestion(id) {
    return request.delete(`/question-bank/questions/${id}`)
  },
  publishQuestion(id) {
    return request.post(`/question-bank/questions/${id}/publish`)
  },
  batchPublish(questionIds) {
    return request.post('/question-bank/questions/batch-publish', { question_ids: questionIds })
  },
  batchImport(questions) {
    return request.post('/question-bank/questions/batch-import', { questions })
  },
  getStats() {
    return request.get('/question-bank/stats')
  },
  getKnowledgePoints(params?) {
    return request.get('/question-bank/knowledge-points', { params })
  },
  createKnowledgePoint(data) {
    return request.post('/question-bank/knowledge-points', data)
  },
  updateKnowledgePoint(id, data) {
    return request.put(`/question-bank/knowledge-points/${id}`, data)
  },
  deleteKnowledgePoint(id) {
    return request.delete(`/question-bank/knowledge-points/${id}`)
  },
  uploadImage(formData) {
    return request.post('/question-bank/upload-image', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 30000
    })
  }
}
