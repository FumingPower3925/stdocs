// Package stdocs provides zero-dependency OpenAPI 3.0.3 and 3.1.0 documentation
// generation for a stdlib net/http.ServeMux (Go 1.22+).
package stdocs

// defaultUIDoc is the default docs page HTML, served at the docs prefix
// when no UI sub-package is imported. It is a zero-JS page that fetches
// the JSON spec and renders it as a simple list.
const defaultUIDoc = `<!doctype html>
<html><head><meta charset="utf-8"><title>{{.Title}}</title>
<style>body{font-family:system-ui,sans-serif;max-width:900px;margin:2em auto;padding:0 1em}
h1{font-size:1.4em}code{background:#eee;padding:0 0.2em;border-radius:3px}
table{border-collapse:collapse;width:100%}th,td{text-align:left;padding:0.4em;border-bottom:1px solid #ddd}
.method{font-weight:600;display:inline-block;min-width:4em}
.GET{color:#0a7}.POST{color:#08c}.PUT{color:#960}.DELETE{color:#c33}.PATCH{color:#960}
pre{background:#f4f4f4;padding:0.6em;overflow:auto;border-radius:4px}</style>
</head><body>
<h1>{{.Title}}</h1>
<div id="out">Loading...</div>
<script>
fetch('{{.SpecURL}}').then(r=>r.json()).then(spec=>{
  const out=document.getElementById('out');
  out.innerHTML='';
  for(const [path,methods] of Object.entries(spec.paths||{})){
    const h=document.createElement('h2');h.textContent=path;out.appendChild(h);
    for(const [m,op] of Object.entries(methods)){
      if(typeof op!=='object'||op===null)continue;
      const row=document.createElement('div');
      row.innerHTML='<span class="method '+m+'">'+m.toUpperCase()+'</span> '+
        '<code>'+path+'</code>'+(op.summary?' &mdash; '+op.summary:'');
      out.appendChild(row);
    }
  }
});
</script>
</body></html>`
