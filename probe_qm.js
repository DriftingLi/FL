const https = require('https');
const http = require('http');
const fs = require('fs');

const candidates = [
  'https://webcdn.m.qq.com/spcmgr/download/QQPCMgrSetup.exe',
  'https://pcqq.qpic.cn/qqpc_mgr/QQPCMgrSetup.exe',
  'https://dlied6.qq.com/invc/qq/qqpcmgr/QQPCMgr_18.0.29898.211.exe',
  'https://dlied6.qq.com/invc/qq/qqpcmgr/latest/QQPCMgrSetup.exe'
];
const out = 'E:\\FL\\QQPCMgrSetup_download.exe';

function probe(u) {
  return new Promise((resolve) => {
    const mod = u.startsWith('https') ? https : http;
    const req = mod.get(u, { headers: { 'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120 Safari/537.36', 'Referer': 'https://guanjia.qq.com/' } }, res => {
      const len = res.headers['content-length'];
      console.log('OK  ' + u + ' [' + res.statusCode + '] length=' + (len ? (len/1048576).toFixed(1) + 'MB' : '?'));
      res.resume();
      resolve(res.statusCode === 200);
    });
    req.on('error', e => { console.log('FAIL ' + u + ' -> ' + e.code + ' ' + e.message); resolve(false); });
    req.setTimeout(15000, () => { req.destroy(); });
  });
}

(async () => {
  for (const c of candidates) {
    const ok = await probe(c);
    const ml = await new Promise(r => setTimeout(r, 300));
  }
})();