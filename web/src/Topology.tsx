import React, { useEffect, useRef, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { useTranslation } from 'react-i18next';
import { mountFlow, unmountFlow } from './Flow.jsx';

type API=<T>(path:string,options?:RequestInit)=>Promise<T>;
type Asset={id:string;name:string;site_id?:string;fields?:{kind?:string;location_id?:string;host_id?:string;parent_id?:string}};
type Graph={nodes:string[];details:{id:string;name:string;type:string}[];edges:{from:string;to:string;kind:string}[]};
let root:Root|undefined;
export function unmountTopology(){unmountFlow();if(root){root.unmount();root=undefined}}
export function mountTopology(target:Element,api:API,open:(id:string,type:string)=>void){unmountTopology();root=createRoot(target);root.render(<Topology api={api} open={open}/>)}
function Topology({api,open}:{api:API;open:(id:string,type:string)=>void}){
  const {i18n}=useTranslation(),en=i18n.language.startsWith('en');
  const tx=(zh:string,english:string)=>en?english:zh;
  const [services,setServices]=useState<Asset[]>([]),[sites,setSites]=useState<Asset[]>([]),[hosts,setHosts]=useState<Asset[]>([]),[site,setSite]=useState(new URLSearchParams(location.search).get('site')||''),[service,setService]=useState(''),[view,setView]=useState<'graph'|'access'>('graph'),[loading,setLoading]=useState(true),[error,setError]=useState(''),[partial,setPartial]=useState(false);
  const canvas=useRef<HTMLDivElement>(null),seq=useRef(0);
  useEffect(()=>{let live=true;Promise.all(['service','location','host'].map(type=>api<Asset[]>('/api/browse?type='+type))).then(([s,l,h])=>{if(live){setServices(s);setSites(l);setHosts(h);setLoading(false)}}).catch(e=>{if(live){setError(String(e));setLoading(false)}});return()=>{live=false}},[]);
  const locations=new Map(sites.map(item=>[item.id,item]));
  const hostLocations=new Map(hosts.map(host=>[host.id,host.fields?.location_id]));
  const belongsToSite=(locationID?:string)=>{const visited=new Set<string>();while(locationID&&!visited.has(locationID)){if(locationID===site)return true;visited.add(locationID);locationID=locations.get(locationID)?.fields?.parent_id}return false};
  const visible=services.filter(item=>!site||item.site_id===site||belongsToSite(item.fields?.location_id||hostLocations.get(item.fields?.host_id||'')));
  useEffect(()=>{const token=++seq.current;unmountFlow();setPartial(false);setError('');if(loading)return;if(view==='access'&&!service)return;const ids=service?[service]:visible.map(item=>item.id);if(!ids.length)return;
    Promise.allSettled(ids.map(id=>api<Graph>('/api/services/'+encodeURIComponent(id)+'/topology'+(view==='access'?'?view=access':'')))).then(results=>{if(token!==seq.current)return;const graphs=results.filter((result):result is PromiseFulfilledResult<Graph>=>result.status==='fulfilled').map(result=>result.value);if(!graphs.length){setError(String((results[0] as PromiseRejectedResult)?.reason?.message||tx('拓扑加载失败','Failed to load topology')));return}const details=new Map<string,Graph['details'][number]>(),edges=new Map<string,Graph['edges'][number]>();for(const graph of graphs){for(const node of graph.details||[])details.set(node.id,node);for(const id of graph.nodes||[])if(!details.has(id))details.set(id,{id,name:id,type:id.split(':')[0]});for(const edge of graph.edges||[])edges.set(edge.from+'|'+edge.to+'|'+edge.kind,edge)}if(canvas.current)mountFlow(canvas.current,{details:[...details.values()],edges:[...edges.values()]},open,view==='access'?'LR':'TB');setPartial(graphs.length<ids.length)});return()=>{seq.current++;unmountFlow()};
  },[site,service,view,loading,services,hosts]);
  const chooseSite=(value:string)=>{setSite(value);setService('');history.replaceState({},'',value?'/topology?site='+encodeURIComponent(value):'/topology')};
  return <><div className="hero"><h1>{tx('拓扑','Topology')}</h1><p className="muted">{tx('按站点筛选整体拓扑，或选择服务查看访问链。','Filter the topology by site, or select a service to inspect its access path.')}</p></div><div className="controls"><select aria-label={tx('站点','Site')} value={site} onChange={event=>chooseSite(event.target.value)}><option value="">{tx('全部站点','All sites')}</option>{sites.filter(item=>item.fields?.kind==='站点').map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select><select aria-label={tx('服务','Service')} value={service} onChange={event=>setService(event.target.value)}><option value="">{tx('整体拓扑','Whole topology')}</option>{visible.map(item=><option key={item.id} value={item.id}>{item.name}</option>)}</select><select aria-label={tx('拓扑视图','Topology view')} value={view} onChange={event=>setView(event.target.value as 'graph'|'access')}><option value="graph">{tx('分层拓扑','Layered topology')}</option><option value="access">{tx('访问链','Access path')}</option></select></div>{loading&&<div className="notice">{tx('正在加载…','Loading…')}</div>}{error&&<div className="error" role="alert">{error}</div>}{partial&&<div className="error">{tx('部分服务拓扑加载失败','Some service topologies failed to load')}</div>}{!loading&&!error&&view==='access'&&!service&&<div className="empty">{tx('请选择服务查看访问链','Select a service to view its access path')}</div>}{!loading&&!error&&view==='graph'&&!visible.length&&<div className="empty">{tx('暂无记录','No records yet')}</div>}<div ref={canvas} className="flow-container"/></>
}
