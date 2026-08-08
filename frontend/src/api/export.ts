import { unwrappedRequest } from './request'

export type ExportKind = 'students' | 'exam-records' | 'questions' | 'evaluations'

const exportLabels: Record<ExportKind, string> = {
  students: '学员名单.csv',
  'exam-records': '成绩单.csv',
  questions: '题库.csv',
  evaluations: '评估记录.csv'
}

export async function downloadExport(kind: ExportKind) {
  // responseType: 'blob' 时拦截器直接返回 response.data（Blob），不走 ApiResponse 解包
  const blob = await unwrappedRequest.get<Blob>(`/admin/export/${kind}`, {
    responseType: 'blob'
  })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = exportLabels[kind]
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
