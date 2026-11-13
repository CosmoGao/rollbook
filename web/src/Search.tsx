import React, { useEffect, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Button } from './components/ui/button';

type Asset={id:string;type:string;name:string;status?:string;site_id?:string;site_name?:string;host_id?:string;host_name?:string;primary_ip?:string;kind?:string;services?:string[];endpoints?:string[]};
type API=<T>(path:string)=>Promise<T>;
const client=new QueryClient({defaultOptions:{queries:{retry:1}}});
let root:Root|undefined;
export function unmountSearch(){if(root){root.unmount();root=undefined}}
export function mountSearch(target:Element,api:API,onOpen:(id:string,type:string)=>void,initialTerm=''){unmountSearch();root=createRoot(target);root.render(<QueryClientProvider client={client}><Search api={api} onOpen={onOpen} initialTerm={initialTerm}/></QueryClientProvider>)}
const typeZh:Record<string,string>={ip:'IP 地址',host:'主机',service:'服务',endpoint:'入口',location:'位置',dependency:'依赖',version:'版本',port:'端口'};
const typeEn:Record<string,string>={ip:'IP address',host:'Host',service:'Service',endpoint:'Endpoint',location:'Location',dependency:'Dependency',version:'Version',port:'Port'};
function safeURL(value:string){try{const url=new URL(value);return ['http:','https:'].includes(url.protocol)&&url.host&&!url.username&&!url.password?url.href:null}catch{return null}}
function Search({api,onOpen,initialTerm}:{api:API;onOpen:(id:string,type:string)=>void;initialTerm:string}){
  const {t,i18n}=useTranslation();const en=i18n.language.startsWith('en');const [term,setTerm]=useState(initialTerm);const [debounced,setDebounced]=useState('');
  useEffect(()=>{const timer=setTimeout(()=>setDebounced(term.trim()),220);return ()=>clearTimeout(timer)},[term]);
  useEffect(()=>{const listener=(event:Event)=>setTerm((event as CustomEvent<string>).detail);window.addEventListener('rollbook:search',listener);if(innerWidth>760)document.querySelector<HTMLInputElement>('.search-box input')?.focus();return ()=>window.removeEventListener('rollbook:search',listener)},[]);
  const update=(value:string)=>{setTerm(value);window.dispatchEvent(new CustomEvent('rollbook:search-term',{detail:value}))};
  const query=useQuery({queryKey:['search',debounced],enabled:!!debounced,queryFn:()=>api<Asset[]>('/api/search?q='+encodeURIComponent(debounced))});
  const card=(item:Asset)=> <article key={item.id} className="card search-answer" role="button" tabIndex={0} onClick={()=>onOpen(item.id,item.type)} onKeyDown={event=>{if(event.key==='Enter')onOpen(item.id,item.type)}}>
    <span className="tag">{(en?typeEn:typeZh)[item.type]||item.type}</span><span className={'tag status-badge'+(item.status==='停用'?' gray':'')}>{item.status==='停用'?(en?'Inactive':'停用'):(en?'Active':'在用')}</span>
    <div className="card-title">{item.name||item.id}</div>
    {item.site_name&&<div className="search-context">{en?'Site':'所属站点'}：{item.site_name}</div>}
    {item.host_name&&item.type!=='host'&&<div className="search-context">{en?'Host':'宿主'}：{item.host_name}</div>}
    {item.primary_ip&&<div className="search-context">{en?'Primary IP':'主 IP'}：{item.primary_ip}</div>}
    {!!item.services?.length&&<div className="search-context">{en?'Services':'运行服务'}：{item.services.join('、')}</div>}
    {!!item.endpoints?.length&&<div className="search-context">{en?'Access':'访问方式'}：{item.endpoints.map(address=><span className="search-url-line" key={address}>{safeURL(address)?<a href={safeURL(address)!} target="_blank" rel="noopener noreferrer" onClick={event=>event.stopPropagation()}>{address}</a>:address}<Button variant="ghost" className="copy-button" aria-label={en?'Copy':'复制'} onClick={async event=>{event.stopPropagation();await navigator.clipboard.writeText(address)}}>{en?'Copy':'复制'}</Button></span>)}</div>}
    {item.kind&&<div className="muted">{item.kind}</div>}
  </article>;
  const hits=query.data||[];const primary=hits.filter(item=>!['dependency','version','port'].includes(item.type));const secondary=hits.filter(item=>['dependency','version','port'].includes(item.type));
  return <><div className="hero"><h1>{en?'Find a service or asset':'找到你需要的服务'}</h1><p className="muted">{en?'Search by IP, domain, port, service name, or alias.':'搜索 IP、域名、端口、服务名称或别名。'}</p></div><form className="search-box" onSubmit={event=>{event.preventDefault();setDebounced(term.trim())}}><input type="search" value={term} onChange={event=>update(event.target.value)} placeholder={en?'IP, service, or site name':'IP、服务或站点名称'} autoComplete="off"/><button type="submit">{t('search')}</button></form>{!debounced?null:query.isLoading?<div className="notice">{en?'Searching…':'正在搜索…'}</div>:query.isError?<div className="error" role="alert">{en?'Search failed':'搜索失败'} <Button variant="outline" onClick={()=>query.refetch()}>{en?'Retry':'重试'}</Button></div>:!hits.length?<div className="empty">{en?'No matching records':'没有匹配的记录'}</div>:<><p className="muted">{en?`Found ${hits.length} results`:`找到 ${hits.length} 条结果`}</p><div className="grid">{(primary.length?primary:secondary).map(card)}</div>{!!primary.length&&!!secondary.length&&<details className="section search-secondary"><summary>{en?'Dependencies, versions and ports':'依赖、版本与端口'}（{secondary.length}）</summary><div className="grid">{secondary.map(card)}</div></details>}</>}</>;
}
