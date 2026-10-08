import { readFileSync, writeFileSync } from 'node:fs';
const p = 'D:/FL/.scratch/research/1545-doc-prose.md';
let t = readFileSync(p, 'utf8');
const startA = '三种「命令是全量面、但汇总行没进上下文」的机制';
const endA = '**S1 是唯一的「为什么」源**';
const i = t.indexOf(startA), j = t.indexOf(endA);
if (i < 0 || j < 0 || j <= i) throw new Error('锚点异常');
const block = `两类数字不能混：

- **111 次内部**有两种「绕路进上下文」：5 次命令带落盘重定向（\`… 2>&1 | Out-File "$log"\`），9 次由宿主后台的 \`.output\` 文件解析出汇总（\`src=bg:*\`）。它们的汇总行**最终仍进了上下文**，只是路径不同。后台那一类有个口径细节必须写下来：\`resultChars\` 记的是 jest 输出部分，**不含宿主包装那一行**（票 1532 实测块长 1227，首行 \`1113 C:\\Users\\ZHENG\\AppData\\Local\\Temp\\qoder-cli\\D--FL\\98e35be1-…\\tasks\\b2j5dnsu9.output\` 占 114 字符，1227 − 114 = 1113 与台账逐位相符）。
- **另有 13 次命令是全量面、汇总行没进上下文**（§2 用它定上界）。逐条形态在转录里可指名（\`.scratch/research/1545-orphans.out\` 留了每条的命令与结果块头部）：3 次带 \`Out-File\` 重定向、0 次走宿主后台、10 次是**stdout 被下游截走或改写**——\`--json --outputFile=…\`（汇总进了 json 文件，控制台块被截尾）、\`| Select-String\` / \`| grep -E\` / \`| tail -8\` / \`| tail -15\`（只回显片段）、\`foreach ($i in 4..5) { … }\` 与 \`for pat in …\`（循环把多次跑的回显拼成一坨）。#1410 的并行/串行实验两条就在这 13 里（§6.4）。

这 13 次不是「没跑」——它们中的 2 次在结果块里留下了 \`Time:\`（164.326 s / 164.691 s，合计 329.0 s），2 次能在盘上按时间对上（都是 #1410，见 §2 与 §6.4），其余只留下命令。

`;
t = t.slice(0, i) + block + t.slice(j);
writeFileSync(p, t);
console.log('替换完成，长度', t.length, '圈号 ①' + (t.match(/①/g) || []).length + ' ②' + (t.match(/②/g) || []).length + ' ③' + (t.match(/③/g) || []).length + ' ④' + (t.match(/④/g) || []).length + ' ⑤' + (t.match(/⑤/g) || []).length);
