'use strict';
const $ = id => document.getElementById(id);
const initialFEN='rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1';
const state={fen:initialFEN,legal:[],selected:null,history:[],color:'white',flipped:false,tab:'setup',tree:null,nodeID:0,zoom:1,width:850,height:450,coords:{},lastPhase:'idle',lastToast:0,depthLimit:12,collapseDepth:8,expanded:new Set(),collapsed:new Set(),nodeBudget:250,progressStamp:0};
function esc(t){return String(t??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]))}
async function api(path,opts={}){
 const res=await fetch(path,{...opts,headers:{'Content-Type':'application/json',...(opts.headers||{})}});
 const isJSON=(res.headers.get('content-type')||'').includes('json');const data=isJSON?await res.json():await res.text();
 if(!res.ok)throw Error(data?.error||`HTTP ${res.status}`);return data;
}
function toast(message,error=false){let el=$('toast');el.textContent=message;el.classList.toggle('error',error);el.classList.add('visible');const t=++state.lastToast;setTimeout(()=>{if(t===state.lastToast)el.classList.remove('visible')},3500)}
function tab(name){state.tab=name;$('page-setup').classList.toggle('active',name==='setup');$('page-results').classList.toggle('active',name==='results');for(const key of ['setup','results']){$('tab-'+key).classList.toggle('active',key===name);$('tab-'+key).setAttribute('aria-selected',String(key===name))}if(name==='results'&&state.tree){setTimeout(()=>{graphRender();if(!state.didFit){fitGraph();state.didFit=true}},50)}}
function parseBoard(fen){const ranks=fen.split(' ')[0].split('/'),board={};for(let ri=0;ri<8;ri++){let file=0;for(const c of ranks[ri]){if(/[1-8]/.test(c)){file+=Number(c)}else{board['abcdefgh'[file]+(8-ri)]=c;file++}}}return board}
// The board is a single fixed-coordinate SVG: pieces cannot change the size of
// its 64 squares. Legal move validation remains on the Go backend.
function boardMarkup(fen,flipped=false,interactive=false){
 const board=parseBoard(fen), selected=interactive?state.selected:null;
 let squares='',pieces='',coordinates='',hitAreas='';
 for(let i=0;i<64;i++){
  const row=Math.floor(i/8),col=i%8,file=flipped?7-col:col,rank=flipped?row+1:8-row;
  const s='abcdefgh'[file]+rank,p=board[s],x=col*100,y=row*100;
  const dark=(file+rank)%2===1; // a1 dark, h1 light
  squares+=`<rect x="${x}" y="${y}" width="100" height="100" fill="${dark?'#91a47c':'#f0e8ce'}"/>`;
  if(selected===s)squares+=`<rect x="${x+3}" y="${y+3}" width="94" height="94" fill="none" stroke="#e7c664" stroke-width="6"/>`;
  const legal=selected&&state.legal.some(m=>m.from===selected&&m.to===s);
  if(legal)squares+=p?`<circle cx="${x+50}" cy="${y+50}" r="44" fill="none" stroke="#267c44" stroke-width="7" opacity=".85"/>`:`<circle cx="${x+50}" cy="${y+50}" r="15" fill="#267c44" opacity=".55"/>`;
  if(p){const side=p===p.toUpperCase()?'w':'b';pieces+=`<image x="${x+6}" y="${y+6}" width="88" height="88" href="/assets/pieces/${side+p.toUpperCase()}.svg" preserveAspectRatio="xMidYMid meet" pointer-events="none"/>`;}
  if(col===0)coordinates+=`<text x="${x+6}" y="${y+15}" fill="${dark?'#223622':'#657859'}" font-size="16" font-weight="700" pointer-events="none">${rank}</text>`;
  if(row===7)coordinates+=`<text x="${x+87}" y="${y+94}" text-anchor="end" fill="${dark?'#223622':'#657859'}" font-size="16" font-weight="700" pointer-events="none">${s[0]}</text>`;
  if(interactive)hitAreas+=`<rect data-square="${s}" x="${x}" y="${y}" width="100" height="100" fill="transparent" role="button" tabindex="0" aria-label="${s}${p?' '+(p===p.toUpperCase()?'white ':'black ')+p.toUpperCase():''}"/>`;
 }
 return `<svg class="chessboard-svg" viewBox="0 0 800 800" preserveAspectRatio="xMidYMid meet" xmlns="http://www.w3.org/2000/svg" role="${interactive?'group':'img'}" aria-label="Chess position, ${flipped?'Black':'White'} at the bottom">${squares}${pieces}${coordinates}${hitAreas}</svg>`;
}
async function syncBoard(){
 try{const data=await api('/api/board?fen='+encodeURIComponent(state.fen));state.fen=data.fen;state.legal=data.legal;state.selected=null;$('fen-input').value=state.fen;$('side-to-move').textContent=data.turn==='white'?'White to move':'Black to move';drawBoard()}
 catch(e){toast(e.message,true)}
}
function drawBoard(){const area=$('chessboard');area.innerHTML=boardMarkup(state.fen,state.flipped,true);area.querySelectorAll('[data-square]').forEach(b=>{
  b.addEventListener('click',()=>boardClick(b.dataset.square));
  b.addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();boardClick(b.dataset.square)}});
 });
 const moves=state.history.map((x,i)=>`<b>${esc(x.san)}</b>`).join(' · ');$('move-history').innerHTML=moves||'Starting position ready. Click a piece to play a legal move.';
 $('undo-move').disabled=state.history.length===0;
}
async function boardClick(s){
 const legalFrom=state.legal.filter(x=>x.from===s);
 if(legalFrom.length){state.selected=state.selected===s?null:s;drawBoard();return}
 if(!state.selected){return}
 const moves=state.legal.filter(x=>x.from===state.selected&&x.to===s);
 if(moves.length===0){state.selected=null;drawBoard();return}
 let move=moves[0];if(moves.length>1){const p=prompt('Promote to: Q, R, B or N','Q');if(!p)return;move=moves.find(x=>x.uci.endsWith(p.trim().toLowerCase()))||moves[0]}
 try {const previous=state.fen;const reply=await api('/api/apply',{method:'POST',body:JSON.stringify({fen:previous,uci:move.uci})});state.fen=reply.fen;state.history.push({fen:previous,san:reply.san});await syncBoard()}
 catch(e){toast(e.message,true)}
}
function selectedValues(id,asNumber=false){return [...$(id).querySelectorAll('input:checked')].map(x=>asNumber?Number(x.value):x.value)}
function num(id){return Number($(id).value)}
function settings(){return {
 start_fen:state.fen,color:state.color,speeds:selectedValues('speed-options'),ratings:selectedValues('rating-options',true),
 max_depth:num('max-depth'),max_nodes:num('max-nodes'),coverage:num('coverage'),min_games:num('min-games'),delay_ms:num('delay-ms'),
 resume:$('resume-generation').checked,engine_mode:$('engine-mode').value,engine_lines:num('engine-lines'),engine_threshold:num('engine-threshold'),
 candidates:num('candidates'),stockfish_depth:num('stockfish-depth'),stockfish_path:$('stockfish-path').value.trim(),event:$('event-name').value.trim()
}}
const stockfishState={available:false,platform:'linux',path:''};
function engineVisibility(){
 const mode=$('engine-mode').value;
 const needsLocal=(mode==='local'||mode==='hybrid');
 $('stockfish-depth').disabled=!needsLocal;
 $('local-engine-panel').classList.toggle('engine-selected',needsLocal);
}
async function checkEngine(){
 const path=$('stockfish-path').value.trim();
 const d=await api('/api/engine'+(path?'?path='+encodeURIComponent(path):''));
 stockfishState.available=d.available;stockfishState.path=d.path;
 const hints={darwin:'macOS: brew install stockfish',linux:'Ubuntu: sudo apt update && sudo apt install stockfish',windows:'Windows: install Stockfish from stockfishchess.org and provide the path to stockfish.exe'};
 $('stockfish-hint').textContent=d.available?'✓ Stockfish available: '+d.path:'Stockfish not detected. '+(hints[stockfishState.platform]||hints.linux);
 for(const mode of ['local','hybrid'])$('engine-mode').querySelector(`option[value="${mode}"]`).disabled=!d.available;
 if(!d.available&&['hybrid','local'].includes($('engine-mode').value)){
   $('engine-mode').value='cloud';toast('Stockfish is missing: switched to Lichess Cloud.',true);
 }
 engineVisibility();return d;
}
async function loadMeta(){try{const d=await api('/api/meta');$('app-version').textContent='v'+d.version;stockfishState.platform=d.platform;
 if(d.stockfish&&!$('stockfish-path').value){$('stockfish-path').placeholder=d.stockfish_path; if($('engine-mode').value==='cloud')$('engine-mode').value='hybrid';}
 await checkEngine();
 }catch(e){toast(e.message,true)}}
async function tokenStatus(){
 const d=await api('/api/credentials');$('token-state').textContent=d.saved?(d.source==='environment'?'Provided by environment':'Saved · '+d.source):'Not saved';
 $('forget-token').disabled=!d.saved||d.environment_override;
 $('save-token').disabled=!!d.environment_override;
 if(d.environment_override)$('lichess-token').placeholder='Environment variable LICHESS_TOKEN takes precedence';
}
async function saveToken(){try{const input=$('lichess-token');const token=input.value.trim();if(!token)throw Error('Paste a token before saving.');
 await api('/api/credentials',{method:'POST',body:JSON.stringify({token})});input.value='';await tokenStatus();toast('Token saved for this user.');
 }catch(e){toast(e.message,true)}}
async function forgetToken(){try{await api('/api/credentials',{method:'DELETE'});$('lichess-token').value='';await tokenStatus();toast('Saved token removed.');}catch(e){toast(e.message,true)}}
async function start(){try{
 const cfg=settings();if(!cfg.speeds.length||!cfg.ratings.length)throw Error('Choose at least one speed and rating.');
 await api('/api/start',{method:'POST',body:JSON.stringify(cfg)});
 state.tree=null;state.didFit=false;state.nodeBudget=cfg.max_nodes;
 $('analysis-heading').textContent='Growing your repertoire';$('analysis-message').textContent='Retrieving positions and evaluating candidate moves…';
 $('generate').disabled=true;$('cancel-analysis').hidden=false;$('progress-overlay').hidden=false;
 await updateProgress();toast('Generation started. Watch the tree grow.');
 }catch(e){toast(e.message,true)}
}
// Positive repertoire POV -> green; negative -> red; missing evaluation -> neutral.
// 200 cp gives a strong but not saturated color. Mate scores saturate.
function liveNodeColor(n,playerColor){
 const mate=n.eval_mate;
 const cp=mate!=null?(mate>0?100000:-100000):n.eval_cp;
 if(cp===null||cp===undefined)return '#81918e';
 const v=Math.tanh(((playerColor==='black'?-1:1)*cp)/220);
 const neutral=[111,135,128], positive=[77,199,108], negative=[230,94,105];
 const target=v>=0?positive:negative,t=Math.abs(v);
 return '#'+neutral.map((c,i)=>Math.round(c*(1-t)+target[i]*t).toString(16).padStart(2,'0')).join('');
}
function liveTreeRender(d){
 const tree=d.tree||[],svg=$('live-tree-svg');if(!tree.length){svg.innerHTML='';return;}
 const byId=new Map(tree.map(n=>[n.id,n]));
 const kids=new Map(tree.map(n=>[n.id,[]]));
 for(const n of tree)if(kids.has(n.parent))kids.get(n.parent).push(n.id);
 let leaf=0,maxDepth=Math.max(1,...tree.map(n=>n.depth));const Y={},X={};
 // Fixed SVG coordinates, with a scrollable viewport rather than shrinking labels.
 function visit(id){const n=byId.get(id);if(!n)return;
  X[id]=70+n.depth*122;const ch=kids.get(id)||[];
  if(!ch.length)Y[id]=++leaf;
  else{for(const cid of ch)visit(cid);Y[id]=(Y[ch[0]]+Y[ch[ch.length-1]])/2;}
 }
 visit(tree[0].id);
 const height=Math.max(290,leaf*45+65),width=Math.max(900,(maxDepth+1)*122+95);
 const py=id=>26+(Y[id]-.5)*45;
 let paths='',nodes='';
 const cfgColor=state.color;
 for(const n of tree){if(!Number.isFinite(X[n.id])||!Number.isFinite(Y[n.id]))continue;
  const x=X[n.id],y=py(n.id),parentX=X[n.parent],parentY=py(n.parent);
  if(n.parent>=0&&Number.isFinite(parentX))paths+=`<path class="live-tree-edge" d="M${parentX},${parentY} C${(parentX+x)/2},${parentY} ${(parentX+x)/2},${y} ${x},${y}"/>`;
  const label=n.id===0?'Start':(n.ply?Math.ceil(n.ply/2)+(n.ply%2?'.':'...'):'')+n.san;
  const color=liveNodeColor(n,cfgColor);
  const evalStr=n.eval_mate!=null?'#'+n.eval_mate:n.eval_cp!=null?(n.eval_cp/100).toFixed(2):'not available';
  const tooltip=`${label} • Eval (white POV): ${evalStr}${n.transposition_to!=null?' • Transposition to #'+n.transposition_to:''}`;
  nodes+=`<g class="live-point"><title>${esc(tooltip)}</title><text class="live-tree-move" x="${x}" y="${y-12}" text-anchor="middle">${esc(label)}</text><circle class="live-tree-node${n.id===d.current?' just-added':''}" cx="${x}" cy="${y}" r="${n.id===d.current?7:5.5}" fill="${color}"/>${n.transposition_to!=null?`<text x="${x+11}" y="${y+4}" fill="#d3c2ff" font-size="13">↪</text>`:''}</g>`;
 }
 svg.setAttribute('viewBox',`0 0 ${width} ${height}`);
 svg.style.width=width+'px';svg.style.height=height+'px';
 svg.innerHTML=paths+nodes;
 // Follow the currently explored region without forcing full-page scrolling.
 const cur=byId.get(d.current),container=$('live-tree-viewport');
 if(cur&&Number.isFinite(X[cur.id])){
  const wantX=Math.max(0,X[cur.id]-container.clientWidth*.60),wantY=Math.max(0,py(cur.id)-container.clientHeight*.50);
  if(Math.abs(container.scrollLeft-wantX)>120||Math.abs(container.scrollTop-wantY)>120){
   container.scrollLeft=wantX;container.scrollTop=wantY;
  }
 }
}
async function updateProgress(){
 const d=await api('/api/progress');if(d.phase!=='running')return;
 const parsed=d.tree||[],depth=parsed.reduce((m,n)=>Math.max(m,n.depth),0);
 $('progress-nodes').textContent=d.nodes;
 $('progress-processed').textContent=d.processed;
 $('progress-pending').textContent=d.pending;
 $('progress-depth').textContent=depth;
 $('progress-requests').textContent=d.requests;
 $('progress-local').textContent=d.local_evaluations;
 $('progress-transpositions').textContent=d.transpositions||0;
 $('progress-phase').textContent=$('analysis-message').textContent;
 const pct=(d.processed+d.pending)>0?Math.round(100*d.processed/(d.processed+d.pending)):0;
 $('frontier-percent').textContent=pct+'% · live estimate';$('progress-fill').style.width=pct+'%';
 const used=Math.min(100,Math.round(100*d.nodes/Math.max(1,state.nodeBudget)));
 $('budget-percent').textContent=used+'%';$('budget-fill').style.width=used+'%';
 liveTreeRender(d);
}
async function poll(){try{
 const d=await api('/api/status');state.lastPhase=d.phase;
 $('system-status').textContent=d.phase==='running'?`Analyzing · ${d.nodes} nodes`:d.phase==='done'?'Repertoire ready':d.phase==='error'?'Analysis issue':d.phase==='cancelled'?'Analysis stopped':'Ready to configure';
 $('analysis-message').textContent=d.message+(d.phase==='running'?` · ${d.requests} API requests`:'');
 $('generate').disabled=d.phase==='running';$('cancel-analysis').hidden=d.phase!=='running';
 $('progress-overlay').hidden=d.phase!=='running';
 if(d.phase==='running'){$('analysis-heading').textContent=`Exploring ${d.nodes} nodes`;await updateProgress();}
 else if(d.phase==='done'){$('analysis-heading').textContent='Ready to review';}
 else if(d.phase==='error'){$('analysis-heading').textContent='Generation stopped';if(d.error&&d.error!==state.lastError){toast(d.error,true);state.lastError=d.error}}
 if((d.phase==='done'||d.phase==='cancelled'||d.phase==='error')&&d.ready&&!state.tree){await loadTree();tab('results');}
 }catch(e){/* Temporary HTTP failures do not disrupt the interface. */}}

async function loadTree(){const data=await api('/api/tree');state.tree=data;state.nodeID=0;state.didFit=false;
 let active=data.nodes.filter(n=>!n.deleted);$('stat-nodes').textContent=active.length;
 $('stat-leaves').textContent=active.filter(n=>n.id!==0&&!n.children.some(id=>!data.nodes[id].deleted)).length;
 const maxD=Math.max(1,...data.nodes.map(n=>n.depth));
 $('view-depth').max=maxD;$('collapse-depth').max=maxD+1;
 if(!state.treeConfigured){state.depthLimit=Math.min(12,maxD);state.collapseDepth=Math.min(8,maxD+1);state.treeConfigured=true}
 $('view-depth').value=Math.min(state.depthLimit,maxD);$('collapse-depth').value=Math.min(state.collapseDepth,maxD+1);
 state.depthLimit=Number($('view-depth').value);state.collapseDepth=Number($('collapse-depth').value);
 $('view-depth-label').textContent=state.depthLimit;$('collapse-depth-label').textContent=state.collapseDepth;
 const warnings=(data.warnings||[]).join(' · ');$('results-warning').textContent=warnings||'Engine evaluations use white’s perspective. Generated lines should be reviewed before study.';
 graphRender();selectNode(0);
}
function isPlayerMove(n){if(n.id===0||!state.tree)return false;let parent=state.tree.nodes[n.parent];return (parent.fen.split(' ')[1]==='w')===(state.tree.config.color==='white')}
function graphRender(){
 const r=state.tree,root=$('tree-svg');if(!r){$('graph-empty').hidden=false;root.innerHTML='';return;}
 $('graph-empty').hidden=true;
 const xs={},ys={},visible=new Set(),expanded=state.expanded,collapsed=state.collapsed;
 let leaf=0,maxDepth=0;const gapY=101,gapX=210,startY=62,startX=37,nodeW=169,nodeH=55;
 const alive=id=>r.nodes[id].children.filter(cid=>!r.nodes[cid].deleted);
 function kids(id,depth){if(depth>=state.depthLimit)return [];
  if(collapsed.has(id))return [];
  if(depth>=state.collapseDepth&&!expanded.has(id))return [];
  return alive(id);
 }
 function layout(id,depth){if(!r.nodes[id]||r.nodes[id].deleted)return;
  maxDepth=Math.max(maxDepth,depth);visible.add(id);xs[id]=startX+depth*gapX;
  const children=kids(id,depth);
  if(!children.length){ys[id]=startY+leaf*gapY;leaf++}
  else {for(const cid of children)layout(cid,depth+1);ys[id]=(ys[children[0]]+ys[children[children.length-1]])/2;}
 }
 layout(0,0);
 const naturalW=Math.max(850,(maxDepth+1)*gapX+95),naturalH=Math.max(430,leaf*gapY+90);
 state.width=naturalW;state.height=naturalH;state.coords={x:xs,y:ys};state.visible=visible;
 let edges='',nodes='';
 for(const id of visible){if(id===0)continue;const n=r.nodes[id],px=xs[n.parent]+nodeW,py=ys[n.parent]+nodeH/2,cx=xs[id],cy=ys[id]+nodeH/2;
  edges+=`<path class="tree-connector" d="M${px},${py} C${px+23},${py} ${cx-23},${cy} ${cx},${cy}"/>`;
 }
 for(const id of visible){const n=r.nodes[id],player=isPlayerMove(n),rootNode=n.id===0,selected=n.id===state.nodeID;
  const bg=rootNode?'#314b41':player?'#344f38':'#3f3d35',stroke=rootNode?'#8cb06c':player?'#719867':'#887e60';
  const move=rootNode?'START':n.san,fullOpening=n.opening_name?`${n.opening_eco||''} ${n.opening_name}`.trim():'Opening not classified';
  const opening=fullOpening.length>29?fullOpening.slice(0,28)+'…':fullOpening;
  const score=n.eval_mate!=null?'#'+n.eval_mate:n.eval_cp!=null?(n.eval_cp/100).toFixed(2):'—';
  const subtitle=rootNode?'Starting FEN':`${n.frequency?`${n.frequency.toFixed(1)}% · `:''}${score} eval`;
  const hidden=alive(id).length-kids(id,n.depth).length;
  nodes+=`<g class="node-card${selected?' selected':''}${rootNode?' root-node':''}" data-node="${id}" transform="translate(${xs[id]},${ys[id]})" tabindex="0" role="button" aria-label="${esc(move)}"><title>${esc(fullOpening)} — ${esc(move)}</title>`+
   `<text class="node-opening" x="3" y="-8">${esc(opening)}</text>`+
   `<rect width="${nodeW}" height="${nodeH}" rx="9" fill="${bg}" stroke="${stroke}" stroke-width="1.5"/>`+
   `<circle cx="11" cy="16" r="3" fill="${player?'#b6dc79':'#dec38f'}"/><text class="node-txt" x="22" y="20">${esc(move)}</text>`+
   `<text class="node-small" x="13" y="43">${esc(subtitle)}</text>`+
   `${n.trap_status==='yes'?'<text x="145" y="19" font-size="12" fill="#f7ca77">⚠</text>':''}`+
   `${hidden?`<g class="node-collapse" data-toggle="${id}" role="button"><circle cx="157" cy="39" r="11" fill="#25372c" stroke="#9eb58a"/><text x="157" y="43" text-anchor="middle" font-size="13" fill="#e7f3d4">${state.collapsed.has(id)?'+':'+'}</text></g><text class="node-collapsed-note" x="${nodeW-2}" y="${nodeH+12}" text-anchor="end">+${hidden} hidden</text>`:alive(id).length?`<g class="node-collapse" data-toggle="${id}" role="button"><circle cx="157" cy="39" r="9" fill="#25372c" stroke="#9eb58a"/><text x="157" y="43" text-anchor="middle" font-size="11" fill="#e7f3d4">−</text></g>`:''}</g>`;
 }
 root.setAttribute('viewBox',`0 0 ${naturalW} ${naturalH}`);root.innerHTML=edges+nodes;
 root.querySelectorAll('[data-node]').forEach(el=>{
  const id=Number(el.dataset.node);
  el.addEventListener('click',e=>{if(e.target.closest('[data-toggle]'))return;selectNode(id)});
  el.addEventListener('keydown',e=>{if((e.key==='Enter'||e.key===' ')&&!e.target.closest('[data-toggle]')){e.preventDefault();selectNode(id)}});
 });
 root.querySelectorAll('[data-toggle]').forEach(el=>el.addEventListener('click',e=>{
  e.stopPropagation();const id=Number(el.dataset.toggle);
  if(state.collapsed.has(id)){state.collapsed.delete(id);state.expanded.add(id)}
  else if(alive(id).length&&state.visible.has(alive(id)[0])){state.collapsed.add(id);state.expanded.delete(id)}
  else {state.collapsed.delete(id);state.expanded.add(id)}
  graphRender();
 }));
 setZoom(state.zoom);
 $('graph-subtitle').textContent=`${visible.size} shown of ${r.nodes.filter(n=>!n.deleted).length} active · Tap + to expand branches`;
}
function renderMoveRuler(){
 const r=state.tree;if(!r)return;
 const maxDepth=Math.max(0,...[...state.visible||[]].map(id=>r.nodes[id].depth));
 const rootPly=r.nodes[0].ply,nums=new Map();
 for(let d=1;d<=maxDepth;d++){
  const moveNo=Math.ceil((rootPly+d)/2);const x=37+d*210+84;
  if(!nums.has(moveNo))nums.set(moveNo,[]);nums.get(moveNo).push(x);
 }
 const inner=$('move-ruler-inner');inner.style.width=Math.round(state.width*state.zoom)+'px';
 inner.innerHTML=[...nums].map(([no,arr])=>`<span class="move-number" style="left:${(arr.reduce((a,b)=>a+b,0)/arr.length)*state.zoom}px">Move ${no}</span>`).join('');
 syncRuler();
}
function syncRuler(){$('move-ruler-inner').style.transform=`translateX(-${$('graph-scroller').scrollLeft}px)`}

function setZoom(z){state.zoom=Math.max(.3,Math.min(1.8,z));$('tree-svg').style.width=Math.round(state.width*state.zoom)+'px';$('tree-svg').style.height=Math.round(state.height*state.zoom)+'px';$('zoom-level').textContent=Math.round(state.zoom*100)+'%';renderMoveRuler()}
function fitGraph(){const area=$('graph-scroller');if(!area.clientWidth)return;setZoom(Math.min(1.15,Math.max(.35,(area.clientWidth-30)/state.width)));area.scrollLeft=0;area.scrollTop=0;if(state.tree)focusedNode(0)}
function focusedNode(id){const x=state.coords.x[id],y=state.coords.y[id];if(x===undefined)return;const el=$('graph-scroller');el.scrollTo({left:Math.max(0,x*state.zoom-el.clientWidth*.38),top:Math.max(0,y*state.zoom-el.clientHeight*.45),behavior:'smooth'})}
function evalText(n){if(n.eval_mate!=null)return '#'+n.eval_mate;
 if(n.eval_cp!=null)return (n.eval_cp/100).toFixed(2);return 'Unavailable'}
function selectNode(id){
 if(!state.tree)return;const n=state.tree.nodes[id];if(!n||n.deleted)return;state.nodeID=id;
 $('selected-move').textContent=n.id===0?'Starting position':n.san;
 $('selected-ply').textContent=n.id===0?'Root':`Move ${Math.ceil(n.ply/2)} · ply ${n.ply}`;
 $('inspector-board').innerHTML=boardMarkup(n.fen,state.tree.config.color==='black',false);
 $('inspector-opening').textContent=n.opening_name?`${n.opening_eco||''} · ${n.opening_name}${n.opening_inherited?' (inherited classification)':''}`:'No named opening for this position';
 $('inspector-frequency').textContent=n.id===0?'—':`${(n.frequency||0).toFixed(1)}%`;
 $('inspector-games').textContent=(n.white||0)+(n.draws||0)+(n.black||0)?((n.white||0)+(n.draws||0)+(n.black||0)).toLocaleString('en-US'):'—';
 $('inspector-eval').textContent=evalText(n);
 $('inspector-eval-source').textContent=n.eval_source?`Source: ${n.eval_source}`:n.eval_error?`Engine unavailable: ${n.eval_error}`:state.tree.config.engine_mode==='practical'?'Statistics-only mode: engine evaluation disabled.':'No engine score for this position. Choose Cloud → Stockfish fallback with Stockfish installed.';
 const total=(n.white||0)+(n.draws||0)+(n.black||0),percent=x=>total?100*x/total:0;
 const w=percent(n.white||0),d=percent(n.draws||0),b=percent(n.black||0);
 $('inspector-white').textContent='White '+(total?w.toFixed(1)+'%':'—');
 $('inspector-draw').textContent='Draw '+(total?d.toFixed(1)+'%':'—');
 $('inspector-black').textContent='Black '+(total?b.toFixed(1)+'%':'—');
 const bars=$('inspector-wdl-bar').children;[w,d,b].forEach((v,i)=>bars[i].style.width=v+'%');
 const trap=n.trap_status||'unverified';$('inspector-trap').textContent=`Trap: ${trap}${n.trap_reason?' — '+n.trap_reason:''}`;
 $('inspector-trap').className='trap-label'+(trap==='yes'?' trap-yes':trap==='unverified'?' trap-unverified':'');
 let desc=0;function count(i){for(const ch of state.tree.nodes[i].children){if(!state.tree.nodes[ch].deleted){desc++;count(ch)}}}count(id);
 $('inspector-descendants').textContent=desc;
 $('inspector-note').textContent=n.id===0?'The starting position cannot be deleted. Select a continuation to remove it with all descendants.':n.transposition?`Transposition to node #${n.transposition_to??'?'}: continuation analysed only once.`:n.reason||`Selected branch includes ${desc+1} move nodes.`;
 $('prune-branch').disabled=n.id===0||state.lastPhase==='running';
 $('prune-branch').textContent=n.id===0?'Select a branch to remove':`Remove branch · ${desc+1} node${desc?'s':''}`;
 document.querySelectorAll('.node-card').forEach(el=>el.classList.toggle('selected',Number(el.dataset.node)===id));
}
async function prune(){if(!state.tree||state.nodeID===0)return;const n=state.tree.nodes[state.nodeID];if(!confirm(`Remove ${n.san} and all its descendants from the repertoire?\nYou can undo this action.`))return;
 try{const p=n.parent;const d=await api('/api/prune',{method:'POST',body:JSON.stringify({id:n.id})});await loadTree();selectNode(p);toast(`Removed ${d.removed} nodes. PGN export updated.`)}catch(e){toast(e.message,true)}}
async function undoPrune(){try{const d=await api('/api/undo',{method:'POST',body:'{}'});await loadTree();toast(`Restored ${d.restored} nodes.`)}catch(e){toast(e.message,true)}}
function bind(){
 $('tab-setup').onclick=()=>tab('setup');$('tab-results').onclick=()=>{tab('results');if(!state.tree)poll()};
 document.querySelectorAll('[data-color]').forEach(b=>b.onclick=()=>{state.color=b.dataset.color;document.querySelectorAll('[data-color]').forEach(t=>t.classList.toggle('active',t===b));drawBoard()});
 $('reset-board').onclick=async()=>{state.fen=initialFEN;state.history=[];await syncBoard()};
 $('flip-board').onclick=()=>{state.flipped=!state.flipped;drawBoard()};
 $('undo-move').onclick=async()=>{const p=state.history.pop();if(p){state.fen=p.fen;await syncBoard()}};
 $('load-fen').onclick=async()=>{const old=state.fen;state.fen=$('fen-input').value.trim();try{const d=await api('/api/board?fen='+encodeURIComponent(state.fen));state.fen=d.fen;state.history=[];await syncBoard();toast('Starting FEN updated.')}catch(e){state.fen=old;await syncBoard();toast(e.message,true)}};
 $('apply-moves').onclick=async()=>{let moves=$('moves-input').value.trim();if(!moves)return;try{const previous=state.fen;const d=await api('/api/sequence',{method:'POST',body:JSON.stringify({fen:previous,moves})});state.history.push({fen:previous,san:(d.moves||[]).map(x=>x.san).join(' ')});state.fen=d.fen;$('moves-input').value='';await syncBoard();toast('Move sequence applied.')}catch(e){toast(e.message,true)}};
 $('coverage').oninput=()=>$('coverage-label').textContent=$('coverage').value+'%';
 $('engine-mode').onchange=engineVisibility;$('check-stockfish').onclick=()=>checkEngine().catch(e=>toast(e.message,true));
 $('save-token').onclick=saveToken;$('forget-token').onclick=forgetToken;
 $('generate').onclick=start;
 $('cancel-analysis').onclick=async()=>{await api('/api/cancel',{method:'POST',body:'{}'});toast('Stop requested. Partial results will be retained.')};$('cancel-progress').onclick=$('cancel-analysis').onclick;
 $('prune-branch').onclick=prune;$('undo-prune').onclick=undoPrune;
 $('graph-scroller').addEventListener('scroll',syncRuler);
 $('view-depth').oninput=()=>{state.depthLimit=Number($('view-depth').value);$('view-depth-label').textContent=state.depthLimit;graphRender()};
 $('collapse-depth').oninput=()=>{state.collapseDepth=Number($('collapse-depth').value);$('collapse-depth-label').textContent=state.collapseDepth;state.expanded.clear();state.collapsed.clear();graphRender()};
 $('expand-all').onclick=()=>{state.collapseDepth=state.depthLimit+1;$('collapse-depth').value=state.collapseDepth;$('collapse-depth-label').textContent=state.collapseDepth;state.collapsed.clear();state.expanded.clear();graphRender()};
 $('collapse-all').onclick=()=>{state.collapseDepth=Math.min(8,state.depthLimit);$('collapse-depth').value=state.collapseDepth;$('collapse-depth-label').textContent=state.collapseDepth;state.expanded.clear();state.collapsed.clear();graphRender()};
 $('zoom-in').onclick=()=>setZoom(state.zoom*1.25);$('zoom-out').onclick=()=>setZoom(state.zoom/1.25);$('zoom-fit').onclick=fitGraph;
 $('node-search').onkeydown=e=>{if(e.key!=='Enter'||!state.tree)return;const q=e.target.value.toLowerCase().trim();if(!q)return;const n=state.tree.nodes.find(n=>!n.deleted&&n.san.toLowerCase().includes(q));if(n){state.depthLimit=Math.max(state.depthLimit,n.depth);$('view-depth').value=state.depthLimit;$('view-depth-label').textContent=state.depthLimit;let k=n.parent;while(k>=0){state.expanded.add(k);state.collapsed.delete(k);k=state.tree.nodes[k].parent;}graphRender();selectNode(n.id);focusedNode(n.id)}else{toast('No matching move found.',true)}};
}
async function init(){bind();engineVisibility();await loadMeta();await tokenStatus().catch(e=>toast(e.message,true));await syncBoard();await poll();setInterval(poll,1200)}
$('quit-application').addEventListener('click', () => quitApplication().catch(e => toast(e.message,true)));
// Browser window lifecycle. EventSource stays connected even in background tabs;
// unlike polling timers it is not paused by background-tab throttling.
let appBrowserSession = '';
let appBrowserEvents = null;
let appIsQuitting = false;
function connectAppLifecycle() {
  if (appIsQuitting || appBrowserEvents) return;
  const stream = new EventSource('/api/lifecycle/events');
  appBrowserEvents = stream;
  stream.addEventListener('session', event => { appBrowserSession = event.data; });
  stream.addEventListener('error', () => {
    // EventSource automatically reconnects after a transient interruption.
    // No secret or heartbeat is saved in browser storage.
    appBrowserSession = '';
  });
}
function releaseAppLifecycle() {
  const session = appBrowserSession;
  appBrowserSession = '';
  if (appBrowserEvents) { appBrowserEvents.close(); appBrowserEvents = null; }
  if (session) {
    // The beacon is best-effort; closing the SSE stream is the primary signal.
    const payload = new Blob([JSON.stringify({session})], {type:'application/json'});
    navigator.sendBeacon?.('/api/lifecycle/close', payload);
  }
}
window.addEventListener('pagehide', releaseAppLifecycle);
window.addEventListener('pageshow', event => { if (event.persisted) connectAppLifecycle(); });
async function quitApplication() {
  if (appIsQuitting) return;
  if (!appBrowserSession) { alert('Connecting to the local application. Please retry in a moment.'); return; }
  if (!confirm('Quit this application? Any running analysis will be cancelled.')) return;
  const response = await fetch('/api/lifecycle/quit', {
    method:'POST', headers:{'Content-Type':'application/json'},
    body:JSON.stringify({session:appBrowserSession}), cache:'no-store'
  });
  if (!response.ok) throw Error('Unable to close the application ('+response.status+').');
  appIsQuitting = true;
  releaseAppLifecycle();
  document.body.innerHTML = '<main style="min-height:100vh;display:grid;place-content:center;text-align:center;gap:12px;background:#111b20;color:#e7eedc;font:16px system-ui,sans-serif"><h1 style="margin:0">Application closed</h1><p style="margin:0;color:#9cb3a9">The local server has stopped. You can close this tab.</p></main>';
  try { window.close(); } catch (_) { /* Browsers generally restrict closing manually opened tabs. */ }
}
connectAppLifecycle();

init();
