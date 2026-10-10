const automator = require('miniprogram-automator');
(async () => {
  const mp = await automator.connect({ wsEndpoint: 'ws://127.0.0.1:9421' });
  for (const route of ['pages/ai-assistant/ai-assistant', 'pages/profile/profile']) {
    try {
      // tabBar 页用 switchTab 若不行则 reLaunch
      let page;
      try { page = await mp.switchTab('/' + route); }
      catch (e) { page = await mp.reLaunch('/' + route); }
      await new Promise(r => setTimeout(r, 3500));
      page = await mp.currentPage();
      await page.waitFor(500);
      const path = 'D:/FL/wt-1602/training-app/叉车维修培训学员端跨端应用/.ci-verify/tab-' + route.split('/').pop() + '.png';
      await mp.screenshot({ path });
      console.log('SHOT', route, '->', path, 'stack=', JSON.stringify(await mp.pageStack()));
    } catch (e) { console.log('ERR', route, e.message); }
  }
  await mp.disconnect();
})().catch(e => { console.error('FATAL', e.message); process.exit(1); });

