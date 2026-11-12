// 使用随程序嵌入的 Dagre 计算分层布局，SVG 由安全的 DOM API 构造。
async function appendTopology(id, type) {
  const sectionNode = section('拓扑与访问链');
  const controls = el('div', 'controls topology-controls');
  const graphButton = el('button', 'ghost', '分层拓扑');
  const accessButton = el('button', 'ghost', '访问链');
  const canvas = el('div', 'topology-canvas');
  add(controls, graphButton, accessButton);
  add(sectionNode, controls, canvas);
  root.append(sectionNode);
  const base = `/api/${type === 'host' ? 'hosts' : 'services'}/${encodeURIComponent(id)}/topology`;
  async function load(view) {
    graphButton.classList.toggle('selected', view === 'graph');
    accessButton.classList.toggle('selected', view === 'access');
    clear(canvas).append(message('正在加载…'));
    try { drawTopology(canvas, await api(base + (view === 'access' ? '?view=access' : '')), view); }
    catch (e) { clear(canvas).append(message(e.message, 'error')); }
  }
  graphButton.onclick = () => load('graph');
  accessButton.onclick = () => load('access');
  await load('graph');
}

function drawTopology(target, data, view) {
  clear(target);
  const details = data.details?.length ? data.details : (data.nodes || []).map(id => ({id, name:id, type:id.split(':')[0], status:'在用'}));
  if (!details.length) { target.append(empty()); return; }
  const byID = new Map(details.map(n => [n.id, n]));
  const graph = new dagre.graphlib.Graph({multigraph:true});
  graph.setGraph({rankdir:view === 'access' ? 'LR' : 'TB', ranksep:75, nodesep:28, marginx:24, marginy:24});
  graph.setDefaultEdgeLabel(() => ({}));
  for (const n of details) graph.setNode(n.id, {width:166, height:64});
  for (const e of data.edges || []) if (byID.has(e.from) && byID.has(e.to)) graph.setEdge(e.from, e.to, {kind:e.kind}, e.kind);
  dagre.layout(graph);
  const svgNS = 'http://www.w3.org/2000/svg';
  const svg = document.createElementNS(svgNS, 'svg');
  svg.setAttribute('viewBox', `0 0 ${Math.ceil(graph.graph().width)} ${Math.ceil(graph.graph().height)}`);
  svg.setAttribute('width', String(Math.ceil(graph.graph().width)));
  svg.setAttribute('height', String(Math.ceil(graph.graph().height)));
  svg.setAttribute('role', 'img');
  svg.setAttribute('aria-label', t(view === 'access' ? '访问链' : '分层拓扑'));
  const line = (tag, attrs, text) => {const node=document.createElementNS(svgNS,tag);for(const [key,value] of Object.entries(attrs))node.setAttribute(key,String(value));if(text!=null)node.textContent=text;return node};
  const defs = line('defs', {});
  const marker = line('marker', {id:'topology-arrow', viewBox:'0 0 10 10', refX:9, refY:5, markerWidth:7, markerHeight:7, orient:'auto-start-reverse'});
  marker.append(line('path', {d:'M 0 0 L 10 5 L 0 10 z', fill:'#789088'}));defs.append(marker);svg.append(defs);
  for (const edge of graph.edges()) {
    const points=graph.edge(edge).points;
    const path=points.map((p,i)=>`${i?'L':'M'} ${p.x} ${p.y}`).join(' ');
    const group=line('g', {'class':'topology-edge'});
    group.append(line('path', {d:path, fill:'none', stroke:'#8aa99c', 'stroke-width':2, 'marker-end':'url(#topology-arrow)'}));
    group.append(line('title', {}, edge.name));svg.append(group);
  }
  for (const n of details) {
    const p=graph.node(n.id);
    const group=line('g', {'class':`topology-node status-${n.status === '停用' ? 'off' : n.status === '计划中' ? 'planned' : 'active'}${n.id===data.center?' center':''}`, tabindex:0, role:'link', 'aria-label':`${n.name} · ${n.kind||types[n.type]||n.type} · ${n.status}`});
    group.append(line('rect', {x:p.x-83,y:p.y-32,width:166,height:64,rx:10}));
    const short = [...n.name].slice(0,17).join('') + ([...n.name].length > 17 ? '…' : '');
    group.append(line('text', {x:p.x,y:p.y-5,'text-anchor':'middle','class':'topology-name'}, short));
    group.append(line('text', {x:p.x,y:p.y+17,'text-anchor':'middle','class':'topology-meta'}, `${n.kind||t(types[n.type]||n.type)} · ${t(n.status)}`));
    group.append(line('title', {}, `${n.name} (${n.id})`));
    group.addEventListener('click',()=>openObject(n.id,n.type));
    group.addEventListener('keydown',event=>{if(event.key==='Enter'||event.key===' '){event.preventDefault();openObject(n.id,n.type)}});
    svg.append(group);
  }
  target.append(svg);
}
