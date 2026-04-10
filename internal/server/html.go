package server

// serveHTML is the full interactive diagram page served by tf-lens serve.
// Unlike the export HTML, it loads graph data from /api/graph rather than
// having it baked in, enabling live refresh without server restart.
const serveHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>TF-Lens — Live</title>
<style>
*,*::before,*::after{box-sizing:border-box;margin:0;padding:0}
body{
  font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif;
  background:#F0F2F5;display:flex;flex-direction:column;height:100vh;overflow:hidden;
  color:#1A202C;
}
#bar{
  height:52px;min-height:52px;display:flex;align-items:center;gap:8px;padding:0 16px;
  background:#232F3E;border-bottom:3px solid #FF9900;flex-shrink:0;z-index:100;
}
#logo{
  display:flex;align-items:center;gap:7px;font-size:15px;font-weight:700;
  color:#F7FAFC;letter-spacing:-.3px;white-space:nowrap;margin-right:4px;
}
.live-dot{
  width:7px;height:7px;border-radius:50%;background:#38A169;
  animation:pulse 2s ease-in-out infinite;flex-shrink:0;
}
@keyframes pulse{0%,100%{opacity:1;transform:scale(1)}50%{opacity:.6;transform:scale(.85)}}
.sw{position:relative}
.sw svg{position:absolute;left:9px;top:50%;transform:translateY(-50%);pointer-events:none;color:#718096}
#q{
  width:220px;padding:6px 10px 6px 30px;border-radius:6px;
  border:1px solid #2D3748;background:#2D3748;color:#E2E8F0;
  font-size:13px;outline:none;transition:border-color .15s,background .15s;
}
#q:focus{border-color:#FF9900;background:#1A202C}
#q::placeholder{color:#4A5568}
.bg{display:flex;gap:1px}
.btn{
  height:30px;padding:0 11px;border-radius:5px;
  border:1px solid #2D3748;background:#2D3748;color:#CBD5E0;
  font-size:12px;font-weight:600;cursor:pointer;
  display:flex;align-items:center;gap:4px;white-space:nowrap;
  transition:background .12s,color .12s,border-color .12s;
}
.btn:hover{background:#4A5568;color:#F7FAFC;border-color:#4A5568}
.btn-p:hover{background:#FF9900;border-color:#FF9900;color:#1A202C}
.btn-refresh:hover{background:#38A169;border-color:#38A169;color:#FFF}
.vsp{width:1px;height:20px;background:#2D3748;margin:0 4px;flex-shrink:0}
#leg{display:flex;align-items:center;margin-left:auto}
.lg{display:flex;align-items:center;gap:10px;padding:0 10px}
.lg+.lg{border-left:1px solid #2D3748}
.li{display:flex;align-items:center;gap:5px;font-size:11px;color:#718096;white-space:nowrap}
.ld{width:10px;height:10px;border-radius:2px;flex-shrink:0}
#cy{
  flex:1;background-color:#F0F2F5;
  background-image:
    linear-gradient(rgba(160,174,192,.2) 1px,transparent 1px),
    linear-gradient(90deg,rgba(160,174,192,.2) 1px,transparent 1px);
  background-size:20px 20px;
}
.nc{
  width:80px;height:80px;background:#FFF;border-radius:10px;
  border:1.5px solid #E2E8F0;display:flex;flex-direction:column;
  align-items:center;justify-content:center;
  box-shadow:0 1px 3px rgba(0,0,0,.07),0 1px 2px rgba(0,0,0,.04);
  cursor:pointer;user-select:none;
  transition:box-shadow .15s,transform .1s;
  position:relative;overflow:hidden;
}
.nc::before{content:'';position:absolute;top:0;left:0;right:0;height:3px;border-radius:10px 10px 0 0}
.nc--networking{border-color:#BEE3F8}.nc--networking::before{background:#147EBA}
.nc--compute   {border-color:#FDDCB5}.nc--compute::before   {background:#ED7100}
.nc--storage   {border-color:#C6F6D5}.nc--storage::before   {background:#3F8624}
.nc--security  {border-color:#FED7D7}.nc--security::before  {background:#DD344C}
.nc--messaging {border-color:#E9D8FD}.nc--messaging::before {background:#8C4FFF}
.nc--unknown   {border-color:#E2E8F0}.nc--unknown::before   {background:#A0AEC0}
.nc__b{width:48px;height:48px;border-radius:9px;display:grid;place-items:center}
.nc--networking .nc__b{background:#EBF8FF}
.nc--compute    .nc__b{background:#FFFAF0}
.nc--storage    .nc__b{background:#F0FFF4}
.nc--security   .nc__b{background:#FFF5F5}
.nc--messaging  .nc__b{background:#FAF5FF}
.nc--unknown    .nc__b{background:#F7FAFC}
.nc__t{
  font-family:ui-monospace,'Menlo','Monaco','Consolas',monospace;
  font-size:13px;font-weight:800;letter-spacing:-.3px;
  line-height:1;display:block;text-align:center;position:relative;top:-1px;
}
.nc--networking .nc__t{color:#147EBA}
.nc--compute    .nc__t{color:#C05621}
.nc--storage    .nc__t{color:#276749}
.nc--security   .nc__t{color:#C53030}
.nc--messaging  .nc__t{color:#6B46C1}
.nc--unknown    .nc__t{color:#718096}
.nc:hover{box-shadow:0 4px 14px rgba(0,0,0,.11),0 2px 4px rgba(0,0,0,.07);transform:translateY(-1px)}
.nc--added  {outline:2.5px solid #38A169;outline-offset:2px}
.nc--removed{outline:2.5px dashed #E53E3E;outline-offset:2px;opacity:.6}
.nc--updated{outline:2.5px solid #D69E2E;outline-offset:2px}
.nc--sel    {outline:3px solid #0073BB;outline-offset:2px;box-shadow:0 0 0 5px rgba(0,115,187,.13)}
.cl{
  position:absolute;white-space:nowrap;background:#FFFFFF;
  border:1.5px solid currentColor;border-radius:4px;
  padding:2px 8px;font-size:10px;font-weight:700;
  letter-spacing:.6px;text-transform:uppercase;
  pointer-events:none;box-shadow:0 1px 3px rgba(0,0,0,.08);
}
.cl--networking{color:#147EBA}.cl--compute{color:#ED7100}
.cl--storage{color:#3F8624}.cl--security{color:#DD344C}
.cl--messaging{color:#8C4FFF}.cl--unknown{color:#718096}
#sb{position:absolute;bottom:14px;left:14px;display:flex;gap:8px;z-index:10;pointer-events:none}
.sp{
  background:rgba(255,255,255,.93);border:1px solid #E2E8F0;
  border-radius:20px;padding:5px 12px;font-size:11px;color:#4A5568;
  white-space:nowrap;box-shadow:0 1px 4px rgba(0,0,0,.08);
}
.sp b{color:#1A202C;font-weight:700}
#diffbar{
  position:absolute;top:60px;left:50%;transform:translateX(-50%);
  display:none;align-items:center;gap:6px;
  background:rgba(255,255,255,.95);border:1px solid #E2E8F0;
  border-radius:20px;padding:5px 14px;font-size:11px;font-weight:600;
  box-shadow:0 2px 8px rgba(0,0,0,.1);z-index:50;pointer-events:none;white-space:nowrap;
}
#diffbar.show{display:flex}
.db-dot{width:8px;height:8px;border-radius:50%;flex-shrink:0}
#loading{
  position:absolute;inset:0;background:#F0F2F5;
  display:flex;flex-direction:column;align-items:center;justify-content:center;
  gap:16px;z-index:200;transition:opacity .3s;
}
#loading.hidden{opacity:0;pointer-events:none}
.spinner{
  width:40px;height:40px;border:3px solid #E2E8F0;
  border-top-color:#FF9900;border-radius:50%;
  animation:spin .7s linear infinite;
}
@keyframes spin{to{transform:rotate(360deg)}}
#loading p{font-size:13px;color:#718096}
#panel{
  position:absolute;right:0;top:55px;bottom:0;width:280px;
  background:#FFF;border-left:1px solid #E2E8F0;
  box-shadow:-4px 0 20px rgba(0,0,0,.06);
  display:flex;flex-direction:column;
  transform:translateX(100%);
  transition:transform .22s cubic-bezier(.4,0,.2,1);z-index:30;
}
#panel.open{transform:translateX(0)}
#ph{padding:16px;background:#1A202C;color:#F7FAFC;flex-shrink:0;display:flex;gap:10px;align-items:flex-start}
#phi{width:40px;height:40px;border-radius:8px;display:grid;place-items:center;font-family:ui-monospace,'Menlo',monospace;font-size:13px;font-weight:800;color:#FFF;flex-shrink:0}
#phn{font-size:14px;font-weight:600;word-break:break-all;line-height:1.3;flex:1;min-width:0}
#pht{font-size:10px;color:#718096;margin-top:3px;font-family:ui-monospace,'Menlo',monospace}
#phx{background:none;border:none;color:#4A5568;cursor:pointer;font-size:22px;line-height:1;padding:0;flex-shrink:0;transition:color .12s}
#phx:hover{color:#F7FAFC}
#pb{flex:1;overflow-y:auto;padding:14px 16px}
.pa{margin-bottom:14px}
.pk{font-size:10px;color:#A0AEC0;text-transform:uppercase;letter-spacing:.8px;font-weight:700;margin-bottom:4px}
.pv{font-size:12px;color:#2D3748;line-height:1.5;word-break:break-all}
.pc{font-family:ui-monospace,'Menlo',monospace;font-size:11px;background:#EDF2F7;padding:3px 7px;border-radius:4px;color:#2B6CB0;display:inline-block}
.pd{height:1px;background:#EDF2F7;margin:14px 0}
.badge{display:inline-flex;align-items:center;gap:4px;padding:3px 9px;border-radius:20px;font-size:11px;font-weight:700}
.bn{background:#EBF8FF;color:#2B6CB0}.bc{background:#FFFAF0;color:#C05621}
.bs{background:#F0FFF4;color:#276749}.bse{background:#FFF5F5;color:#C53030}
.bm{background:#FAF5FF;color:#6B46C1}.bu{background:#EDF2F7;color:#4A5568}
.cb{display:inline-flex;align-items:center;gap:5px;padding:3px 9px;border-radius:4px;font-size:11px;font-weight:700}
.ca{background:#F0FFF4;color:#276749;border:1px solid #9AE6B4}
.cr{background:#FFF5F5;color:#C53030;border:1px solid #FEB2B2}
.cu{background:#FFFFF0;color:#975A16;border:1px solid #F6E05E}
/* Attributes table in panel */
.attr-table{width:100%;border-collapse:collapse;font-size:11px;margin-top:6px}
.attr-table tr{border-bottom:1px solid #EDF2F7}
.attr-table tr:last-child{border-bottom:none}
.attr-table td{padding:4px 0;vertical-align:top}
.attr-table td:first-child{color:#718096;width:45%;padding-right:8px;word-break:break-word}
.attr-table td:last-child{color:#2D3748;word-break:break-all;font-family:ui-monospace,'Menlo',monospace;font-size:10px}
</style>
</head>
<body>
<div id="bar">
  <div id="logo">
    <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#FF9900" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
    TF-Lens
    <div class="live-dot" title="Live server"></div>
  </div>
  <div class="sw">
    <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/></svg>
    <input id="q" type="text" placeholder="Search resources…" oninput="doSearch(this.value)" autocomplete="off">
  </div>
  <div class="bg">
    <button class="btn btn-p" onclick="fitG()" title="F">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M8 3H5a2 2 0 0 0-2 2v3m18 0V5a2 2 0 0 0-2-2h-3m0 18h3a2 2 0 0 0 2-2v-3M3 16v3a2 2 0 0 0 2 2h3"/></svg>
      Fit
    </button>
    <button class="btn" onclick="cy&&cy.zoom(cy.zoom()*1.3)">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="11" y1="8" x2="11" y2="14"/><line x1="8" y1="11" x2="14" y2="11"/></svg>
    </button>
    <button class="btn" onclick="cy&&cy.zoom(cy.zoom()*.77)">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><circle cx="11" cy="11" r="8"/><line x1="21" y1="21" x2="16.65" y2="16.65"/><line x1="8" y1="11" x2="14" y2="11"/></svg>
    </button>
    <button class="btn btn-refresh" onclick="refreshGraph()" title="Reload graph from server">
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M3 12a9 9 0 0 1 9-9 9.75 9.75 0 0 1 6.74 2.74L21 8"/><path d="M21 3v5h-5"/><path d="M21 12a9 9 0 0 1-9 9 9.75 9.75 0 0 1-6.74-2.74L3 16"/><path d="M8 16H3v5"/></svg>
      Refresh
    </button>
  </div>
  <div class="vsp"></div>
  <div id="leg">
    <div class="lg">
      <div class="li"><div class="ld" style="background:#147EBA"></div>Network</div>
      <div class="li"><div class="ld" style="background:#ED7100"></div>Compute</div>
      <div class="li"><div class="ld" style="background:#3F8624"></div>Storage</div>
      <div class="li"><div class="ld" style="background:#DD344C"></div>Security</div>
      <div class="li"><div class="ld" style="background:#8C4FFF"></div>Messaging</div>
    </div>
    <div class="lg">
      <div class="li"><div class="ld" style="background:#38A169"></div>Added</div>
      <div class="li"><div class="ld" style="background:#E53E3E;opacity:.7"></div>Removed</div>
      <div class="li"><div class="ld" style="background:#D69E2E"></div>Changed</div>
    </div>
  </div>
</div>

<div id="cy"></div>

<div id="loading">
  <div class="spinner"></div>
  <p>Loading infrastructure diagram…</p>
</div>

<div id="sb">
  <div class="sp" id="sc">Loading…</div>
  <div class="sp" id="sh" style="display:none">Press <b>Esc</b> to clear &nbsp;·&nbsp; <b>F</b> to fit</div>
</div>

<div id="diffbar">
  <span style="color:#718096;font-size:10px;margin-right:4px">DIFF</span>
  <span style="display:flex;align-items:center;gap:4px;color:#276749;font-size:11px">
    <span class="db-dot" style="background:#38A169"></span><span id="db-ac">0</span> added
  </span>
  <span style="color:#CBD5E0">·</span>
  <span style="display:flex;align-items:center;gap:4px;color:#C53030;font-size:11px">
    <span class="db-dot" style="background:#E53E3E"></span><span id="db-rc">0</span> removed
  </span>
  <span style="color:#CBD5E0">·</span>
  <span style="display:flex;align-items:center;gap:4px;color:#975A16;font-size:11px">
    <span class="db-dot" style="background:#D69E2E"></span><span id="db-uc">0</span> changed
  </span>
</div>

<div id="panel">
  <div id="ph">
    <div id="phi">?</div>
    <div style="flex:1;min-width:0"><div id="phn">Resource</div><div id="pht"></div></div>
    <button id="phx" onclick="closePanel()" title="Esc">×</button>
  </div>
  <div id="pb"></div>
</div>

<script src="https://cdnjs.cloudflare.com/ajax/libs/cytoscape/3.28.1/cytoscape.min.js"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/dagre/0.8.5/dagre.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/cytoscape-dagre@2.5.0/cytoscape-dagre.min.js"></script>
<script src="https://cdn.jsdelivr.net/npm/cytoscape-node-html-label@1.2.1/dist/cytoscape-node-html-label.min.js"></script>

<script>
(function(){
var cy = null;
var CAT={
  networking:{label:'Networking',color:'#147EBA',badge:'bn'},
  compute:   {label:'Compute',   color:'#ED7100',badge:'bc'},
  storage:   {label:'Storage',   color:'#3F8624',badge:'bs'},
  security:  {label:'Security',  color:'#DD344C',badge:'bse'},
  messaging: {label:'Messaging', color:'#8C4FFF',badge:'bm'},
  unknown:   {label:'Unknown',   color:'#A0AEC0',badge:'bu'},
};

// ── Build / rebuild diagram from element data ──────────────────────────────
function buildDiagram(data){
  var elements = data.elements;

  // Destroy previous instance
  if(cy){ cy.destroy(); cy=null; }
  document.querySelectorAll('.cl').forEach(function(el){ el.remove(); });

  cy = cytoscape({
    container: document.getElementById('cy'),
    elements: elements,
    style:[
      {selector:'node',style:{
        shape:'roundrectangle',width:'80px',height:'80px',
        'background-opacity':0,'border-width':0,
        label:'data(label)','text-valign':'bottom','text-halign':'center',
        'text-margin-y':'10px','font-size':'11.5px',
        'font-family':"-apple-system,BlinkMacSystemFont,'Segoe UI',Arial,sans-serif",
        'font-weight':'600',color:'#2D3748','text-wrap':'ellipsis','text-max-width':'100px',
      }},
      {selector:'$node > node',style:{
        shape:'roundrectangle',padding:'48px','background-image':'none',
        'background-color':'#FAFCFF','background-opacity':'0.6',
        'border-width':'1.5px','border-style':'dashed','border-color':'#147EBA',label:'',
      }},
      {selector:'edge',style:{
        'curve-style':'taxi','taxi-direction':'auto','taxi-turn':'50px',
        'taxi-turn-min-distance':'10px',width:'1.2px',
        'line-color':'#A0AEC0','target-arrow-color':'#718096',
        'target-arrow-shape':'triangle','arrow-scale':0.6,
        'source-distance-from-node':'8px','target-distance-from-node':'8px',opacity:0.85,
      }},
      {selector:'edge:selected',style:{'line-color':'#0073BB','target-arrow-color':'#0073BB',width:'2px',opacity:'1'}},
      {selector:'.faded',style:{opacity:0.1}},
      {selector:'.edge-hl',style:{'line-color':'#0073BB','target-arrow-color':'#0073BB',width:'2px',opacity:1}},
    ],
    layout:{
      name:'dagre',rankDir:'TB',
      nodeSep:60,rankSep:80,edgeSep:20,
      padding:60,spacingFactor:1.1,animate:false,
    },
    wheelSensitivity:0.2,minZoom:0.1,maxZoom:4,
    boxSelectionEnabled:false,selectionType:'single',
  });

  window.cy = cy;

  // HTML node labels
  if(typeof cy.nodeHtmlLabel==='function'){
    cy.nodeHtmlLabel([{
      query:'node',halign:'center',valign:'center',halignBox:'center',valignBox:'center',
      tpl:function(d){
        if(d.isParent) return '<div style="display:none"></div>';
        var cat=d.category||'unknown';
        var chg=d.changeType?' nc--'+d.changeType:'';
        return '<div class="nc nc--'+cat+chg+'" data-id="'+d.id+'">'
             + '<div class="nc__b"><span class="nc__t">'+(d.abbrev||'?')+'</span></div>'
             + '</div>';
      }
    }]);
  }

  // Container labels
  function renderContainerLabels(){
    var container=document.getElementById('cy');
    container.querySelectorAll('.cl').forEach(function(el){ el.remove(); });
    var pan=cy.pan(), zoom=cy.zoom();
    cy.nodes().filter(function(n){ return n.isParent(); }).forEach(function(n){
      var bb=n.boundingBox();
      var cat=n.data('category')||'networking';
      var screenX=(bb.x1+(bb.x2-bb.x1)/2)*zoom+pan.x;
      var screenY=bb.y1*zoom+pan.y;
      var el=document.createElement('div');
      el.className='cl cl--'+cat;
      el.textContent=n.data('label');
      el.style.cssText='position:absolute;left:'+screenX+'px;top:'+screenY+'px;transform:translate(-50%,-50%);z-index:20;font-size:'+Math.max(8,Math.min(13,10*zoom))+'px;padding:'+Math.max(1,2*zoom)+'px '+Math.max(4,8*zoom)+'px';
      container.appendChild(el);
    });
  }
  cy.on('render pan zoom',renderContainerLabels);
  renderContainerLabels();

  // Statusbar
  var lc=cy.nodes().filter(function(n){ return !n.isParent(); }).length;
  document.getElementById('sc').innerHTML='<b>'+lc+'</b> resources &nbsp;·&nbsp; <b>'+data.edgeCount+'</b> connections';

  // Diff banner
  var dc={added:0,removed:0,updated:0};
  elements.forEach(function(el){
    if(el.group==='nodes'&&el.data.changeType&&el.data.changeType!==''){
      if(dc[el.data.changeType]!==undefined) dc[el.data.changeType]++;
    }
  });
  if(dc.added+dc.removed+dc.updated>0){
    document.getElementById('db-ac').textContent=dc.added;
    document.getElementById('db-rc').textContent=dc.removed;
    document.getElementById('db-uc').textContent=dc.updated;
    document.getElementById('diffbar').classList.add('show');
  }

  // Events
  cy.on('tap','node',function(evt){
    var n=evt.target;
    if(n.isParent()) return;
    highlightNode(n);
    openPanel(n.data());
    document.getElementById('sh').style.display='';
  });
  cy.on('tap',function(evt){
    if(evt.target===cy){ clearSel(); closePanel(); document.getElementById('sh').style.display='none'; }
  });

  fitG();
}

// ── Fetch graph from server and build/rebuild diagram ──────────────────────
function refreshGraph(){
  var refreshBtn = document.querySelector('.btn-refresh');
  if(refreshBtn){ refreshBtn.disabled=true; refreshBtn.style.opacity='0.5'; }

  fetch('/api/graph')
    .then(function(r){
      if(!r.ok) throw new Error('HTTP '+r.status);
      return r.json();
    })
    .then(function(data){
      document.getElementById('loading').classList.add('hidden');
      buildDiagram(data);
      if(refreshBtn){ refreshBtn.disabled=false; refreshBtn.style.opacity='1'; }
    })
    .catch(function(err){
      console.error('Failed to load graph:', err);
      document.getElementById('sc').innerHTML = '⚠️ Failed to load graph — check server';
      if(refreshBtn){ refreshBtn.disabled=false; refreshBtn.style.opacity='1'; }
    });
}
window.refreshGraph = refreshGraph;

// ── Selection ─────────────────────────────────────────────────────────────
var selId=null;
function highlightNode(node){
  clearSel();
  selId=node.id();
  var el=document.querySelector('.nc[data-id="'+selId+'"]');
  if(el) el.classList.add('nc--sel');
  cy.elements().addClass('faded');
  node.closedNeighborhood().removeClass('faded');
  node.connectedEdges().addClass('edge-hl').removeClass('faded');
}
function clearSel(){
  if(cy) cy.elements().removeClass('faded edge-hl');
  document.querySelectorAll('.nc--sel').forEach(function(el){ el.classList.remove('nc--sel'); });
  selId=null;
}

// ── Panel ────────────────────────────────────────────────────────────────
window.openPanel=function(d){
  var cat=d.category||'unknown';
  var c=CAT[cat]||CAT.unknown;
  var phi=document.getElementById('phi');
  phi.textContent=d.abbrev||'?';
  phi.style.background=c.color;
  document.getElementById('phn').textContent=d.label;
  document.getElementById('pht').textContent=d.type;

  var h='';
  h+='<div class="pa"><div class="pk">Category</div><div class="pv"><span class="badge '+c.badge+'">'+c.label+'</span></div></div>';
  if(d.changeType&&d.changeType!==''){
    var ci={added:'✚',removed:'✕',updated:'↻'};
    var cc={added:'ca',removed:'cr',updated:'cu'};
    h+='<div class="pa"><div class="pk">Change</div><div class="pv"><span class="cb '+(cc[d.changeType]||'')+'">'+( ci[d.changeType]||'')+' '+d.changeType.toUpperCase()+'</span></div></div>';
  }
  h+='<div class="pd"></div>';
  h+='<div class="pa"><div class="pk">Address</div><div class="pv"><span class="pc">'+d.id+'</span></div></div>';
  h+='<div class="pa"><div class="pk">Type</div><div class="pv"><span class="pc">'+d.type+'</span></div></div>';
  document.getElementById('pb').innerHTML=h;
  document.getElementById('panel').classList.add('open');
};
window.closePanel=function(){
  document.getElementById('panel').classList.remove('open');
};

// ── Search ────────────────────────────────────────────────────────────────
window.doSearch=function(q){
  if(!cy) return;
  clearSel();
  var t=q.toLowerCase().trim();
  if(!t){ cy.elements().removeClass('faded'); return; }
  cy.nodes().forEach(function(n){
    var m=(n.data('label')||'').toLowerCase().includes(t)
       ||(n.data('type')||'').toLowerCase().includes(t)
       ||(n.data('abbrev')||'').toLowerCase().includes(t)
       ||n.id().toLowerCase().includes(t);
    if(m) n.removeClass('faded'); else n.addClass('faded');
  });
  cy.edges().forEach(function(e){
    var ok=!e.source().hasClass('faded')&&!e.target().hasClass('faded');
    if(ok) e.removeClass('faded'); else e.addClass('faded');
  });
};

// ── Keyboard ──────────────────────────────────────────────────────────────
document.addEventListener('keydown',function(e){
  if(e.target.matches('input')) return;
  if(e.key==='Escape'||e.key==='Esc'){
    clearSel(); closePanel();
    document.getElementById('q').value='';
    if(cy) cy.elements().removeClass('faded');
    document.getElementById('sh').style.display='none';
  }
  if(e.key==='f'||e.key==='F') fitG();
  if(e.key==='r'||e.key==='R') refreshGraph();
  if(e.key==='+'||e.key==='=') cy&&cy.zoom(cy.zoom()*1.3);
  if(e.key==='-') cy&&cy.zoom(cy.zoom()*.77);
});

window.fitG=function(){ cy&&cy.fit(undefined,60); };

// ── Initial load ──────────────────────────────────────────────────────────
refreshGraph();

})();
</script>
</body>
</html>
`