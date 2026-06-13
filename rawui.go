package stdocs

// defaultUIDoc is the default docs page HTML, served at the docs
// prefix when no UI sub-package is imported. It is a small (~2 KB)
// dependency-free page — no CDN, no external assets — whose inline
// script fetches the JSON spec and renders the route list. It also
// shows a dismissable notice pointing at the richer UIs; the dismissal
// is remembered in localStorage. Operation keys are whitelisted (a path
// item's "parameters" array must not render as a method) and all
// spec-derived strings are inserted with textContent, never innerHTML.
// The page carries a strict Content-Security-Policy (see defaultDocsCSP)
// with the inline script and style pinned by hash, so it uses no inline
// style attributes — the notice is shown and hidden via the "hidden"
// attribute, not an inline style.
const defaultUIDoc = `<!doctype html>
<html><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>body{font-family:system-ui,sans-serif;max-width:900px;margin:2em auto;padding:0 1em}
h1{font-size:1.4em}code{background:#eee;padding:0 0.2em;border-radius:3px}
.method{font-weight:600;display:inline-block;min-width:4em}
.GET,.QUERY{color:#0a7}.POST{color:#08c}.PUT{color:#960}.DELETE{color:#c33}.PATCH{color:#960}
#stdocs-note{background:#eef4ff;border:1px solid #cdddff;border-radius:6px;padding:0.6em 0.8em;margin-bottom:1.4em;font-size:0.9em;color:#1c3050}
#stdocs-note:not([hidden]){display:flex;gap:0.6em;align-items:flex-start;justify-content:space-between}
#stdocs-note a{color:#0a58ca}
#stdocs-note button{flex:none;border:0;background:none;font-size:1.3em;line-height:1;cursor:pointer;color:#789;padding:0 0.1em}</style>
</head><body>
<div id="stdocs-note" hidden>
<span>You are viewing the minimal built-in docs UI. For a richer reference — Scalar, Swagger UI, Redoc, or Stoplight — import a UI sub-package and pass its <code>WithUI()</code> option, e.g. <code>stdocs.New(scalar.WithUI())</code>. See <a href="https://pkg.go.dev/github.com/FumingPower3925/stdocs#hdr-Docs_UIs" target="_blank" rel="noopener noreferrer">the docs</a>.</span>
<button type="button" id="stdocs-note-x" aria-label="Dismiss this notice">&times;</button>
</div>
<h1>{{.Title}}</h1>
<div id="out">Loading...</div>
<script>
(function(){
  var KEY='stdocs-docs-notice-dismissed';
  var note=document.getElementById('stdocs-note');
  var dismissed=false;
  try{dismissed=!!localStorage.getItem(KEY);}catch(e){}
  if(note&&!dismissed)note.hidden=false;
  var x=document.getElementById('stdocs-note-x');
  if(x)x.addEventListener('click',function(){
    if(note)note.hidden=true;
    try{localStorage.setItem(KEY,'1');}catch(e){}
  });
})();
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
