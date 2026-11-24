import { describe, expect, it } from 'vitest'
import type { Eligibility, Reason } from '@/api/generated/course'
import { describePublicEligibility, describePublicReason, hasLegacyCategoryReason } from '../publicEligibility'

describe('publicEligibility', () => {
  it('distinguishes workflow status from public eligibility', () => {
    const eligible: Eligibility = { eligible: true, reasons: [] }
    const blocked: Eligibility = { eligible: false, reasons: ['not_published', 'category_missing'] }
    expect(describePublicEligibility(eligible)).toEqual({ label: '已公开', tone: 'success' })
    expect(describePublicEligibility(blocked)).toEqual({ label: '未公开', tone: 'warning' })
  })

  it('maps every stable reason code to Chinese wording', () => {
    const reasons: Reason[] = [
      'not_published',
      'category_missing',
      'category_disabled',
      'category_unmapped',
      'credential_missing',
      'credential_disabled',
      'source_tag_excluded'
    ]
    for (const reason of reasons) {
      expect(describePublicReason(reason).label).not.toContain('未知')
    }
  })

  it('recognizes the legacy category bucket without treating workflow blockers as legacy', () => {
    expect(hasLegacyCategoryReason(['category_missing'])).toBe(true)
    expect(hasLegacyCategoryReason(['credential_disabled'])).toBe(true)
    expect(hasLegacyCategoryReason(['not_published', 'source_tag_excluded'])).toBe(false)
    expect(hasLegacyCategoryReason(undefined)).toBe(false)
  })
})
