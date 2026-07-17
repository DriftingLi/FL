import request from './request'

// 题库练习模式接口，对应后端 /api/practice-mode
export const practiceModeApi = {
  // 随机练习：随机抽 count 题（可按题型/知识点筛选）
  getFreeQuestions(params?) {
    return request.get('/practice-mode/free', { params })
  },
  // 顺序练习：开始/续练，返回当前批次题目 + 进度
  startSequential() {
    return request.get('/practice-mode/sequential')
  },
  // 顺序练习进度（卡片展示用）
  getSequentialProgress() {
    return request.get('/practice-mode/sequential-progress')
  },
  // 保存练习游标和答题状态（支持顺序/专项/章节练习）
  saveProgress(index: number, mode: string = 'sequential', total: number = 0, answersState: Record<string, unknown> = {}) {
    return request.post('/practice-mode/progress', { index, practice_mode: mode, total, answers_state: answersState })
  },
  // 查询任意模式的练习进度和答题状态（断点续练用）
  getProgress(mode: string = 'sequential') {
    return request.get('/practice-mode/progress', { params: { mode } })
  },
  // 章节练习：按课程分类抽题
  getCategoryQuestions(params: { category: string; count?: number }) {
    return request.get('/practice-mode/category', { params })
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
