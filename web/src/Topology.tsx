import React, { useEffect, useMemo, useRef, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { useTranslation } from 'react-i18next';
import { mountFlow, unmountFlow } from './Flow.jsx';
import { drawLayers, siteOf, type Layers, type DrawGraph } from './topology-model.js';

type API=<T>(path:string,options?:RequestInit)=>Promise<T>;
type Graph={nodes:string[];details:{id:string;name:string;type:string;kind?:string}[];edges:{from:string;to:string;kind:string}[]};
type Impact={root:{id:string;name:string;type:string};hosts:{object:{id:string;name:string;type:string};path:string[]}[];services:{object:{id:string;name:string;type:string};path:string[]}[];endpoints:{object:{id:string;name:string;type:string};path:string[]}[]};
let root:Root|undefined;
export function unmountTopology(){unmountFlow();if(root){root.unmount();root=undefined}}
export function mountTopology(target:Element,api:API,open:(id:string,type:string)=>void){unmountTopology();root=createRoot(target);root.render(<Topology api={api} open={open}/>)}
function Topology({api,open}:{api:API;open:(id:string,type:string)=>void}){
 const {i18n}=useTranslation(),en=i18n.language.startsWith('en');const tx=(zh:string,english:string)=>en?english:zh;
 const initial=new URLSearchParams(location.search);
 const kindName=(kind?:string)=>en?({物理机:'Physical',虚拟机:'VM',网络设备:'Network device',容器:'Container',机柜:'Rack',房间:'Room',站点:'Site'} as Record<string,string>)[kind||'']||kind||'Host':kind||'主机';
 const [site,setSite]=useState(initial.get('site')||''),[focus,setFocus]=useState(initial.get('focus')||''),[view,setView]=useState<'assets'|'access'|'impact'>(initial.get('view')==='impact'?'impact':initial.get('view')==='access'?'access':'assets');
 const [level,setLevel]=useState<'physical'|'virtual'|'all'>('physical'),[expanded,setExpanded]=useState<Set<string>>(new Set()),[data,setData]=useState<Layers|null>(null),[graph,setGraph]=useState<DrawGraph|null>(null),[loading,setLoading]=useState(true),[error,setError]=useState('');
 const canvas=useRef<HTMLDivElement>(null),request=useRef(0);
 useEffect(()=>{let live=true;
  // 旧服务端过渡期使用三次 browse，避免前后端发布顺序造成空白拓扑。
  const fallback=()=>Promise.all(['location','host','service'].map(type=>api<{id:string;name:string;type:string;fields?:Record<string,string>}[]>('/api/browse?type='+type))).then(groups=>({nodes:groups.flat().map(item=>({id:item.id,name:item.name,type:item.type as 'location'|'host'|'service',kind:item.fields?.kind,parent_id:item.fields?.parent_host_id||item.fields?.parent_id,location_id:item.fields?.location_id,host_id:item.fields?.host_id})),edges:[]} as Layers));
  api<Layers>('/api/topology/layers').then(result=>Array.isArray(result?.nodes)?result:fallback()).catch(fallback).then(result=>{if(live){setData(result);setLoading(false)}}).catch(e=>{if(live){setError(String(e));setLoading(false)}});return()=>{live=false}},[]);
 useEffect(()=>{const handler=(event:Event)=>{const detail=(event as CustomEvent<{id:string;view:string}>).detail;if(detail?.id){setFocus(detail.id);setView(detail.view==='impact'?'impact':detail.view==='access'?'access':'assets')}};window.addEventListener('rollbook:topology-focus',handler);return()=>window.removeEventListener('rollbook:topology-focus',handler)},[]);
 const byID=useMemo(()=>new Map((data?.nodes||[]).map(n=>[n.id,n])),[data]);
 const sites=useMemo(()=>(data?.nodes||[]).filter(n=>n.type==='location'&&n.kind==='站点'),[data]);
 const choices=useMemo(()=>(data?.nodes||[]).filter(n=>(n.type==='host'||n.type==='service')&&(!site||siteOf(n,byID)===site)),[data,site,byID]);
 const grouped=useMemo(():DrawGraph=>{
  if(!data)return {details:[],edges:[]};
  const shown=sites.filter(n=>!site||n.id===site),details:DrawGraph['details']=[];
  const descendants=(id:string)=>{const seen=new Set<string>(),queue=[id];while(queue.length){const parent=queue.shift()!;for(const node of data.nodes)if((node.parent_id||node.host_id)===parent&&!seen.has(node.id)){seen.add(node.id);queue.push(node.id)}}return [...seen].map(key=>byID.get(key)!).filter(Boolean)};
  const toggle=(id:string)=>setExpanded(previous=>{const next=new Set(previous);if(next.has(id))next.delete(id);else next.add(id);return next});
  for(const currentSite of shown){
   const hostList=data.nodes.filter(n=>n.type==='host'&&siteOf(n,byID)===currentSite.id);
   const physical=hostList.filter(n=>!n.parent_id||['物理机','NAS','网络设备'].includes(n.kind||''));
   const groups=new Map<string,typeof physical>();for(const host of physical){const id=host.location_id||currentSite.id;groups.set(id,[...(groups.get(id)||[]),host])}
   const label=<div className="topology-site-card"><header><strong>{currentSite.name}</strong><small>{tx('站点','Site')} · {hostList.length} {tx('台主机','hosts')} · {data.nodes.filter(n=>n.type==='service'&&siteOf(n,byID)===currentSite.id).length} {tx('个服务','services')}</small></header><div className="topology-site-scroll">{[...groups].map(([locationID,hosts])=><section key={locationID}><h3>{byID.get(locationID)?.name||currentSite.name}</h3><div className="topology-host-grid">{hosts.map(host=>{const descendantsList=descendants(host.id),virtual=descendantsList.filter(n=>n.type==='host'),services=descendantsList.filter(n=>n.type==='service'),isOpen=expanded.has(host.id)||level!=='physical';return <div className="topology-host-card" key={host.id}><button type="button" onClick={e=>{e.stopPropagation();toggle(host.id)}} aria-expanded={isOpen}><strong>{host.name}</strong><small>{kindName(host.kind)}{host.ip?' · '+host.ip:''} · {virtual.length} {tx('台虚拟机','VMs')} · {services.length} {tx('个服务','services')}</small></button>{isOpen&&<div className="topology-host-children">{virtual.map(vm=><div key={vm.id}><button type="button" onClick={e=>{e.stopPropagation();toggle(vm.id)}}>{vm.name} · {kindName(vm.kind)}</button>{(level==='all'||expanded.has(vm.id))&&<small>{descendants(vm.id).filter(n=>n.type==='service').map(n=>n.name).join('、')}</small>}</div>)}{(level==='all'||expanded.has(host.id))&&data.nodes.filter(n=>n.type==='service'&&n.host_id===host.id).map(n=><button key={n.id} type="button" onClick={e=>{e.stopPropagation();setFocus(n.id)}}>{n.name} · {tx('服务','Service')}</button>)}</div>}</div>})}</div></section>)}</div></div>;
   details.push({id:currentSite.id,name:currentSite.name,type:'location',label,width:site?800:460,height:site?390:390});
  }
  const edges=new Map<string,{from:string;to:string;kind:string;count:number}>();for(const edge of data.edges){const a=byID.get(edge.from),b=byID.get(edge.to);if(!a||!b)continue;const from=siteOf(a,byID),to=siteOf(b,byID);if(!from||!to||from===to||!shown.some(n=>n.id===from)||!shown.some(n=>n.id===to))continue;const pair=[from,to].sort();const key=pair.join('|');const old=edges.get(key);if(old)old.count++;else edges.set(key,{from:pair[0],to:pair[1],kind:'站点互通',count:1})}
  return {details,edges:[...edges.values()]};
 },[data,site,sites,byID,expanded,level,en]);
 useEffect(()=>{const token=++request.current;setError('');if(!data)return;
  if(view==='assets'){setGraph(focus?drawLayers(data,site,level,expanded,focus):grouped);return}
  if(!focus){setGraph(null);return}
  const selected=byID.get(focus);if(!selected||!['host','service'].includes(selected.type)){setGraph(null);return}
  if(view==='access')api<Graph>('/api/'+(selected.type==='host'?'hosts':'services')+'/'+encodeURIComponent(focus)+'/topology?view=access&depth=1').then(result=>{if(token!==request.current)return;const allowed=new Set(result.nodes?.length?result.nodes:(result.details||[]).map(n=>n.id));setGraph({details:(result.details||[]).filter(n=>allowed.has(n.id)).map(n=>({...n,name:byID.get(n.id)?.url||(byID.get(n.id)?.ip?n.name+' · '+byID.get(n.id)?.ip:n.name),focus:n.id===focus})),edges:result.edges||[]})}).catch(e=>{if(token===request.current)setError(String(e))});
  else api<Impact>('/api/'+(selected.type==='host'?'hosts':'services')+'/'+encodeURIComponent(focus)+'/impact?recursive=1').then(result=>{if(token!==request.current)return;const items=[result.root,...(result.hosts||[]).map(x=>x.object),...(result.services||[]).map(x=>x.object),...(result.endpoints||[]).map(x=>x.object)];const details=[...new Map(items.map(n=>[n.id,{id:n.id,name:byID.get(n.id)?.url||n.name,type:n.type,kind:byID.get(n.id)?.kind,focus:n.id===focus}])).values()];const edges=new Map<string,{from:string;to:string;kind:string;count:number}>();for(const group of [result.hosts||[],result.services||[],result.endpoints||[]])for(const item of group){for(let i=0;i<item.path.length-1;i++){const from=item.path[i],to=item.path[i+1];if(!details.some(n=>n.id===from)||!details.some(n=>n.id===to))continue;const key=from+'|'+to;const old=edges.get(key);if(old)old.count++;else edges.set(key,{from,to,kind:'影响',count:1})}}setGraph({details,edges:[...edges.values()]})}).catch(e=>{if(token===request.current)setError(String(e))});
 },[data,site,level,expanded,focus,view,byID,grouped]);
 useEffect(()=>{unmountFlow();if(graph&&canvas.current)mountFlow(canvas.current,graph,(id,type)=>{setFocus(id);if(view==='assets'&&(type==='host'||type==='location'))setExpanded(previous=>{const next=new Set(previous);if(next.has(id))next.delete(id);else next.add(id);return next});else open(id,type)},'LR');return()=>unmountFlow()},[graph,view]);
 const updateURL=(nextSite=site,nextFocus=focus,nextView=view)=>{const params=new URLSearchParams();if(nextSite)params.set('site',nextSite);if(nextFocus)params.set('focus',nextFocus);if(nextView!=='assets')params.set('view',nextView);history.replaceState({},'', '/topology'+(params.size?'?'+params:''))};
 const chooseSite=(value:string)=>{setSite(value);if(focus&&value&&byID.get(focus)&&siteOf(byID.get(focus)!,byID)!==value){setFocus('');updateURL(value,'')}else updateURL(value)};
 const chooseFocus=(value:string)=>{setFocus(value);updateURL(site,value)};
 const chooseView=(value:'assets'|'access'|'impact')=>{setView(value);updateURL(site,focus,value)};
 const chooseLevel=(value:'physical'|'virtual'|'all')=>{if(value==='all'&&!focus&&!window.confirm(tx('全部展开可能产生大量节点，确定继续吗？','Expand every node? This may be a large graph.')))return;setLevel(value)};
 return <><div className="hero"><h1>{tx('拓扑','Topology')}</h1><p className="muted">{tx('选择站点并逐层展开；点击主机展开子资产，选择聚焦对象查看访问或影响。','Choose a site and expand its assets. Select a focus for access or impact.')}</p></div>
 <div className="topology-toolbar"><label>{tx('站点','Site')}<select aria-label={tx('站点','Site')} value={site} onChange={e=>chooseSite(e.target.value)}><option value="">{tx('全部站点','All sites')}</option>{sites.map(n=><option key={n.id} value={n.id}>{n.name}</option>)}</select></label><label>{tx('聚焦对象','Focus')}<select aria-label={tx('聚焦对象','Focus')} value={focus} onChange={e=>chooseFocus(e.target.value)}><option value="">{tx('请选择','Select')}</option>{choices.map(n=><option key={n.id} value={n.id}>{n.name}</option>)}</select></label><label>{tx('展开到','Expand to')}<select aria-label={tx('展开到','Expand to')} value={level} onChange={e=>chooseLevel(e.target.value as typeof level)}><option value="physical">{tx('物理机 / NAS','Physical / NAS')}</option><option value="virtual">{tx('虚拟机 / 容器','VM / container')}</option><option value="all">{tx('全部','All')}</option></select></label></div>
 <div className="topology-tabs" role="tablist" aria-label={tx('拓扑视图','Topology view')}>{([['assets','资产分层','Assets'],['access','访问链','Access path'],['impact','影响面','Impact']] as const).map(([value,zh,english])=><button key={value} role="tab" aria-selected={view===value} onClick={()=>chooseView(value)}>{tx(zh,english)}</button>)}</div>
 {loading&&<div className="notice">{tx('正在加载…','Loading…')}</div>}{error&&<div className="error" role="alert">{error}</div>}
 {!loading&&!error&&view!=='assets'&&!focus&&<div className="empty">{tx('先选择服务或主机，再查看'+(view==='access'?'访问链':'影响面'),'Select a service or host first')}</div>}
 {!loading&&!error&&view==='assets'&&!graph?.details.length&&<div className="empty">{tx('该站点暂无资产','No assets at this site')}</div>}
 <div ref={canvas} className="flow-container topology-flow"/>
 </>
}
