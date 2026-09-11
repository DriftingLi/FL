/**
 * 上传路径正规化契约测试（#816 根因修复）
 *
 * 真机证据（Android / 小米文件管理选文件，2026-09-10 23:56）：
 *   [uploadFile] >>> POST .../api/contributions/upload-file
 *     filePath=content://com.android.fileexplorer.myprovider/external_files/Download/WeiXin/叉车维修相关培训分类.docx
 *   CursorWindow: Failed to read row 0, column 4294967295 from a window with 1 rows, 2 columns
 *   IllegalStateException: Couldn't read row 0, col -1 from CursorWindow
 *     at uts.sdk.modules.DCloudUniNetwork.UploadController.getFileInformation(index.kt:873)
 *   -> uni.uploadFile 解析 content:// 失败，HTTP 请求根本没发出
 *
 * 钉住的契约：
 * 1) api 层对 content:// 先落盘中转（copyFile），非 content:// 路径原样透传
 * 2) 中转文件传完即删（成功/失败/401 都要走到 cleanup）
 * 3) 纯函数：文件名提取、净化、哈希、目标路径（扩展名不可丢——后端白名单依赖它）
 * 4) 安全：中转路径不得含路径分隔符注入
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/**
 * 复刻 utils/uploadPath.uts 的纯函数（UTS 无法在 jest 下 require）。
 * 复刻与实现的同步由本文件末段的结构断言守着——实现改了而这里没跟，断言会红。
 */
const UPLOAD_TMP_DIR = 'upload-tmp';

const isContentUri = (p) => p.startsWith('content://');

const decodeSafe = (v) => {
  if (v.indexOf('%') < 0) return v;
  try { return decodeURIComponent(v); } catch { return v; }
};

const extractBaseName = (uri) => {
  let p = uri;
  const q = p.indexOf('?');
  if (q >= 0) p = p.substring(0, q);
  const h = p.indexOf('#');
  if (h >= 0) p = p.substring(0, h);
  const slash = p.lastIndexOf('/');
  return decodeSafe(slash >= 0 ? p.substring(slash + 1) : p);
};

const sanitizeBaseName = (name) => {
  let out = '';
  for (const c of name) {
    const ok = (c >= '0' && c <= '9') || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c === '.' || c === '-' || c === '_';
    out += ok ? c : '_';
  }
  if (out.length === 0) return 'file';
  if (out.length > 64) out = out.substring(out.length - 64);
  return out;
};

const shortHash = (v) => {
  let h = 2166136261;
  for (let i = 0; i < v.length; i++) {
    h ^= v.charCodeAt(i);
    h = Math.imul(h, 16777619) >>> 0;
  }
  return h.toString(16);
};

const buildTempFilePath = (userDataPath, sourcePath, seq) => {
  const safe = sanitizeBaseName(extractBaseName(sourcePath));
  const h = shortHash(sourcePath + '#' + seq.toString());
  return userDataPath + '/' + UPLOAD_TMP_DIR + '/u' + h + '_' + safe;
};

/** 真机日志里那条真实 URI */
const REAL_URI = 'content://com.android.fileexplorer.myprovider/external_files/Download/WeiXin/%E5%8F%89%E8%BD%A6%E7%BB%B4%E4%BF%AE%E7%9B%B8%E5%85%B3%E5%9F%B9%E8%AE%AD%E5%88%86%E7%B1%BB.docx';

describe('#816 content:// URI 识别', () => {
  it('识别 content:// 与真实路径', () => {
    expect(isContentUri(REAL_URI)).toBe(true);
    expect(isContentUri('content://media/external/file/1')).toBe(true);
    expect(isContentUri('/storage/emulated/0/Download/a.docx')).toBe(false);
    expect(isContentUri('file:///var/mobile/tmp/a.docx')).toBe(false);
    expect(isContentUri('')).toBe(false);
  });

  it('前缀匹配不误伤普通字符串（对照：含 content 但非 URI）', () => {
    expect(isContentUri('mycontent://x')).toBe(false);
    expect(isContentUri('https://x/content://y')).toBe(false);
  });
});

describe('#816 中转文件名推导', () => {
  it('从真实 URI 解出百分号编码的中文文件名', () => {
    expect(extractBaseName(REAL_URI)).toBe('叉车维修相关培训分类.docx');
  });

  it('剥掉 query 与 fragment', () => {
    expect(extractBaseName('content://p/a/b.docx?x=1&y=2')).toBe('b.docx');
    expect(extractBaseName('content://p/a/b.docx#frag')).toBe('b.docx');
  });

  it('非法百分号序列原样返回（不让文件名把整条上传链路带崩）', () => {
    expect(extractBaseName('content://p/%E4%B8')).toBe('%E4%B8');
    expect(extractBaseName('content://p/a%zz.docx')).toBe('a%zz.docx');
  });

  it('净化只保留 [A-Za-z0-9._-]（中文与全角标点转下划线）', () => {
    expect(sanitizeBaseName('叉车维修相关培训分类.docx')).toBe('__________.docx');
    expect(sanitizeBaseName('a b:c*d?.pdf')).toBe('a_b_c_d_.pdf');
    expect(sanitizeBaseName('')).toBe('file');
    expect(sanitizeBaseName('中文')).toBe('__');
  });

  it('超长名截断仍保留扩展名', () => {
    const long = 'x'.repeat(200) + '.docx';
    const r = sanitizeBaseName(long);
    expect(r.length).toBe(64);
    expect(r.endsWith('.docx')).toBe(true);
  });

  it('哈希稳定且区分不同来源', () => {
    expect(shortHash('a')).toBe(shortHash('a'));
    expect(shortHash('a')).not.toBe(shortHash('b'));
    expect(shortHash(REAL_URI)).toMatch(/^[0-9a-f]{1,8}$/);
  });

  it('目标路径落在 upload-tmp 下、保留扩展名、无路径分隔符注入', () => {
    const p = buildTempFilePath('/data/user/0/app/files', REAL_URI, 1757520000000);
    expect(p.startsWith('/data/user/0/app/files/upload-tmp/u')).toBe(true);
    expect(p.endsWith('.docx')).toBe(true);
    // 文件名段（最后一段）不得再含分隔符——净化须掐断二级路径注入
    const segs = p.split('/');
    const fileName = segs[segs.length - 1];
    expect(segs[segs.length - 2]).toBe('upload-tmp');
    expect(fileName.includes('\\')).toBe(false);
    expect(fileName.split('/').length).toBe(1);
    expect(p.includes('..')).toBe(false);
    expect(p.includes('%')).toBe(false);
    expect(p.includes(' ')).toBe(false);
  });

  it('seq 不同则目标路径不同（压掉同名文件冲突）', () => {
    const a = buildTempFilePath('/u', REAL_URI, 1);
    const b = buildTempFilePath('/u', REAL_URI, 2);
    expect(a).not.toBe(b);
  });
});

describe('#816 上传层接入（api/request.uts）', () => {
  const src = read('api/request.uts');

  it('导入路径工具并按 content:// 分派', () => {
    expect(src).toMatch(/import \{ isContentUri, buildTempFilePath, UPLOAD_TMP_DIR \} from '\.\.\/utils\/uploadPath'/);
    expect(src).toMatch(/if \(!isContentUri\(path\)\) return path/);
  });

  it('中转走 getFileSystemManager().copyFile（srcPath/destPath）', () => {
    expect(src).toContain('uni.getFileSystemManager()');
    expect(src).toContain('srcPath: path');
    expect(src).toContain('destPath: destPath');
  });

  it('不引 uni 命名类型（对齐全仓惯例，避免类型名不可解析而编译失败）', () => {
    for (const t of ['CopyFileOptions', 'RemoveFileOptions', 'IFileSystemManagerFail']) {
      expect(src.includes(t)).toBe(false);
    }
  });

  it('中转代码全部锁在 APP-ANDROID 块内（非 Android 端整段剔除）', () => {
    // 每个文件系统 API 引用都必须出现在 #ifdef APP-ANDROID 与对应 #endif 之间
    const lines = src.split('\n');
    let inBlock = false;
    const offenders = [];
    lines.forEach((ln, i) => {
      if (/^\s*\/\/\s*#ifdef\s+APP-ANDROID/.test(ln)) { inBlock = true; return; }
      if (/^\s*\/\/\s*#endif/.test(ln)) { inBlock = false; return; }
      // 只扫代码行：注释（// 与块注释的 * 行）不算引用
      const trimmed = ln.trim();
      const isComment = trimmed.startsWith('//') || trimmed.startsWith('*') || trimmed.startsWith('/*');
      if (!isComment && /getFileSystemManager|USER_DATA_PATH/.test(ln) && !inBlock) {
        offenders.push(`第 ${i + 1} 行: ${trimmed}`);
      }
    });
    expect(offenders).toEqual([]);
  });

  it('中转目录取 USER_DATA_PATH 下可写目录', () => {
    expect(src).toContain('uni.env.USER_DATA_PATH');
  });

  it('中转文件传完即删，且成功/失败/401 三条路径都走到清理', () => {
    expect(src).toMatch(/function removeTempFile\(path : string\) : void/);
    expect(src).toContain('unlink(');
    const doneCalls = src.match(/done\(\)/g) || [];
    expect(doneCalls.length).toBeGreaterThanOrEqual(3);
  });

  it('中转失败不静默：抛出「读取所选文件失败」而非笼统网络错误', () => {
    expect(src).toContain('读取所选文件失败');
  });

  it('上传端点/字段/超时未被本次改动波及（回归锁）', () => {
    const api = read('api/contribution.uts');
    expect(api).toContain("url: '/contributions/upload-file'");
    expect(api).toMatch(/timeout: 120000/);
    expect(src).toContain('name: name,');
    expect(src).toContain('timeout: timeout,');
  });
});

describe('#816 复刻函数与 UTS 实现同步（防两处漂移）', () => {
  const uts = read('utils/uploadPath.uts');

  it('常量与阈值一致', () => {
    expect(uts).toContain("export const UPLOAD_TMP_DIR = 'upload-tmp'");
    expect(uts).toContain('out.length > 64');
    expect(uts).toContain("return 'file'");
  });

  it('导出面与哈希初值一致', () => {
    for (const fn of ['isContentUri', 'extractBaseName', 'sanitizeBaseName', 'shortHash', 'buildTempFilePath']) {
      expect(uts).toMatch(new RegExp('export function ' + fn + '\\('));
    }
    expect(uts).toContain('2166136261');
    expect(uts).toContain('16777619');
  });

  it('目标路径拼接形态一致', () => {
    expect(uts).toContain("return userDataPath + '/' + UPLOAD_TMP_DIR + '/u' + h + '_' + safe");
  });
});