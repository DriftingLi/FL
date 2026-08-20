// student.ts 我的课程契约测试（ADR-0017）。
import { describe, it, expect, vi, beforeEach } from 'vitest'

vi.mock('@/api/request', () => ({
  unwrappedRequest: { get: vi.fn() }
}))

import { unwrappedRequest } from '@/api/request'
import { studentApi } from '../student'

const mockGet = vi.mocked(unwrappedRequest.get)

beforeEach(() => {
  mockGet.mockClear()
})

describe('studentApi 我的课程', () => {
  it('getStudentCourses：路径 /student/courses', async () => {
    mockGet.mockResolvedValue({ courses: [], continue_learning: null })
    await studentApi.getStudentCourses()
    expect(mockGet).toHaveBeenCalledWith('/student/courses', expect.objectContaining({ timeout: 45000 }))
  })

  it('getStudentCourseDetail：路径 /student/courses/:courseId', async () => {
    mockGet.mockResolvedValue({ course_id: 1, chapters: [] })
    await studentApi.getStudentCourseDetail(4)
    expect(mockGet).toHaveBeenCalledWith('/student/courses/4', expect.objectContaining({ timeout: 45000 }))
  })
})
