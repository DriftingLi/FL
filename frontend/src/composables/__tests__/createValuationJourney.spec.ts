// createValuationJourney：结果旅程状态机的接口级测试。
// seam：journey 接口——submit/fetch 用内存 adapter，不触达 API 层。
import { describe, it, expect } from 'vitest'
import { createValuationJourney } from '@/composables/createValuationJourney'

interface SubmitResult {
  id: number
  value: string
}

interface Detail {
  id: number
  detail: string
}

function makeJourney(opts: { fetchWritesResult?: boolean } = {}) {
  const calls: string[] = []
  const journey = createValuationJourney<{ input: string }, SubmitResult, Detail>(
    {
      submit: async payload => {
        calls.push(`submit:${payload.input}`)
        return { id: 7, value: 'created' }
      },
      fetch: async id => {
        calls.push(`fetch:${id}`)
        return { id, detail: 'loaded' }
      }
    },
    {
      idOfSubmit: r => r.id,
      idOfDetail: d => d.id,
      fetchWritesResult: opts.fetchWritesResult
    }
  )
  return { journey, calls }
}

describe('createValuationJourney（结果旅程收敛）', () => {
  it('submit 成功后写入 currentResult 与 currentId，loading 复位', async () => {
    const { journey } = makeJourney()

    const p = journey.submit({ input: 'a' })
    expect(journey.loading.value).toBe(true)
    await p

    expect(journey.loading.value).toBe(false)
    expect(journey.error.value).toBeNull()
    expect(journey.currentId.value).toBe(7)
    expect(journey.currentResult.value).toEqual({ id: 7, value: 'created' })
  })

  it('submit 失败写入 error 并向上抛出，loading 复位', async () => {
    const journey = createValuationJourney<{ input: string }, SubmitResult, Detail>(
      {
        submit: async () => {
          throw new Error('提交失败')
        },
        fetch: async id => ({ id, detail: 'loaded' })
      },
      { idOfSubmit: r => r.id, idOfDetail: d => d.id }
    )

    await expect(journey.submit({ input: 'a' })).rejects.toThrow('提交失败')
    expect(journey.loading.value).toBe(false)
    expect(journey.error.value).toBe('提交失败')
    expect(journey.currentResult.value).toBeNull()
  })

  it('fetch 成功写入 currentDetail 与 currentId', async () => {
    const { journey } = makeJourney()

    const detail = await journey.fetch(9)

    expect(detail).toEqual({ id: 9, detail: 'loaded' })
    expect(journey.currentDetail.value).toEqual(detail)
    expect(journey.currentId.value).toBe(9)
  })

  it('fetchWritesResult=true 时 fetch 也刷新 currentResult（残值评估路径）', async () => {
    const { journey } = makeJourney({ fetchWritesResult: true })

    await journey.fetch(9)

    expect(journey.currentResult.value).toEqual({ id: 9, detail: 'loaded' })
    expect(journey.currentDetail.value).toEqual({ id: 9, detail: 'loaded' })
  })

  it('fetch 失败写入 error 并向上抛出', async () => {
    const journey = createValuationJourney<{ input: string }, SubmitResult, Detail>(
      {
        submit: async () => ({ id: 1, value: 'x' }),
        fetch: async () => {
          throw new Error('加载失败')
        }
      },
      { idOfSubmit: r => r.id, idOfDetail: d => d.id }
    )

    await expect(journey.fetch(1)).rejects.toThrow('加载失败')
    expect(journey.error.value).toBe('加载失败')
    expect(journey.loading.value).toBe(false)
  })

  it('reset 清空全部状态', async () => {
    const { journey } = makeJourney()

    await journey.submit({ input: 'a' })
    journey.reset()

    expect(journey.currentId.value).toBeNull()
    expect(journey.currentResult.value).toBeNull()
    expect(journey.currentDetail.value).toBeNull()
    expect(journey.error.value).toBeNull()
  })
})
