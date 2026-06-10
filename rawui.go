package stdocs

// defaultUIDoc is the default docs page HTML, served at the docs
// prefix when no UI sub-package is imported. It is a small (~1.5 KB)
// dependency-free page — no CDN, no external assets — whose inline
// script fetches the JSON spec and renders the route list. Operation
// keys are whitelisted (a path item's "parameters" array must not
// render as a method) and all spec-derived strings are inserted with
// textContent, never innerHTML.
const defaultUIDoc = `<!doctype html>
<html><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>body{font-family:system-ui,sans-serif;max-width:900px;margin:2em auto;padding:0 1em}
h1{font-size:1.4em}code{background:#eee;padding:0 0.2em;border-radius:3px}
.method{font-weight:600;display:inline-block;min-width:4em}
.GET,.QUERY{color:#0a7}.POST{color:#08c}.PUT{color:#960}.DELETE{color:#c33}.PATCH{color:#960}</style>
</head><body>
<h1>{{.Title}}</h1>
<div id="out">Loading...</div>
<script>
const METHODS=['get','put','post','delete','options','head','patch','trace','query'];
function row(out,method,path,op){
  const div=document.createElement('div');
  const m=document.createElement('span');
  m.className='method '+method.toUpperCase();
  m.textContent=method.toUpperCase();
  const c=document.createElement('code');
  c.textContent=path;
  div.appendChild(m);
  div.appendChild(document.createTextNode(' '));
  div.appendChild(c);
  if(op&&op.summary){div.appendChild(document.createTextNode(' — '+op.summary));}
  out.appendChild(div);
}
fetch('{{.SpecURL}}').then(r=>r.json()).then(spec=>{
  const out=document.getElementById('out');
  out.textContent='';
  for(const [path,item] of Object.entries(spec.paths||{})){
    const h=document.createElement('h2');h.textContent=path;out.appendChild(h);
    for(const m of METHODS){
      if(item[m])row(out,m,path,item[m]);
    }
    for(const extra of [item.additionalOperations,item['x-stdocs-additionalOperations']]){
      if(!extra)continue;
      for(const [m,op] of Object.entries(extra))row(out,m,path,op);
    }
  }
});
</script>
</body></html>`
