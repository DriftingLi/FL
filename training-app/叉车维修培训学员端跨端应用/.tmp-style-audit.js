const fs=require('fs'),path=require('path');
function walk(d,out=[]){for(const e of fs.readdirSync(d,{withFileTypes:true})){if(e.name.startsWith('.')||e.name==='node_modules'||e.name==='unpackage')continue;const p=path.join(d,e.name);if(e.isDirectory())walk(p,out);else if(e.name.endsWith('.uvue'))out.push(p);}return out;}
const files=walk('.');
const map=new Map();
for(const f of files){const src=fs.readFileSync(f,'utf8');const m=/<style[^>]*>([\s\S]*?)<\/style>/.exec(src);if(!m)continue;
 const re=/(^|\})\s*([.\w][^{}]*?)\{([^{}]*)\}/g;let x;while((x=re.exec(m[1]))!==null){const sel=x[2].trim();if(!sel.startsWith('.'))continue;const decl=x[3].replace(/\s+/g,' ').trim();const key=sel;if(!map.has(key))map.set(key,[]);map.get(key).push({f,decl});}}
let multi=0,diff=0;const diffList=[];
for(const [sel,arr] of map){const fs2=new Set(arr.map(a=>a.f));if(fs2.size>1){multi++;const ds=new Set(arr.map(a=>a.decl));if(ds.size>1){diff++;if(diffList.length<12)diffList.push(sel+' -> files='+[...fs2].slice(0,4).join('|')+' declVariants='+ds.size);}}}
console.log('selectors defined in >1 file:',multi,' with DIFFERING declarations:',diff);
console.log(diffList.join('\n'));
