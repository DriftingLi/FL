import request from './request'

// 题库练习模式接口，对应后端 /api/practice-mode
export const practiceModeApi = {
  // 自由练习：随机抽题，按学员等级自动筛选难度与题量
  getFreeQuestions(params?) {
    return request.get('/practice-mode/free', { params })
  },
  // 按知识点练习
  getKnowledgePointPractice(params) {
    return request.get('/practice-mode/knowledge-point', { params })
  },
  // 知识点练习进度
  getKnowledgePointProgress(params?) {
    return request.get('/practice-mode/knowledge-point-progress', { params })
  },
  // 提交单题答案并判定
  submitAnswer(data) {
    return request.post('/practice-mode/submit', data)
  },
  // 练习统计
  getStats() {
    return request.get('/practice-mode/stats')
  },
  // 练习历史
  getHistory(params) {
    return request.get('/practice-mode/history', { params })
  }
}
