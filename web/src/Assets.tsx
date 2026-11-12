import React, { useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { Button } from './components/ui/button';
import { useTranslation } from 'react-i18next';

type Asset={id:string;type:string;name:string;status?:string;fields?:Record<string,unknown>};
type API=<T>(path:string)=>Promise<T>;
const queryClient=new QueryClient({defaultOptions:{queries:{retry:1,staleTime:0}}});
let root:Root|undefined;
const typeEn:Record<string,string>={location:'Sites',uplink:'Uplinks',host:'Hosts',network:'Networks',ip:'IP addresses',service:'Services',port:'Ports',endpoint:'Endpoints',dependency:'Dependencies',version:'Versions',credential:'Credential references',note:'Notes'};
const types:Record<string,string>={location:'站点',uplink:'网络接入',host:'主机',network:'网段',ip:'IP 地址',service:'服务',port:'端口',endpoint:'访问入口',dependency:'依赖',version:'版本',credential:'凭据引用',note:'笔记'};
export function unmountAssets(){if(root){root.unmount();root=undefined}}
export function mountAssets(target:Element,api:API,onCreate:(type:string)=>void,onOpen:(id:string,type:string)=>void,onNavigate:(view:string)=>void,initialType='location'){
  unmountAssets();root=createRoot(target);
  root.render(<QueryClientProvider client={queryClient}><Assets api={api} onCreate={onCreate} onOpen={onOpen} onNavigate={onNavigate} initialType={initialType}/></QueryClientProvider>);
}
function Assets({api,onCreate,onOpen,onNavigate,initialType}:{api:API;onCreate:(type:string)=>void;onOpen:(id:string,type:string)=>void;onNavigate:(view:string)=>void;initialType:string}){
  const [type,setType]=useState(initialType);
  const query=useQuery({queryKey:['assets',type],queryFn:()=>api<Asset[]>('/api/browse?type='+type)});
  const {i18n}=useTranslation();const en=i18n.language.startsWith('en');
  return <>
    <div className="hero"><p className="eyebrow">ROLLBOOK / {en?'Assets':'资产'}</p><h1>{en?'Manage assets':'资产维护'}</h1><p className="muted">{en?'View, create, and maintain catalog assets by type.':'按类型查看、新建和维护清册。'}</p></div>
    <div className="controls asset-shortcuts">{([['站点','sites'],['主机浏览树','hosts'],['网段','networks'],['服务','services']] as const).map(([label,view])=><Button key={view} variant="outline" onClick={()=>view==='services'?setType('service'):onNavigate(view)}>{en?({sites:'Sites',hosts:'Host tree',networks:'Networks',services:'Services'} as Record<string,string>)[view]:label}</Button>)}</div>
    <div className="controls asset-toolbar"><select className="field asset-type-select" aria-label={en?'Asset type':'资产类型'} value={type} onChange={event=>setType(event.target.value)}>{Object.entries(types).map(([key,label])=><option key={key} value={key}>{en?typeEn[key]:label}</option>)}</select><Button onClick={()=>onCreate(type)}>{en?'New':'新建'}</Button></div>
    {query.isLoading?<p className="notice">{en?'Loading…':'正在加载…'}</p>:query.isError?<div className="error" role="alert">{en?'Failed to load assets':'加载失败'} <Button variant="outline" onClick={()=>query.refetch()}>{en?'Retry':'重试'}</Button></div>:query.data?.length?<div className="grid">{query.data.map(item=><article key={item.id} className="card" tabIndex={0} role="button" onClick={()=>onOpen(item.id,item.type)} onKeyDown={event=>{if(event.key==='Enter')onOpen(item.id,item.type)}}><span className="tag">{en?typeEn[item.type]:types[item.type]}</span><div className="card-title">{item.name}</div><div className="muted">{item.status==='停用'?'停用 · ':''}{item.id}</div></article>)}</div>:<div className="empty">{en?'No records yet; create one here':'暂无记录，可从这里新建'}</div>}
  </>
}
