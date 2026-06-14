package stdocs

// defaultUIDoc is the default docs page HTML, served at the docs prefix
// when no UI sub-package is imported. It is a small (~4 KB)
// dependency-free page — no CDN, no external assets — whose inline
// script fetches the JSON spec and renders the operations grouped by
// path, with method badges, an auth padlock on secured operations, and
// a marker on deprecated ones. It also shows a dismissable notice that
// it is the minimal built-in UI and points at the richer ones; the
// dismissal is remembered in localStorage. Operation keys are
// whitelisted (a path item's "parameters" array must not render as a
// method) and all spec-derived strings are inserted with textContent,
// never innerHTML. The page carries a strict Content-Security-Policy
// (see defaultDocsCSP) with the inline script and style pinned by hash,
// so it uses no inline style attributes: the notice is shown and hidden
// via the "hidden" attribute, and the padlock is a static inline <svg>
// cloned from a <template>, not an emoji or an injected string.
const defaultUIDoc = `<!doctype html>
<html><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;max-width:920px;margin:2em auto;padding:0 1em;color:#1c1c1c;line-height:1.5}
h1{font-size:1.5em;margin:0 0 .15em}
.meta{color:#777;font-size:.88em}.meta a{color:#0a58ca;text-decoration:none}.meta a:hover{text-decoration:underline}
.desc{color:#444;margin:.7em 0 0;font-size:.96em}
h2{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:1.02em;margin:1.6em 0 .25em;color:#111;border-bottom:1px solid #eee;padding-bottom:.25em}
.op{display:flex;align-items:center;gap:.6em;padding:.4em .3em;border-radius:5px}
.op:hover{background:#f7f8fa}
.op.dep{opacity:.65}
.m{font-weight:700;font-size:.72em;min-width:3.4em;text-align:center;padding:.22em .3em;border-radius:4px;color:#fff;letter-spacing:.02em}
.GET,.QUERY{background:#16a34a}.POST{background:#0284c7}.PUT{background:#b45309}.PATCH{background:#ca8a04}.DELETE{background:#dc2626}.OTHER{background:#6b7280}
.sum{color:#555;font-size:.93em}
.tag{margin-left:auto;display:flex;gap:.5em;align-items:center}
.pill{font-size:.66em;text-transform:uppercase;letter-spacing:.04em;padding:.12em .45em;border-radius:10px;background:#fde8ec;color:#b3324f;white-space:nowrap}
.lock{width:14px;height:14px;color:#9098a8;flex:none}
#stdocs-note{background:#eef4ff;border:1px solid #cdddff;border-radius:6px;padding:0.6em 0.8em;margin-bottom:1.4em;font-size:0.9em;color:#1c3050}
#stdocs-note:not([hidden]){display:flex;gap:0.6em;align-items:flex-start;justify-content:space-between}
#stdocs-note a{color:#0a58ca}#stdocs-note code{background:#dde7fb;padding:0 .25em;border-radius:3px}
#stdocs-note button{flex:none;border:0;background:none;font-size:1.3em;line-height:1;cursor:pointer;color:#789;padding:0 0.1em}
</style></head><body>
<template id="lock-tmpl"><svg class="lock" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" role="img"><title>Requires authentication</title><rect x="4.5" y="10.5" width="15" height="9.5" rx="2"></rect><path d="M8 10.5V7a4 4 0 0 1 8 0v3.5"></path></svg></template>
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
function badge(method){var s=document.createElement('span');var M=method.toUpperCase();s.className='m '+(KNOWN.indexOf(M)>=0?M:'OTHER');s.textContent=M;return s;}
function lock(){return document.getElementById('lock-tmpl').content.firstElementChild.cloneNode(true);}
function opRow(method,op){
  var dep=op&&op.deprecated;
  var row=document.createElement('div');row.className='op'+(dep?' dep':'');
  row.appendChild(badge(method));
  if(op&&op.summary){var s=document.createElement('span');s.className='sum';s.textContent=op.summary;row.appendChild(s);}
  var t=document.createElement('span');t.className='tag';
  if(op&&op.security)t.appendChild(lock());
  if(dep){var dp=document.createElement('span');dp.className='pill';dp.textContent='deprecated';t.appendChild(dp);}
  row.appendChild(t);
  return row;
}
fetch('{{.SpecURL}}').then(r=>r.json()).then(spec=>{
  var info=spec.info||{};
  if(info.version){document.getElementById('ver').textContent='v'+info.version;document.getElementById('sep').hidden=false;}
  if(info.description){var dd=document.getElementById('desc');dd.textContent=info.description;dd.hidden=false;}
  var out=document.getElementById('out');out.textContent='';
  for(const [path,item] of Object.entries(spec.paths||{})){
    var h=document.createElement('h2');h.textContent=path;out.appendChild(h);
    for(const m of METHODS){if(item[m])out.appendChild(opRow(m,item[m]));}
    for(const extra of [item.additionalOperations,item['x-stdocs-additionalOperations']]){
      if(!extra)continue;
      for(const [m,op] of Object.entries(extra))out.appendChild(opRow(m,op));
    }
  }
});
</script>
</body></html>`
