// #1602 取证探针：文字化查当前页 path / 证件浮层数据态 / 页头元素，回答「首页只剩证件浮层？」
const automator = require('miniprogram-automator');
(async () => {
  const mp = await automator.connect({ wsEndpoint: 'ws://127.0.0.1:9420' });
  const show = async (tag) => {
    const page = await mp.currentPage();
    let d = {};
    try { d.showCertDropdown = await page.data('showCertDropdown'); } catch (e) { d.showCertDropdown = 'ERR:' + e.message; }
    try { d.currentCert = await page.data('currentCert'); } catch (e) { d.currentCert = 'ERR:' + e.message; }
    const mask = await page.$('.cert-dropdown-mask');
    const navBody = await page.$('.nav-bar-body');
    const navTitle = await page.$('.nav-bar-title');
    const right = await page.$('.nav-bar-right-wrap');
    console.log(`PROBE[${tag}] path=${page.path} showCertDropdown=${d.showCertDropdown} currentCert=${d.currentCert} mask=${!!mask} navBody=${!!navBody} navTitle=${!!navTitle} rightWrap=${!!right}`);
  };
  await new Promise((r) => setTimeout(r, 8000));
  await show('coldstart');
  try {
    await mp.switchTab('/pages/dashboard/dashboard');
    await new Promise((r) => setTimeout(r, 5000));
  } catch (e) { console.log('switchTab fail:', e.message); }
  await show('after-switchTab-dashboard');
  await mp.screenshot({ path: 'D:/FL/wt-1602/training-app/叉车维修培训学员端跨端应用/.ci-verify/tab-dashboard3.png' });
  await mp.disconnect();
  console.log('DONE');
})().catch((e) => { console.error('FATAL', e.message); process.exit(1); });
