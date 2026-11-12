import React, { useEffect, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { Button } from './components/ui/button';

type Asset={id:string;type:string;name:string;status?:string;site_id?:string;network_id?:string;related_ids?:string[];fields?:Record<string,unknown>};
type API=<T>(path:string)=>Promise<T>;
const client=new QueryClient({defaultOptions:{queries:{retry:1}}});
let root:Root|undefined;
export function unmountSearch(){if(root){root.unmount();root=undefined}}
export function mountSearch(target:Element,api:API,onOpen:(id:string,type:string)=>void,initialTerm=''){unmountSearch();root=createRoot(target);root.render(<QueryClientProvider client={client}><Search api={api} onOpen={onOpen} initialTerm={initialTerm}/></QueryClientProvider>)}
function safeURL(value:unknown){try{const url=new URL(String(value));return ['http:','https:'].includes(url.protocol)&&url.host?url.href:null}catch{return null}}
function Search({api,onOpen,initialTerm}:{api:API;onOpen:(id:string,type:string)=>void;initialTerm:string}){
  const {t,i18n}=useTranslation();const [term,setTerm]=useState(initialTerm);const [debounced,setDebounced]=useState('');
  useEffect(()=>{const timer=setTimeout(()=>setDebounced(term.trim()),220);return ()=>clearTimeout(timer)},[term]);
  useEffect(()=>{const listener=(event:Event)=>setTerm((event as CustomEvent<string>).detail);window.addEventListener('rollbook:search',listener);if(innerWidth>760)document.querySelector<HTMLInputElement>('.search-box input')?.focus();return ()=>window.removeEventListener('rollbook:search',listener)},[]);
  const query=useQuery({queryKey:['search',debounced],enabled:!!debounced,queryFn:async()=>{
    const hits=await api<Asset[]>('/api/search?q='+encodeURIComponent(debounced));
    const ids=[...new Set(hits.flatMap(hit=>[hit.site_id,hit.network_id,hit.type==='endpoint'?hit.id:undefined,...(hit.related_ids||[])].filter(Boolean) as string[]))];
    const results=await Promise.allSettled(ids.map(id=>api<Asset>('/api/assets/'+encodeURIComponent(id))));
    const details=new Map<string,Asset>(ids.flatMap((id,index)=>results[index].status==='fulfilled'?[[id,(results[index] as PromiseFulfilledResult<Asset>).value] as [string,Asset]]:[]));
    const related=[...new Set(hits.flatMap(hit=>hit.related_ids||[]))].filter(id=>!hits.some(hit=>hit.id===id)).map(id=>details.get(id)).filter((item):item is Asset=>!!item&&['host','service','endpoint'].includes(item.type));
    return {hits,details,related};
  }});
  const en=i18n.language.startsWith('en');
  const card=(item:Asset,related=false)=>{
    const refs=related?[]:([['站点',item.site_id],['网段',item.network_id]] as const).filter(([,id])=>id&&query.data?.details.has(id)).map(([label,id])=>[label,query.data!.details.get(id!)] as const);
    const url=item.type==='endpoint'?item.fields?.url:undefined;const href=safeURL(url);
    return <article key={item.id} className="card" role="button" tabIndex={0} onClick={()=>onOpen(item.id,item.type)} onKeyDown={event=>{if(event.key==='Enter')onOpen(item.id,item.type)}}><span className="tag">{item.type}</span><span className={'tag status-badge'+(item.status==='停用'?' gray':'')}>{item.status==='停用'?(en?'Inactive':'停用'):(en?'Active':'在用')}</span><div className="card-title">{item.name||item.id}</div><div className="muted">{item.id}</div>{refs.map(([label,ref])=><div className="search-context" key={label}><span>{en?(label==='站点'?'Site':'Network'):label} {ref?.name}</span><span className="tag status-badge">{ref?.status==='停用'?(en?'Inactive':'停用'):(en?'Active':'在用')}</span></div>)}{url!=null&&url!==''&&<div className="search-url-line">{href?<a className="search-url" href={href} target="_blank" rel="noopener noreferrer" onClick={event=>event.stopPropagation()}>{String(url)}</a>:<span className="search-url">{String(url)}</span>}<Button variant="ghost" className="copy-button" aria-label={en?'Copy':'复制'} onClick={async event=>{event.stopPropagation();await navigator.clipboard.writeText(String(url))}}>{en?'Copy':'复制'}</Button></div>}</article>
  };
  return <><div className="hero"><p className="eyebrow">ROLLBOOK / {t('search')}</p><h1>{en?'Find a service or asset':'找到你需要的服务'}</h1><p className="muted">{en?'Search by IP, domain, port, service name, or alias.':'搜索 IP、域名、端口、服务名称或别名。'}</p></div><form className="search-box" onSubmit={event=>{event.preventDefault();setDebounced(term.trim())}}><input type="search" value={term} onChange={event=>setTerm(event.target.value)} placeholder={en?'For example 126, Overleaf, or a site name':'例如 126、Overleaf、香河'} autoComplete="off"/><button type="submit">{t('search')}</button></form><p className="hint">{en?'Try searching for “126” or a site name':'试试搜索 “126” 或 “香河”'}</p>{!debounced?null:query.isLoading?<div className="notice">{en?'Searching…':'正在搜索…'}</div>:query.isError?<div className="error" role="alert">{en?'Search failed':'搜索失败'} <Button variant="outline" onClick={()=>query.refetch()}>{en?'Retry':'重试'}</Button></div>:!query.data?.hits.length?<div className="empty">{en?'No matching records':'没有匹配的记录'}</div>:<><p className="muted">{en?`Found ${query.data.hits.length} results`:`找到 ${query.data.hits.length} 条结果`}</p><div className="grid">{query.data.hits.map(hit=>card(hit))}</div>{!!query.data.related.length&&<section className="section"><h2>{en?'Related hosts, services and endpoints':'相关主机、服务与入口'}</h2><div className="grid">{query.data.related.map(hit=>card(hit,true))}</div></section>}</>}</>
}
