import React, { useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { Button } from './components/ui/button';
import { Tabs, CatalogTable, SkeletonRows } from './components/ui/catalog';
import { useTranslation } from 'react-i18next';

type Asset={id:string;type:string;name:string;status?:string;fields?:Record<string,unknown>};
type API=<T>(path:string)=>Promise<T>;
const client=new QueryClient({defaultOptions:{queries:{retry:1,staleTime:0}}});
let root:Root|undefined;
const typeEn:Record<string,string>={location:'Sites',uplink:'Uplinks',host:'Hosts',network:'Networks',ip:'IP addresses',service:'Services',port:'Ports',endpoint:'Endpoints',dependency:'Dependencies',version:'Versions',credential:'Credential references',note:'Notes'};
const types:Record<string,string>={location:'站点',uplink:'网络接入',host:'主机',network:'网段',ip:'IP 地址',service:'服务',port:'端口',endpoint:'访问入口',dependency:'依赖',version:'版本',credential:'凭据引用',note:'笔记'};
const tabs=['location','host','network','service'];
export function unmountAssets(){if(root){root.unmount();root=undefined}}
export function mountAssets(target:Element,api:API,onCreate:(type:string)=>void,onOpen:(id:string,type:string)=>void,onNavigate:(view:string)=>void,initialType='location'){
  unmountAssets();root=createRoot(target);
  root.render(<QueryClientProvider client={client}><Assets api={api} onCreate={onCreate} onOpen={onOpen} onNavigate={onNavigate} initialType={initialType}/></QueryClientProvider>);
}
function Assets({api,onCreate,onOpen,onNavigate,initialType}:{api:API;onCreate:(type:string)=>void;onOpen:(id:string,type:string)=>void;onNavigate:(view:string)=>void;initialType:string}){
  const [type,setType]=useState(initialType),[search,setSearch]=useState('');
  const query=useQuery({queryKey:['assets',type],queryFn:()=>api<Asset[]>('/api/browse?type='+type)});
  const {i18n}=useTranslation();const en=i18n.language.startsWith('en');
  const label=(value:string)=>en?typeEn[value]:types[value];
  const items=(query.data||[]).filter(item=>(item.name+' '+item.id).toLocaleLowerCase().includes(search.toLocaleLowerCase()));
  return <>
    <div className="hero page-heading"><div><h1>{en?'Manage assets':'资产维护'}</h1><p className="muted">{en?'View, create, and maintain catalog assets by type.':'按类型查看、新建和维护清册。'}</p></div><Button onClick={()=>onCreate(type)}>{en?'New':'新建'}</Button></div>
    <div className="asset-shortcuts"><Tabs items={tabs.map(value=>({value,label:label(value)}))} value={type} onChange={value=>{setType(value);setSearch('')}}/></div>
    <div className="filter-toolbar asset-toolbar"><div className="filter-fields"><input type="search" value={search} onChange={event=>setSearch(event.target.value)} placeholder={en?'Search assets':'搜索资产'} aria-label={en?'Search assets':'搜索资产'}/><select className="field asset-type-select" aria-label={en?'Asset type':'资产类型'} value={type} onChange={event=>setType(event.target.value)}>{Object.entries(types).map(([key])=><option key={key} value={key}>{label(key)}</option>)}</select></div>{type==='host'&&<div className="view-switch"><Button variant="outline" onClick={()=>onNavigate('hosts')}>{en?'Host tree':'主机浏览树'}</Button></div>}</div>
    {query.isLoading?<SkeletonRows/>:query.isError?<div className="error" role="alert">{en?'Failed to load assets':'加载失败'} <Button variant="outline" onClick={()=>query.refetch()}>{en?'Retry':'重试'}</Button></div>:items.length?<CatalogTable headers={[en?'Name':'名称',en?'Type':'类型',en?'Status':'状态','ID']}>
      {items.map(item=><tr className="card" key={item.id} tabIndex={0} onClick={()=>onOpen(item.id,item.type)} onKeyDown={event=>{if(event.key==='Enter')onOpen(item.id,item.type)}}><td className="table-name">{item.name}</td><td>{label(item.type)||item.type}</td><td><span className={'tag'+(item.status==='停用'?' gray':'')}>{item.status==='停用'?(en?'Inactive':'停用'):(en?'Active':'在用')}</span></td><td className="muted">{item.id}</td></tr>)}
    </CatalogTable>:<div className="empty">{en?'No records yet; create one here':'暂无记录，可从这里新建'}</div>}
  </>;
}
