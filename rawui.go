package stdocs

// defaultUIDoc is the default docs page HTML, served at the docs prefix
// when no UI sub-package is imported. It is a small (~10 KB)
// dependency-free page — no CDN, no external assets — whose inline
// script fetches the JSON spec and renders the operations grouped by
// path, with method badges, an auth padlock on secured operations, and
// a marker on deprecated ones. Clicking an operation expands a panel
// with its description, parameters, and a generated JSON example of the
// request body and each response (schemas resolved through their $refs,
// recursion-capped). It also shows a dismissable notice that it is the
// minimal built-in UI and points at the richer ones; the dismissal is
// remembered in localStorage.
//
// All spec-derived strings are inserted with textContent, never
// innerHTML; operation keys are whitelisted (a path item's "parameters"
// array must not render as a method). The page carries a strict
// Content-Security-Policy (see defaultDocsCSP) with the inline script
// and style pinned by hash, so it uses no inline style attributes: the
// notice toggles via the "hidden" attribute, and the padlock and chevron
// are static inline <svg> elements cloned from <template>s, not emoji or
// injected strings.
const defaultUIDoc = `<!doctype html>
<html><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;max-width:920px;margin:2em auto;padding:0 1em;color:#1c1c1c;line-height:1.5}
h1{font-size:1.5em;margin:0 0 .15em}
.meta{color:#777;font-size:.88em}.meta a{color:#0a58ca;text-decoration:none}.meta a:hover{text-decoration:underline}
.desc{color:#444;margin:.7em 0 0;font-size:.96em}
h2{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:1.02em;margin:1.6em 0 .25em;color:#111;border-bottom:1px solid #eee;padding-bottom:.25em}
.op{display:flex;align-items:center;gap:.55em;padding:.42em .3em;border-radius:5px;cursor:pointer;user-select:none}
.op:hover{background:#f4f6f9}
.op.dep{opacity:.65}
.chev{width:11px;height:11px;color:#aab;flex:none;transition:transform .12s}
.op.open .chev{transform:rotate(90deg)}
.m{font-weight:700;font-size:.72em;min-width:3.4em;text-align:center;padding:.22em .3em;border-radius:4px;color:#fff;letter-spacing:.02em}
.GET,.QUERY{background:#16a34a}.POST{background:#0284c7}.PUT{background:#b45309}.PATCH{background:#ca8a04}.DELETE{background:#dc2626}.OTHER{background:#6b7280}
.sum{color:#555;font-size:.93em}
.tag{margin-left:auto;display:flex;gap:.5em;align-items:center}
.pill{font-size:.66em;text-transform:uppercase;letter-spacing:.04em;padding:.12em .45em;border-radius:10px;background:#fde8ec;color:#b3324f;white-space:nowrap}
.lock{width:14px;height:14px;color:#9098a8;flex:none}
.detail{margin:.1em 0 .6em 1.9em;padding:.2em 0 .4em .9em;border-left:2px solid #eef;font-size:.9em}
.ddesc{color:#444;margin:.2em 0 .6em}
.detail h3{font-size:.72em;text-transform:uppercase;letter-spacing:.05em;color:#999;margin:.7em 0 .25em}
.param{margin:.15em 0}.param code{background:#f1f3f5;padding:0 .25em;border-radius:3px;font-size:.92em}
.pmeta{color:#888;font-size:.86em}.pdesc{color:#666;font-size:.86em;margin:.05em 0 .3em .2em}
.ct{color:#888;font-size:.8em;margin:.3em 0 .1em}
.resp{margin:.35em 0 .1em}.code{font-weight:700;font-family:ui-monospace,monospace}.rdesc{color:#555}
pre{background:#0f172a;color:#e2e8f0;border-radius:6px;padding:.7em .9em;overflow:auto;font-size:.84em;margin:.15em 0 .4em;line-height:1.45}
#stdocs-note{background:#eef4ff;border:1px solid #cdddff;border-radius:6px;padding:0.6em 0.8em;margin-bottom:1.4em;font-size:0.9em;color:#1c3050}
#stdocs-note:not([hidden]){display:flex;gap:0.6em;align-items:flex-start;justify-content:space-between}
#stdocs-note a{color:#0a58ca}#stdocs-note code{background:#dde7fb;padding:0 .25em;border-radius:3px}
#stdocs-note button{flex:none;border:0;background:none;font-size:1.3em;line-height:1;cursor:pointer;color:#789;padding:0 0.1em}
</style></head><body>
<template id="lock-tmpl"><svg class="lock" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" role="img"><title>Requires authentication</title><rect x="4.5" y="10.5" width="15" height="9.5" rx="2"></rect><path d="M8 10.5V7a4 4 0 0 1 8 0v3.5"></path></svg></template>
<template id="chev-tmpl"><svg class="chev" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.6" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M9 6l6 6-6 6"></path></svg></template>
<div id="stdocs-note" hidden>
<span>You are viewing the minimal built-in docs UI. For a richer reference — Scalar, Swagger UI, Redoc, or Stoplight — import a UI sub-package and pass its <code>WithUI()</code> option, e.g. <code>stdocs.New(scalar.WithUI())</code>. See <a href="https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Docs_UIs" target="_blank" rel="noopener noreferrer">the docs</a>.</span>
<button type="button" id="stdocs-note-x" aria-label="Dismiss this notice">&times;</button>
</div>
<h1>{{.Title}}</h1>
<div class="meta"><span id="ver"></span><span id="sep" hidden> · </span><a href="openapi.json">openapi.json</a> · <a href="openapi.yaml">openapi.yaml</a></div>
<p class="desc" id="desc" hidden></p>
<div id="out">Loading…</div>
<script>
(function(){var K='stdocs-docs-notice-dismissed',n=document.getElementById('stdocs-note'),d=false;try{d=!!localStorage.getItem(K);}catch(e){}if(n&&!d)n.hidden=false;var x=document.getElementById('stdocs-note-x');if(x)x.addEventListener('click',function(){if(n)n.hidden=true;try{localStorage.setItem(K,'1');}catch(e){}});})();
const METHODS=['get','put','post','delete','options','head','patch','trace','query'];
const KNOWN=['GET','PUT','POST','DELETE','PATCH','QUERY'];
var SPEC=null;
function tmpl(id){return document.getElementById(id).content.firstElementChild.cloneNode(true);}
function badge(method){var s=document.createElement('span');var M=method.toUpperCase();s.className='m '+(KNOWN.indexOf(M)>=0?M:'OTHER');s.textContent=M;return s;}
function deref(s){if(s&&s.$ref){var nm=s.$ref.split('/').pop();return{name:nm,s:(SPEC.components&&SPEC.components.schemas&&SPEC.components.schemas[nm])||{}};}return{name:null,s:s||{}};}
function example(schema,depth,seen){
  if(depth>6)return null;
  var d=deref(schema),s=d.s;
  if(d.name){if(seen.indexOf(d.name)>=0)return{};seen=seen.concat([d.name]);}
  if(s.example!==undefined)return s.example;
  if(s.default!==undefined)return s.default;
  if(s.enum&&s.enum.length)return s.enum[0];
  var comp=s.allOf||s.oneOf||s.anyOf;
  if(comp){for(var i=0;i<comp.length;i++){if(deref(comp[i]).s.type!=='null')return example(comp[i],depth,seen);}return null;}
  var t=s.type;if(Array.isArray(t))t=t.filter(function(x){return x!=='null';})[0];
  if(t==='object'||s.properties){var o={},p=s.properties||{};for(var k in p)o[k]=example(p[k],depth+1,seen);return o;}
  if(t==='array')return [example(s.items||{},depth+1,seen)];
  if(t==='string'){if(s.format==='date-time')return '2024-01-15T09:30:00Z';if(s.format==='date')return '2024-01-15';if(s.format==='byte')return 'c3Rkb2Nz';return 'string';}
  if(t==='integer'||t==='number')return s.minimum!==undefined?s.minimum:0;
  if(t==='boolean')return false;
  return null;
}
function typeLabel(s){var d=deref(s);if(d.name)return d.name;var t=d.s.type;if(Array.isArray(t))t=t.filter(function(x){return x!=='null';})[0];if(t==='array'){var it=deref(d.s.items||{});return (it.name||it.s.type||'item')+'[]';}return d.s.format?t+' ('+d.s.format+')':(t||'');}
function jsonPre(val){var pre=document.createElement('pre');pre.textContent=JSON.stringify(val,null,2);return pre;}
function h3(t){var e=document.createElement('h3');e.textContent=t;return e;}
function bodyBlock(parent,content){Object.keys(content).forEach(function(ct){var l=document.createElement('div');l.className='ct';l.textContent=ct;parent.appendChild(l);var sc=content[ct].schema;if(sc)parent.appendChild(jsonPre(example(sc,0,[])));});}
function buildDetail(op){
  var d=document.createElement('div');d.className='detail';
  if(op.description){var p=document.createElement('p');p.className='ddesc';p.textContent=op.description;d.appendChild(p);}
  if(op.parameters&&op.parameters.length){
    d.appendChild(h3('Parameters'));
    op.parameters.forEach(function(pm){
      var row=document.createElement('div');row.className='param';
      var nm=document.createElement('code');nm.textContent=pm.name;row.appendChild(nm);
      var meta=document.createElement('span');meta.className='pmeta';var ty=pm.schema?typeLabel(pm.schema):'';
      meta.textContent=' '+pm['in']+(ty?' · '+ty:'')+(pm.required?' · required':'');row.appendChild(meta);
      if(pm.description){var ds=document.createElement('div');ds.className='pdesc';ds.textContent=pm.description;row.appendChild(ds);}
      d.appendChild(row);
    });
  }
  if(op.requestBody&&op.requestBody.content){d.appendChild(h3('Request body'));bodyBlock(d,op.requestBody.content);}
  if(op.responses){
    d.appendChild(h3('Responses'));
    Object.keys(op.responses).forEach(function(code){
      var r=op.responses[code];var line=document.createElement('div');line.className='resp';
      var c=document.createElement('span');c.className='code';c.textContent=code;line.appendChild(c);
      if(r.description){var ds=document.createElement('span');ds.className='rdesc';ds.textContent=' — '+r.description;line.appendChild(ds);}
      d.appendChild(line);
      if(r.content)bodyBlock(d,r.content);
    });
  }
  return d;
}
function opItem(method,op){
  var wrap=document.createElement('div');
  var dep=op&&op.deprecated;
  var row=document.createElement('div');row.className='op clickable'+(dep?' dep':'');row.tabIndex=0;
  row.appendChild(tmpl('chev-tmpl'));
  row.appendChild(badge(method));
  if(op&&op.summary){var s=document.createElement('span');s.className='sum';s.textContent=op.summary;row.appendChild(s);}
  var t=document.createElement('span');t.className='tag';
  if(op&&op.security)t.appendChild(tmpl('lock-tmpl'));
  if(dep){var dp=document.createElement('span');dp.className='pill';dp.textContent='deprecated';t.appendChild(dp);}
  row.appendChild(t);
  var detail=null,open=false;
  function toggle(){if(!detail){detail=buildDetail(op);detail.hidden=true;wrap.appendChild(detail);}open=!open;detail.hidden=!open;row.classList.toggle('open',open);}
  row.addEventListener('click',toggle);
  row.addEventListener('keydown',function(e){if(e.key==='Enter'||e.key===' '){e.preventDefault();toggle();}});
  wrap.appendChild(row);
  return wrap;
}
fetch('{{.SpecURL}}').then(r=>r.json()).then(spec=>{
  SPEC=spec;var info=spec.info||{};
  if(info.version){document.getElementById('ver').textContent='v'+info.version;document.getElementById('sep').hidden=false;}
  if(info.description){var dd=document.getElementById('desc');dd.textContent=info.description;dd.hidden=false;}
  var out=document.getElementById('out');out.textContent='';
  for(const [path,item] of Object.entries(spec.paths||{})){
    var h=document.createElement('h2');h.textContent=path;out.appendChild(h);
    for(const m of METHODS){if(item[m])out.appendChild(opItem(m,item[m]));}
    for(const extra of [item.additionalOperations,item['x-stdocs-additionalOperations']]){if(!extra)continue;for(const [m,op] of Object.entries(extra))out.appendChild(opItem(m,op));}
  }
});
</script>
</body></html>`
