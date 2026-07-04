<template>
  <div class="portal-home">
    <!-- ===== Hero ===== -->
    <section id="hero" class="hero">
      <img src="/images/hero-bg.jpg" alt="" class="hero-bg" />
      <div class="hero-overlay"></div>
      <div class="hero-content">
        <p class="hero-tagline">AI赋能叉车行业</p>
        <h1 class="hero-title">和润天下 HRWAI</h1>
        <p class="hero-subtitle">
          和润天下人工智能科技有限公司 — 专注叉车残值评估与智能运维，用AI让每一台叉车的价值透明可见。
        </p>
        <div class="hero-cta">
          <a href="#about" class="btn-primary" @click.prevent="scrollTo('about')">了解我们</a>
          <a href="#products" class="btn-outline" @click.prevent="scrollTo('products')">核心服务</a>
        </div>
      </div>
    </section>

    <!-- ===== About ===== -->
    <section id="about" class="section about">
      <div class="container">
        <div class="section-title-wrap">
          <h2 class="section-title">公司介绍<span class="title-underline"></span></h2>
        </div>
        <div class="about-grid">
          <div class="about-image">
            <img src="/images/about.jpg" alt="和润天下公司办公环境" />
          </div>
          <div class="about-content">
            <p class="about-paragraph">
              和润天下人工智能科技有限公司是一家专注于叉车行业人工智能应用的高新技术企业。公司秉承"科技赋能、精准服务"的理念，致力于为叉车产业链上下游企业提供智能化、数字化的一站式解决方案。
            </p>
            <p class="about-paragraph">
              凭借深厚的技术积累与行业洞察，我们打造了涵盖叉车残值智能评估、叉车维修培训认证、二手叉车信息撮合交易、AI叉车智能助手等核心服务体系，帮助客户实现降本增效、科学决策，推动叉车行业迈向智能化新时代。
            </p>
            <div class="stat-grid">
              <div class="stat-card">
                <span class="stat-number">5+</span>
                <p class="stat-label">行业经验（年）</p>
              </div>
              <div class="stat-card">
                <span class="stat-number">20+</span>
                <p class="stat-label">覆盖品牌</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>

    <!-- ===== Founder ===== -->
    <section class="section founder">
      <div class="container narrow">
        <div class="founder-avatar">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="#fff" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round">
            <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2"/>
            <circle cx="12" cy="7" r="4"/>
          </svg>
        </div>
        <h3 class="founder-title">创始人介绍</h3>
        <p class="founder-name">刘永桂 · 和润天下创始人 &amp; 董事长</p>
        <div class="founder-quote-wrap">
          <svg class="quote-mark" width="60" height="48" viewBox="0 0 60 48" fill="none">
            <path d="M2 36C2 26 8 20 18 18L16 2L2 36ZM34 36C34 26 40 20 50 18L48 2L34 36Z" fill="#fff" opacity="0.15"/>
          </svg>
          <blockquote class="founder-quote">
            华南理工大学教授、博导，国家及省市级项目评审咨询专家，曾挂职江门市工信局副局长。深耕工业互联网与产业数字化转型领域，发表学术论文八十余篇，获专利及软著六十余项，主持国家级等各类项目二十余项。牵头研发多款智能监测系统与产业互联网平台并落地应用，创办多家企业并任董事长，带领硕博团队深耕产学研融合与科研成果产业化。
          </blockquote>
        </div>
      </div>
    </section>

    <!-- ===== Products ===== -->
    <section id="products" class="section products">
      <div class="container">
        <div class="section-title-wrap">
          <h2 class="section-title">核心服务<span class="title-underline"></span></h2>
        </div>
        <div class="products-grid">
          <div
            v-for="item in products"
            :key="item.title"
            class="service-card"
          >
            <div class="service-icon">
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" v-html="item.icon"></svg>
            </div>
            <h3 class="service-title">{{ item.title }}</h3>
            <p class="service-desc">{{ item.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ===== Cooperation ===== -->
    <section id="cooperation" class="section cooperation">
      <div class="container">
        <div class="section-title-wrap">
          <h2 class="section-title">合作模式<span class="title-underline"></span></h2>
        </div>
        <div class="cooperation-grid">
          <div v-for="item in cooperations" :key="item.title" class="coop-card">
            <div class="coop-icon">
              <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" v-html="item.icon"></svg>
            </div>
            <h3 class="coop-title">{{ item.title }}</h3>
            <ul class="coop-list">
              <li v-for="point in item.points" :key="point">
                <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round">
                  <polyline points="20 6 9 17 4 12"/>
                </svg>
                <span>{{ point }}</span>
              </li>
            </ul>
            <a href="#footer" class="btn-coop" @click.prevent="scrollTo('footer')">联系我们</a>
          </div>
        </div>
      </div>
    </section>

    <!-- ===== Service Guarantee ===== -->
    <section id="service" class="section service-guarantee">
      <div class="container">
        <div class="section-title-wrap">
          <h2 class="section-title">服务保障<span class="title-underline"></span></h2>
        </div>
        <div class="guarantee-grid">
          <div v-for="item in guarantees" :key="item.no" class="guarantee-item">
            <div class="guarantee-no">
              <span>{{ item.no }}</span>
            </div>
            <h3 class="guarantee-title">{{ item.title }}</h3>
            <p class="guarantee-desc">{{ item.desc }}</p>
          </div>
        </div>
      </div>
    </section>

    <!-- ===== CTA Banner ===== -->
    <section class="cta-banner">
      <div class="container narrow cta-content">
        <h2 class="cta-title">准备好开启AI驱动的叉车残值评估了吗？</h2>
        <p class="cta-subtitle">加入和润天下，获取一整套成熟的叉车行业AI解决方案。</p>
        <a href="#footer" class="btn-cta" @click.prevent="scrollTo('footer')">立即合作</a>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
const products = [
  {
    title: '叉车维修培训',
    desc: '专业叉车维修技能培训与认证服务，涵盖电动叉车、内燃叉车全系列机型，理论+实操一体化教学，帮助学员快速掌握核心技术，提升职业竞争力。',
    icon: '<path d="M2 3h6a4 4 0 0 1 4 4v14a3 3 0 0 0-3-3H2z"/><path d="M22 3h-6a4 4 0 0 0-4 4v14a3 3 0 0 1 3-3h7z"/>'
  },
  {
    title: '叉车残值评估',
    desc: '基于AI多维度加权模型的智能残值评估系统，精准量化二手叉车价值，输出专业评估报告与置信区间，为交易定价提供科学依据。',
    icon: '<path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/>'
  },
  {
    title: '二手叉车交易',
    desc: '一站式二手叉车信息撮合平台，整合车源信息与AI评估报告，消除信息不对称，让买卖双方高效对接、透明交易。',
    icon: '<circle cx="9" cy="21" r="1"/><circle cx="20" cy="21" r="1"/><path d="M1 1h4l2.68 13.39a2 2 0 0 0 2 1.61h9.72a2 2 0 0 0 2-1.61L23 6H6"/>'
  },
  {
    title: 'AI叉车助手',
    desc: '基于大语言模型的智能问答助手，随时解答叉车选购、维保、故障诊断等专业问题，降低行业知识门槛，提升决策效率。',
    icon: '<path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>'
  }
]

const cooperations = [
  {
    title: '叉车经销商',
    points: [
      '精准定价，提升二手车流转效率',
      '一键生成专业评估报告',
      '数据驱动采购与库存决策'
    ],
    icon: '<rect x="2" y="7" width="20" height="14" rx="2" ry="2"/><path d="M16 21V5a2 2 0 0 0-2-2h-4a2 2 0 0 0-2 2v16"/>'
  },
  {
    title: '维修服务商',
    points: [
      '智能故障诊断与维保建议',
      '标准化维修流程与质量管控',
      '提升客户信任度与服务效率'
    ],
    icon: '<circle cx="12" cy="12" r="3"/><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06a1.65 1.65 0 0 0-.33 1.82V9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z"/>'
  },
  {
    title: '保险金融机构',
    points: [
      '客观公正的资产评估依据',
      '降低承保风险，优化理赔流程',
      '支持融资租赁与资产证券化'
    ],
    icon: '<path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>'
  }
]

const guarantees = [
  { no: '01', title: '数据安全保障', desc: '企业级加密存储，严格的数据访问权限控制' },
  { no: '02', title: '模型精度保障', desc: '持续训练优化，评估准确率行业领先' },
  { no: '03', title: '7x24技术支持', desc: '全天候专业团队在线，快速响应技术需求' },
  { no: '04', title: '评估报告可追溯', desc: '完整评估链路记录，支持全流程审计追踪' },
  { no: '05', title: '定制化服务', desc: '根据业务场景灵活定制评估模型与报告模板' },
  { no: '06', title: '数据洞察分析', desc: '深度挖掘评估数据价值，提供行业趋势洞察' },
  { no: '07', title: '合规审计支持', desc: '符合行业监管标准，协助完成合规审计流程' },
  { no: '08', title: '持续迭代升级', desc: '定期更新算法模型，确保技术始终领先' }
]

function scrollTo(id: string) {
  const el = document.getElementById(id)
  if (el) {
    el.scrollIntoView({ behavior: 'smooth' })
  }
}
</script>

<style scoped>
.portal-home {
  background: var(--color-bg-card);
}

.container {
  max-width: var(--container-page);
  margin: 0 auto;
  padding: 0 var(--space-6);
}
.container.narrow {
  max-width: var(--container-narrow);
}

/* ============ Hero ============ */
.hero {
  position: relative;
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}
.hero-bg {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.hero-overlay {
  position: absolute;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: var(--gradient-hero-overlay);
}
.hero-content {
  position: relative;
  z-index: 1;
  max-width: var(--container-page);
  margin: 0 auto;
  padding: var(--space-32) var(--space-6) var(--space-16);
  text-align: center;
}
.hero-tagline {
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  color: var(--color-primary-200);
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin-bottom: var(--space-6);
}
.hero-title {
  font-family: var(--font-display);
  font-size: clamp(2.5rem, 6vw, 4.5rem);
  font-weight: var(--font-bold);
  line-height: var(--leading-tight);
  letter-spacing: -0.025em;
  color: #fff;
  margin: 0 auto var(--space-8);
  max-width: 800px;
  word-break: keep-all;
  overflow-wrap: break-word;
}
.hero-subtitle {
  font-size: clamp(var(--text-base), 2vw, var(--text-lg));
  line-height: var(--leading-relaxed);
  color: rgba(255, 255, 255, 0.85);
  max-width: 640px;
  margin: 0 auto var(--space-12);
}
.hero-cta {
  display: flex;
  gap: var(--space-4);
  justify-content: center;
  flex-wrap: wrap;
}
.btn-primary {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  padding: 12px 32px;
  background: var(--gradient-brand);
  color: #fff;
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  border-radius: var(--radius-md);
  text-decoration: none;
  border: none;
  cursor: pointer;
  transition: opacity var(--duration-fast), transform var(--duration-fast);
}
.btn-primary:hover {
  opacity: 0.9;
  transform: translateY(-1px);
}
.btn-outline {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  padding: 12px 32px;
  background: transparent;
  color: #fff;
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  border-radius: var(--radius-md);
  text-decoration: none;
  border: 2px solid rgba(255, 255, 255, 0.5);
  cursor: pointer;
  transition: border-color var(--duration-fast), background var(--duration-fast);
}
.btn-outline:hover {
  border-color: rgba(255, 255, 255, 0.8);
  background: rgba(255, 255, 255, 0.1);
}

/* ============ Section common ============ */
.section {
  padding: var(--section-padding) var(--space-6);
}
.section-title-wrap {
  text-align: center;
  margin-bottom: var(--space-16);
}
.section-title {
  font-family: var(--font-display);
  font-size: var(--text-3xl);
  font-weight: var(--font-bold);
  line-height: var(--leading-snug);
  color: var(--color-text-primary);
  margin: 0;
  position: relative;
  display: inline-block;
}
.title-underline {
  position: absolute;
  bottom: -8px;
  left: 0;
  right: 0;
  height: 3px;
  background: var(--gradient-brand);
  border-radius: 2px;
}

/* ============ About ============ */
.about {
  background: var(--color-bg-card);
}
.about-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-12);
  align-items: center;
}
.about-image {
  overflow: hidden;
  border-radius: var(--radius-lg);
  box-shadow: var(--shadow-lg);
}
.about-image img {
  width: 100%;
  height: auto;
  display: block;
  object-fit: cover;
  aspect-ratio: 16 / 10;
}
.about-paragraph {
  font-size: var(--text-base);
  line-height: var(--leading-relaxed);
  color: var(--color-text-secondary);
  margin: 0 0 var(--space-6);
}
.about-paragraph:last-of-type {
  margin-bottom: var(--space-10);
}
.stat-grid {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: var(--space-6);
}
.stat-card {
  text-align: center;
  padding: var(--space-5);
  background: var(--surface-card-alt);
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
}
.stat-number {
  font-family: var(--font-display);
  font-size: var(--text-4xl);
  font-weight: var(--font-bold);
  color: var(--color-primary-500);
  line-height: var(--leading-tight);
}
.stat-label {
  font-size: var(--text-sm);
  color: var(--color-text-tertiary);
  margin: var(--space-2) 0 0;
}

/* ============ Founder ============ */
.founder {
  background: var(--surface-dark);
  text-align: center;
  color: var(--color-text-on-dark);
}
.founder-avatar {
  width: 100px;
  height: 100px;
  border-radius: var(--radius-full);
  background: var(--gradient-brand);
  margin: 0 auto var(--space-8);
  display: flex;
  align-items: center;
  justify-content: center;
}
.founder-title {
  font-family: var(--font-display);
  font-size: var(--text-2xl);
  font-weight: var(--font-semibold);
  color: var(--color-primary-200);
  margin: 0 0 var(--space-2);
}
.founder-name {
  font-size: var(--text-sm);
  color: var(--color-text-muted);
  margin: 0 0 var(--space-10);
}
.founder-quote-wrap {
  position: relative;
}
.quote-mark {
  position: absolute;
  top: -20px;
  left: 50%;
  transform: translateX(-50%);
  opacity: 0.15;
}
.founder-quote {
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
  color: var(--color-text-on-dark);
  margin: 0;
  padding: var(--space-8) 0;
}

/* ============ Products ============ */
.products {
  background: var(--surface-card-alt);
}
.products-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-8);
}
.service-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-sm);
  padding: var(--space-8);
  transition: box-shadow var(--duration-normal), transform var(--duration-normal);
}
.service-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-2px);
}
.service-icon {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-md);
  background: var(--color-primary-50);
  color: var(--color-primary-500);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: var(--space-6);
}
.service-title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
  font-weight: var(--font-semibold);
  line-height: var(--leading-snug);
  color: var(--color-text-primary);
  margin: 0 0 var(--space-4);
}
.service-desc {
  font-size: var(--text-base);
  line-height: var(--leading-relaxed);
  color: var(--color-text-secondary);
  margin: 0;
}

/* ============ Cooperation ============ */
.cooperation {
  background: var(--surface-card-alt);
}
.cooperation-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-8);
}
.coop-card {
  background: var(--color-bg-card);
  border-radius: var(--radius-lg);
  border: 1px solid var(--color-border);
  box-shadow: var(--shadow-sm);
  padding: var(--space-8);
  text-align: center;
  transition: box-shadow var(--duration-normal), transform var(--duration-normal);
  display: flex;
  flex-direction: column;
  align-items: center;
  height: 100%;
}
.coop-card:hover {
  box-shadow: var(--shadow-lg);
  transform: translateY(-2px);
}
.coop-icon {
  width: 56px;
  height: 56px;
  border-radius: var(--radius-lg);
  background: var(--color-primary-50);
  color: var(--color-primary-500);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto var(--space-6);
}
.coop-title {
  font-family: var(--font-display);
  font-size: var(--text-xl);
  font-weight: var(--font-semibold);
  line-height: var(--leading-snug);
  color: var(--color-text-primary);
  margin: 0 0 var(--space-6);
}
.coop-list {
  list-style: none;
  padding: 0;
  margin: 0 0 var(--space-8);
  text-align: left;
  width: 100%;
  flex: 1;
}
.coop-list li {
  font-size: var(--text-base);
  line-height: var(--leading-relaxed);
  color: var(--color-text-secondary);
  padding: var(--space-2) 0;
  display: flex;
  gap: var(--space-3);
}
.coop-list svg {
  flex-shrink: 0;
  margin-top: 4px;
  color: var(--color-primary-500);
}
.btn-coop {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  padding: 12px 28px;
  border: 2px solid var(--color-primary-500);
  color: var(--color-primary-500);
  font-size: var(--text-sm);
  font-weight: var(--font-medium);
  border-radius: var(--radius-md);
  text-decoration: none;
  transition: background var(--duration-fast), color var(--duration-fast);
}
.btn-coop:hover {
  background: var(--color-primary-500);
  color: #fff;
}

/* ============ Service Guarantee ============ */
.service-guarantee {
  background: var(--color-bg-card);
}
.guarantee-grid {
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: var(--space-6);
}
.guarantee-item {
  text-align: center;
  padding: var(--space-6);
}
.guarantee-no {
  width: 48px;
  height: 48px;
  border-radius: var(--radius-full);
  background: var(--color-primary-50);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto var(--space-5);
}
.guarantee-no span {
  font-family: var(--font-display);
  font-size: var(--text-lg);
  font-weight: var(--font-bold);
  color: var(--color-primary-500);
}
.guarantee-title {
  font-family: var(--font-display);
  font-size: var(--text-base);
  font-weight: var(--font-semibold);
  line-height: var(--leading-snug);
  color: var(--color-text-primary);
  margin: 0 0 var(--space-3);
}
.guarantee-desc {
  font-size: var(--text-sm);
  line-height: var(--leading-relaxed);
  color: var(--color-text-tertiary);
  margin: 0;
}

/* ============ CTA Banner ============ */
.cta-banner {
  background: var(--gradient-cta-banner);
  padding: var(--space-20) var(--space-6);
}
.cta-content {
  text-align: center;
}
.cta-title {
  font-family: var(--font-display);
  font-size: clamp(var(--text-2xl), 4vw, var(--text-4xl));
  font-weight: var(--font-bold);
  line-height: var(--leading-snug);
  color: #fff;
  margin: 0 0 var(--space-6);
  word-break: keep-all;
  overflow-wrap: break-word;
}
.cta-subtitle {
  font-size: var(--text-lg);
  line-height: var(--leading-relaxed);
  color: rgba(255, 255, 255, 0.9);
  max-width: 560px;
  margin: 0 auto var(--space-10);
}
.btn-cta {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-height: 44px;
  padding: 12px 32px;
  border: 2px solid #fff;
  color: #fff;
  font-size: var(--text-base);
  font-weight: var(--font-medium);
  border-radius: var(--radius-md);
  text-decoration: none;
  transition: background var(--duration-fast);
}
.btn-cta:hover {
  background: rgba(255, 255, 255, 0.15);
}

/* ============ Responsive ============ */
@media (min-width: 768px) {
  .about-grid {
    grid-template-columns: 1fr 1fr;
  }
  .products-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .cooperation-grid {
    grid-template-columns: repeat(3, 1fr);
  }
}

@media (max-width: 1023px) {
  .guarantee-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 639px) {
  .section {
    padding: var(--section-padding-mobile) var(--space-4);
  }
  .products-grid,
  .cooperation-grid {
    grid-template-columns: 1fr;
  }
  .stat-grid {
    grid-template-columns: repeat(2, 1fr);
  }
  .hero-content {
    padding: var(--space-32) var(--space-4) var(--space-12);
  }
  .hero-cta {
    flex-direction: column;
    align-items: stretch;
  }
  .hero-cta .btn-primary,
  .hero-cta .btn-outline {
    width: 100%;
  }
  .section-title-wrap {
    margin-bottom: var(--space-10);
  }
}
</style>
