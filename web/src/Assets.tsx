import React, { useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { Button } from './components/ui/button';
import { Tabs, CatalogTable, SkeletonRows } from './components/ui/catalog';
import { HostTree, TreeAsset } from './components/HostTree';
import { useTranslation } from 'react-i18next';

type Asset={id:string;type:string;name:string;status?:string;site_id?:string;site_name?:string;primary_ip?:string;kind?:string;fields?:Record<string,unknown>};
type API=<T>(path:string)=>Promise<T>;
const client=new QueryClient({defaultOptions:{queries:{retry:1,staleTime:0}}});
let root:Root|undefined;
const typeEn:Record<string,string>={location:'Sites',uplink:'Uplinks',host:'Hosts',network:'Networks',ip:'IP addresses',service:'Services',port:'Ports',endpoint:'Endpoints',dependency:'Dependencies',version:'Versions',credential:'Credential references',note:'Notes'};
const types:Record<string,string>={location:'站点',uplink:'网络接入',host:'主机',network:'网段',ip:'IP 地址',service:'服务',port:'端口',endpoint:'访问入口',dependency:'依赖',version:'版本',credential:'凭据引用',note:'笔记'};
const tabs=['location','host','network','service'];
export function unmountAssets(){if(root){root.unmount();root=undefined}}
export function mountAssets(target:Element,api:API,onCreate:(type:string)=>void,onOpen:(id:string,type:string)=>void,enumLabel:(field:string,value:string)=>string,initialType='location',initialView:'list'|'tree'='list'){
  unmountAssets();root=createRoot(target);
  root.render(<QueryClientProvider client={client}><Assets api={api} onCreate={onCreate} onOpen={onOpen} enumLabel={enumLabel} initialType={initialType} initialView={initialView}/></QueryClientProvider>);
}
function Assets({api,onCreate,onOpen,enumLabel,initialType,initialView}:{api:API;onCreate:(type:string)=>void;onOpen:(id:string,type:string)=>void;enumLabel:(field:string,value:string)=>string;initialType:string;initialView:'list'|'tree'}){
  const [type,setType]=useState(initialType),[search,setSearch]=useState(''),[view,setView]=useState(initialView);
  const query=useQuery({queryKey:['assets',type],queryFn:()=>api<Asset[]>('/api/browse?type='+type)});
  const treeEnabled=type==='host'&&view==='tree';
  const services=useQuery({queryKey:['host-tree','service'],queryFn:()=>api<TreeAsset[]>('/api/browse?type=service'),enabled:treeEnabled});
  const ips=useQuery({queryKey:['host-tree','ip'],queryFn:()=>api<TreeAsset[]>('/api/browse?type=ip'),enabled:treeEnabled});
  const {i18n}=useTranslation();const en=i18n.language.startsWith('en');
  const label=(value:string)=>en?typeEn[value]:types[value];
  const items=(query.data||[]).filter(item=>(item.name+' '+item.id+' '+(item.site_name||'')).toLocaleLowerCase().includes(search.toLocaleLowerCase()));
  const locationTree=(entries:Asset[])=>{const seen=new Set<string>();const node=(item:Asset,depth=0):React.ReactNode=>{if(seen.has(item.id))return null;seen.add(item.id);return <details className="tree-node" key={item.id} open={depth<2}><summary><button className="tree-title" onClick={event=>{event.preventDefault();onOpen(item.id,item.type)}}>{item.name}</button><span className="tree-meta">{enumLabel('kind',item.kind||String(item.fields?.kind||''))}</span></summary><div className="tree-children">{entries.filter(child=>child.fields?.parent_id===item.id).map(child=>node(child,depth+1))}</div></details>};return <div className="host-tree">{entries.filter(item=>!item.fields?.parent_id||!entries.some(parent=>parent.id===item.fields?.parent_id)).map(item=>node(item))}{entries.filter(item=>!seen.has(item.id)).map(item=>node(item))}</div>};
  return <>
    <div className="hero page-heading"><div><h1>{en?'Manage assets':'资产维护'}</h1><p className="muted">{en?'View, create, and maintain catalog assets by type.':'按类型查看、新建和维护清册。'}</p></div><Button onClick={()=>onCreate(type)}>{en?'New':'新建'}</Button></div>
    <div className="asset-shortcuts"><Tabs items={tabs.map(value=>({value,label:label(value)}))} value={type} onChange={value=>{setType(value);setView('list');setSearch('')}}/></div>
    <div className="filter-toolbar asset-toolbar"><div className="filter-fields"><input type="search" value={search} onChange={event=>setSearch(event.target.value)} placeholder={en?'Search assets':'搜索资产'} aria-label={en?'Search assets':'搜索资产'}/><select className="field asset-type-select" aria-label={en?'Other asset types':'其他资产类型'} value={tabs.includes(type)?'':type} onChange={event=>{if(event.target.value){setType(event.target.value);setView('list');setSearch('')}}}><option value="">{en?'Other types':'其他类型'}</option>{Object.entries(types).filter(([key])=>!tabs.includes(key)).map(([key])=><option key={key} value={key}>{label(key)}</option>)}</select></div>{type==='host'&&<div className="view-switch" role="group" aria-label={en?'Host view':'主机视图'}><Button variant={view==='list'?'default':'outline'} aria-pressed={view==='list'} onClick={()=>setView('list')}>{en?'List':'列表'}</Button><Button variant={view==='tree'?'default':'outline'} aria-pressed={view==='tree'} onClick={()=>setView('tree')}>{en?'Tree':'树'}</Button></div>}</div>
    {query.isLoading||treeEnabled&&(services.isLoading||ips.isLoading)?<SkeletonRows/>:query.isError||treeEnabled&&(services.isError||ips.isError)?<div className="error" role="alert">{en?'Failed to load assets':'加载失败'} <Button variant="outline" onClick={()=>{query.refetch();services.refetch();ips.refetch()}}>{en?'Retry':'重试'}</Button></div>:items.length&&type==='location'?locationTree(items):items.length&&treeEnabled?<HostTree hosts={items as TreeAsset[]} services={services.data||[]} ips={ips.data||[]} open={onOpen} enumLabel={enumLabel} en={en}/>:items.length?<CatalogTable headers={[en?'Name':'名称',en?'Kind':'类别',en?'Site':'站点',en?'Primary IP':'主 IP',en?'Status':'状态']}>
      {items.map(item=><tr className="card" key={item.id} tabIndex={0} onClick={()=>onOpen(item.id,item.type)} onKeyDown={event=>{if(event.key==='Enter')onOpen(item.id,item.type)}}><td className="table-name">{item.name}</td><td>{item.kind?enumLabel('kind',item.kind):label(item.type)||item.type}</td><td>{item.site_name||'—'}</td><td>{item.primary_ip||'—'}</td><td><span className={'tag'+(item.status==='停用'?' gray':'')}>{item.status==='停用'?(en?'Inactive':'停用'):(en?'Active':'在用')}</span></td></tr>)}
    </CatalogTable>:<div className="empty">{en?'No records yet; create one here':'暂无记录，可从这里新建'}</div>}
  </>;
}
