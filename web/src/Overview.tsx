import React, { useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQueries, useQuery } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { proposalTitle } from './proposal-summary';

const client=new QueryClient({defaultOptions:{queries:{retry:1,staleTime:10000}}});
type Item={id:string;type:string;name:string;created_at?:string;fields?:Record<string,any>};
type API=<T>(path:string)=>Promise<T>;
let root:Root|undefined;
export function unmountOverview(){if(root){root.unmount();root=undefined}}
export function mountOverview(target:Element,api:API,onNavigate:(view:string)=>void,onSearch:(term:string)=>void,onCreate:()=>void,onSiteTopology:(siteID:string)=>void){
  unmountOverview();root=createRoot(target);
  root.render(<QueryClientProvider client={client}><Overview api={api} onNavigate={onNavigate} onSearch={onSearch} onCreate={onCreate} onSiteTopology={onSiteTopology}/></QueryClientProvider>);
}
function Overview({api,onNavigate,onSearch,onCreate,onSiteTopology}:{api:API;onNavigate:(view:string)=>void;onSearch:(term:string)=>void;onCreate:()=>void;onSiteTopology:(siteID:string)=>void}){
  const {i18n}=useTranslation(),en=i18n.language.startsWith('en');
  const [term,setTerm]=useState('');
  const paths=['/api/changes?state='+encodeURIComponent('待确认'),'/api/assets/unverified','/api/versions/outdated','/api/browse?type=location','/api/browse?type=host','/api/browse?type=service','/api/browse?type=uplink'];
  const queries=useQueries({queries:paths.map(path=>({queryKey:['overview',path],queryFn:()=>api<Item[]>(path)}))});
  const [pending,unverified,outdated,locations,hosts,services,uplinks]=queries.map(q=>q.data||[]);
  const changes=useQuery({queryKey:['overview','changes'],queryFn:()=>api<Item[]>('/api/changes')});
  const contexts=useQuery({queryKey:['overview','contexts'],queryFn:()=>api<Item[]>('/api/contexts?archived=false')});
  const locationByID=new Map(locations.map(item=>[item.id,item]));
  const siteByLocation=new Map<string,string>();
  for(const item of locations){let cursor:Item|undefined=item;const seen=new Set<string>();while(cursor&&!seen.has(cursor.id)){seen.add(cursor.id);if(cursor.fields?.kind==='站点'){siteByLocation.set(item.id,cursor.id);break}cursor=locationByID.get(cursor.fields?.parent_id)}}
  const hostByID=new Map(hosts.map(item=>[item.id,item]));
  const sites=locations.filter(item=>item.fields?.kind==='站点').map(site=>({
    site,hosts:hosts.filter(host=>siteByLocation.get(host.fields?.location_id)===site.id),
    uplinks:uplinks.filter(uplink=>siteByLocation.get(uplink.fields?.location_id)===site.id),
    services:services.filter(service=>siteByLocation.get(hostByID.get(service.fields?.host_id)?.fields?.location_id)===site.id)
  }));
  const tasks=[{label:en?'Proposals to review':'待确认提议',count:pending.length,view:'pending'},{label:en?'Assets to verify':'待核实对象',count:unverified.length,view:'unverified'},{label:en?'New versions':'有新版本',count:outdated.length,view:'outdated'}];
  return <>
    <div className="hero page-heading"><div><h1>{en?'Overview':'总览'}</h1><p className="muted">{en?'Handle what needs your attention, then see what runs at each site.':'先处理要你拍板的事，再按站点看家里跑了什么。'}</p></div><button className="primary" onClick={onCreate}>{en?'Register asset':'登记资产'}</button></div>
    <div className="overview-layout"><div className="overview-main">
      <section className="section"><div className="section-header"><h2>{en?'Needs your attention':'需要你处理'}</h2><button className="ghost" onClick={()=>onNavigate('pending')}>{en?'View all':'查看全部'}</button></div>
        <div className="overview-task-counts">{tasks.map(({label,count,view},index)=><button key={view} className="overview-task-count" onClick={()=>onNavigate(view)}><span>{label}</span><strong>{queries[index].isLoading?'…':queries[index].isError?'!':count}</strong></button>)}</div>
        {queries.slice(0,3).some(q=>q.isError)&&<p className="error">{en?'Some tasks failed to load':'部分待办加载失败'}</p>}
        <div className="overview-task-list">{pending.slice(0,2).map(item=><button key={item.id} className="overview-task-row" onClick={()=>onNavigate('pending')}><span className="tag warn">{en?'Review':'待确认'}</span><span><strong>{proposalTitle(item,en)}</strong><small>{item.fields?.reason||item.fields?.actor||''}</small></span><span className="overview-arrow">›</span></button>)}{outdated.slice(0,1).map(item=><button key={item.id} className="overview-task-row" onClick={()=>onNavigate('outdated')}><span className="tag">{en?'Update':'有新版本'}</span><span><strong>{item.name}</strong><small>{item.fields?.current_version||'—'} → {item.fields?.latest_version||'—'}</small></span><span className="overview-arrow">›</span></button>)}{!pending.length&&!outdated.length&&!!unverified.length&&<button className="overview-task-row" onClick={()=>onNavigate('unverified')}><span className="tag">{en?'Verify':'待核实'}</span><span><strong>{en?`${unverified.length} assets need verification`:`${unverified.length} 项资产待核实`}</strong></span><span className="overview-arrow">›</span></button>}{queries.slice(0,3).every(q=>q.isSuccess)&&!pending.length&&!unverified.length&&!outdated.length&&<p className="muted">{en?'Nothing to handle right now.':'目前没有待处理事项。'}</p>}</div>
      </section>
      <section className="section"><div className="section-header"><h2>{en?'Sites':'站点'}</h2><button className="ghost" onClick={()=>onNavigate('sites')}>{en?'Browse assets':'查看资产'}</button></div><div className="overview-sites">{sites.map(({site,hosts:siteHosts,services:siteServices,uplinks:siteUplinks})=><article className="overview-site" key={site.id}><button className="overview-site-name" onClick={()=>onNavigate('sites')}>{site.name}</button><p className="muted">{[site.fields?.city,site.fields?.description].filter(Boolean).join(' · ')|| (en?'Site assets':'站点资产')}</p><p>{siteHosts.length} {en?'hosts':'台主机'}　{siteServices.length} {en?'services':'个服务'}</p>{siteUplinks.length>0&&<p className="muted">{en?'Access':'入口'}: {siteUplinks.map(item=>[item.name,item.fields?.public_ipv4].filter(Boolean).join(' · ')).join('；')}</p>}<button className="ghost" onClick={()=>onSiteTopology(site.id)}>{en?'View topology':'看拓扑'}</button></article>)}{queries[3].isSuccess&&sites.length===0&&<p className="muted">{en?'No sites yet':'还没有站点'}</p>}</div></section>
    </div><div className="overview-aside">
      <section className="section"><h2>{en?'Find a device or service':'查找一台设备或一个服务'}</h2><form className="overview-search" onSubmit={event=>{event.preventDefault();onSearch(term)}}><input type="search" value={term} onChange={event=>setTerm(event.target.value)} placeholder={en?'IP, host or service':'例如 IP、主机或服务'} aria-label={en?'Search assets':'搜索资产'}/><button type="submit">{en?'Search':'搜索'}</button></form><p className="muted">{en?'Find who owns it, where it runs, and how to reach it.':'查清这是谁的、部署在哪、怎么访问。'}</p></section>
      <section className="section"><div className="section-header"><h2>{en?'Recent changes':'最近变更'}</h2><button className="ghost" onClick={()=>onNavigate('changes')}>{en?'View all':'查看全部'}</button></div>{changes.isLoading?<p className="muted">…</p>:changes.isError?<p className="error">{en?'Failed to load':'加载失败'}</p>:changes.data?.length?changes.data.slice(0,5).map(item=><button className="overview-change" key={item.id} onClick={()=>onNavigate('changes')}><span>{item.fields?.action==='create'?(en?'Added':'新建'):item.fields?.action==='delete'?(en?'Deleted':'删除'):(en?'Changed':'修改')}</span><strong>{item.fields?.base_object?.name||item.fields?.object_id||item.name}</strong><small>{item.created_at?new Date(item.created_at).toLocaleDateString(en?'en-US':'zh-CN'):''}</small></button>):<p className="muted">{en?'No recent changes':'暂无记录'}</p>}</section>
      <section className="section overview-shortcuts"><h2>{en?'Browse assets':'常用入口'}</h2><div>{[['hosts',en?'Hosts':'主机'],['services',en?'Services':'服务'],['assets',en?'All assets':'全部资产']].map(([view,label])=><button className="ghost" key={view} onClick={()=>onNavigate(view)}>{label}</button>)}</div></section>
      <section className="section"><div className="section-header"><h2>{en?'Shared context':'共享上下文'}</h2><button className="ghost" onClick={()=>onNavigate('contexts')}>{en?'View all':'查看全部'}</button></div>{contexts.isLoading?<p className="muted">…</p>:contexts.isError?<p className="error">{en?'Failed to load':'加载失败'}</p>:contexts.data?.length?contexts.data.slice(0,3).map(item=><button className="overview-change" key={item.id} onClick={()=>onNavigate('contexts')}>{item.fields?.key||item.name||item.id}</button>):<p className="muted">{en?'No records yet':'暂无记录'}</p>}</section>
    </div></div>
  </>;
}
