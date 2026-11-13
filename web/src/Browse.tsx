import React, { useEffect, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { Button } from './components/ui/button';
import { CatalogTable } from './components/ui/catalog';
import { useTranslation } from 'react-i18next';

type Asset={id:string;type:string;name:string;status?:string;fields?:Record<string,any>};
type API=<T>(path:string)=>Promise<T>;
let root:Root|undefined;
export function unmountBrowse(){if(root){root.unmount();root=undefined}}
export function mountBrowse(target:Element,view:'sites'|'hosts'|'networks',api:API,open:(id:string,type:string)=>void,enumLabel:(field:string,value:string)=>string){unmountBrowse();root=createRoot(target);root.render(<Browse {...{view,api,open,enumLabel}}/>)}
function Browse({view,api,open,enumLabel}:{view:'sites'|'hosts'|'networks';api:API;open:(id:string,type:string)=>void;enumLabel:(field:string,value:string)=>string}){
  const {i18n}=useTranslation(),en=i18n.language.startsWith('en'),tx=(zh:string,english:string)=>en?english:zh;
  const [mode,setMode]=useState<'list'|'tree'>('tree');
  const [items,setItems]=useState<Asset[]>([]),[services,setServices]=useState<Asset[]>([]),[ips,setIPs]=useState<Asset[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState('');
  const load=()=>{setLoading(true);setError('');const type={sites:'location',hosts:'host',networks:'network'}[view];Promise.all([api<Asset[]>('/api/browse?type='+type),...(view==='hosts'?['service','ip'].map(value=>api<Asset[]>('/api/browse?type='+value)):[])]).then(([assets,other,addresses])=>{setItems(view==='sites'?assets.filter(item=>item.fields?.kind==='站点'):assets);setServices(other||[]);setIPs(addresses||[]);setLoading(false)}).catch(e=>{setError(String(e));setLoading(false)})};
  useEffect(load,[view]);
  const byHost=(list:Asset[],host:string)=>list.filter(item=>item.fields?.host_id===host);
  const rank=(item:Asset)=>({物理机:0,NAS:0,虚拟机:1,LXC:2,容器:2,网络设备:3} as Record<string,number>)[item.fields?.kind]??4;
  const children=(host:Asset)=>[...items.filter(item=>item.fields?.parent_host_id===host.id).sort((a,b)=>rank(a)-rank(b)||a.name.localeCompare(b.name)),...byHost(services,host.id)];
  const seen=new Set<string>();
  const node=(item:Asset,depth=0):React.ReactNode=>{if(seen.has(item.id))return null;seen.add(item.id);const ipHost=item.type==='service'?item.fields?.host_id:item.id;const own=byHost(ips,ipHost).map(address=>address.fields?.address).join(', ')||'—';return <details className="tree-node" key={item.id} open={depth<2}><summary><span className="tree-title" onClick={event=>{event.preventDefault();open(item.id,item.type)}}>{item.name}</span><span className="tree-meta">{item.type==='host'?enumLabel('kind',item.fields?.kind):tx('服务','Service')} · IP {own} · {enumLabel('status',item.status||'')}</span><Button variant="ghost" onClick={event=>{event.preventDefault();open(item.id,item.type)}}>{tx('查看','View')}</Button></summary><div className="tree-children">{children(item).map(child=>node(child,depth+1))}</div></details>};
  const roots=items.filter(item=>!item.fields?.parent_host_id||!items.some(host=>host.id===item.fields?.parent_host_id)).sort((a,b)=>rank(a)-rank(b)||a.name.localeCompare(b.name));
  const title={sites:tx('站点','Sites'),hosts:tx('主机','Hosts'),networks:tx('网段','Networks')}[view];
  return <><div className="hero"><h1>{title}</h1><p className="muted">{view==='sites'?tx('查看接入、网段与路由设备。','View uplinks, networks and routers.'):tx('按资产浏览清册。','Browse the catalog by asset.')}</p></div>{view==='hosts'&&<div className="filter-toolbar"><div className="view-switch"><Button variant={mode==='list'?'default':'outline'} onClick={()=>setMode('list')}>{tx('列表','List')}</Button><Button variant={mode==='tree'?'default':'outline'} onClick={()=>setMode('tree')}>{tx('树','Tree')}</Button></div></div>}{loading?<div className="notice">{tx('正在加载…','Loading…')}</div>:error?<div className="error" role="alert">{error} <Button onClick={load}>{tx('重试','Retry')}</Button></div>:!items.length?<div className="empty">{tx('暂无记录','No records yet')}</div>:view==='hosts'&&mode==='list'?<CatalogTable headers={[tx('名称','Name'),tx('类型','Type'),tx('状态','Status'),'ID']}>{items.map(item=><tr key={item.id} className="card" onClick={()=>open(item.id,item.type)}><td className="table-name">{item.name}</td><td>{enumLabel('kind',item.fields?.kind||'')}</td><td>{enumLabel('status',item.status||'')}</td><td>{item.id}</td></tr>)}</CatalogTable>:view==='hosts'?<div className="host-tree">{roots.map(item=>node(item))}{items.filter(item=>!seen.has(item.id)).map(item=>node(item))}</div>:<div className="grid">{items.map(item=><article className="card" key={item.id} tabIndex={0} role="button" onClick={()=>open(item.id,item.type)} onKeyDown={event=>{if(event.key==='Enter')open(item.id,item.type)}}><span className="tag">{title}</span><div className="card-title">{item.name}</div><div className="muted">{view==='networks'?item.fields?.cidr:item.id}</div></article>)}</div>}</>
}
