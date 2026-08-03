import request from './request'

export type ExportKind = 'students' | 'exam-records' | 'questions' | 'evaluations'

const exportLabels: Record<ExportKind, string> = {
  students: '学员名单.xlsx',
  'exam-records': '成绩单.xlsx',
  questions: '题库.xlsx',
  evaluations: '评估记录.xlsx'
}

export async function downloadExport(kind: ExportKind) {
  const blob = (await request.get(`/admin/export/${kind}`, {
    responseType: 'blob'
  })) as unknown as Blob
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = exportLabels[kind]
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  URL.revokeObjectURL(url)
}
