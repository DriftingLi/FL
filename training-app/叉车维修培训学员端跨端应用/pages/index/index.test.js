describe('欢迎页测试', () => {
  let page;

  beforeAll(async () => {
    // 重新 launch 到首页，并获取 page 对象
    page = await program.reLaunch('/');
    await page.waitFor(3000);
  });

  it('页面标题显示正确', async () => {
    const el = await page.$('.title');
    const titleText = await el.text();
    expect(titleText).toEqual('叉车维修培训系统');
  });

  it('副标题显示正确', async () => {
    const el = await page.$('.subtitle');
    const subtitleText = await el.text();
    expect(subtitleText).toEqual('专业维修技能成长平台');
  });

  it('页面标签显示正确', async () => {
    const el = await page.$('.page-tag-text');
    const tagText = await el.text();
    expect(tagText).toEqual('欢迎页');
  });
});