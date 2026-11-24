import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { epLite } from '@/test/element-lite'
import PublicEligibility from '../PublicEligibility.vue'

describe('PublicEligibility', () => {
  it('renders the public result and every blocker', () => {
    const wrapper = mount(PublicEligibility, {
      props: {
        eligibility: {
          eligible: false,
          reasons: ['not_published', 'category_missing', 'source_tag_excluded']
        }
      },
      global: { plugins: [epLite()] }
    })

    expect(wrapper.text()).toContain('未公开')
    expect(wrapper.text()).toContain('未发布')
    expect(wrapper.text()).toContain('未设置培训分类')
    expect(wrapper.text()).toContain('来源标签题不计入公开题库')
  })

  it('does not render blocker text for eligible content', () => {
    const wrapper = mount(PublicEligibility, {
      props: { eligibility: { eligible: true, reasons: [] } },
      global: { plugins: [epLite()] }
    })

    expect(wrapper.text()).toContain('已公开')
    expect(wrapper.text()).not.toContain('未公开')
  })
})
