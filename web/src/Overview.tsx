import React from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery, useQueries } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';

const queryClient = new QueryClient({defaultOptions:{queries:{retry:1,staleTime:10000}}});
type Item = {id:string;name?:string;created_at?:string;fields?:Record<string,unknown>};
type API = <T>(path:string)=>Promise<T>;
let root:Root|undefined;
export function unmountOverview(){if(root){root.unmount();root=undefined}}
export function mountOverview(target:Element,api:API,onNavigate:(view:string)=>void){
  unmountOverview();root=createRoot(target);
  root.render(<QueryClientProvider client={queryClient}><Overview api={api} onNavigate={onNavigate}/></QueryClientProvider>);
}
function Overview({api,onNavigate}:{api:API;onNavigate:(view:string)=>void}){
  const {t} = useTranslation();
  const counts = [
    ['待确认','/api/changes?state='+encodeURIComponent('待确认'),'pending'],['待核实','/api/assets/unverified','unverified'],['有新版本','/api/versions/outdated','outdated'],
    ['站点','/api/browse?type=location','sites'],['主机','/api/browse?type=host','hosts'],['网段','/api/browse?type=network','networks'],['服务','/api/browse?type=service','services'],
  ] as const;
  const queries=useQueries({queries:counts.map(([,path])=>({queryKey:['overview',path],queryFn:()=>api<Item[]>(path)}))});
  const changes=useQuery({queryKey:['overview','changes'],queryFn:()=>api<Item[]>('/api/changes')});
  const contexts=useQuery({queryKey:['overview','contexts'],queryFn:()=>api<Item[]>('/api/contexts?archived=false')});
  return <>
    <div className="hero"><h1>{t('overview')}</h1><p className="muted">{t('服务与资产清册')}</p></div>
    <div className="grid overview-grid">{counts.map(([label,,view],index)=>{
      const q=queries[index];const items=q.data||[];const count=view==='sites'?items.filter(item=>item.fields?.kind==='站点').length:items.length;
      return <button className="overview-card" key={view} onClick={()=>onNavigate(view)} aria-label={`${t(label)} ${q.isLoading?t('正在加载'):q.isError?t('加载失败'):count}`}><span className="muted">{t(label)}</span><strong>{q.isLoading?'…':q.isError?'!':count}</strong>{q.isError&&<small className="error">{t('加载失败')}，{t('点击重试')}</small>}</button>
    })}</div>
    <section className="section"><div className="section-header"><h2>{t('共享上下文')}</h2><button className="ghost" onClick={()=>onNavigate('contexts')}>{t('查看全部')}</button></div>{contexts.isLoading?<p className="muted">{t('正在加载')}…</p>:contexts.isError?<p className="error">{t('加载失败')}</p>:contexts.data?.length?contexts.data.slice(0,3).map(item=><p className="muted" key={item.id}>{String(item.fields?.key||item.name||item.id)}</p>):<p className="empty">{t('暂无记录')}</p>}</section>
    <section className="section"><div className="section-header"><h2>{t('最近变更')}</h2><button className="ghost" onClick={()=>onNavigate('changes')}>{t('查看全部')}</button></div>{changes.isLoading?<p className="muted">{t('正在加载')}…</p>:changes.isError?<p className="error">{t('加载失败')}</p>:changes.data?.length?changes.data.slice(0,5).map(item=><p className="muted" key={item.id}>{String(item.fields?.action||'')} · {String(item.fields?.object_id||'')} · {item.created_at||''}</p>):<p className="empty">{t('暂无记录')}</p>}</section>
  </>
}
