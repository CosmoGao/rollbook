import React, { useEffect, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { useTranslation } from 'react-i18next';
import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { Button } from './components/ui/button';

type API=<T>(path:string,options?:RequestInit)=>Promise<T>;
type Change={fields:{action:string;object_id:string;object_type:string;actor_key_name?:string;actor_key_id?:string;actor?:string;reason?:string};created_at:string};
type Context={fields:{key:string;scope:string;owner:string;project?:string;body_md?:string};created_at:string;source:string};
let root:Root|undefined;
export function unmountAudit(){if(root){root.unmount();root=undefined}}
export function mountAudit(target:Element,view:'changes'|'contexts',api:API,open:(id:string,type:string)=>void){unmountAudit();root=createRoot(target);root.render(<Audit view={view} api={api} open={open}/>)}
function Audit({view,api,open}:{view:'changes'|'contexts';api:API;open:(id:string,type:string)=>void}){
  const {i18n}=useTranslation(),en=i18n.language.startsWith('en');
  const tx=(zh:string,english:string)=>en?english:zh;
  const [key,setKey]=useState(''),[type,setType]=useState(''),[since,setSince]=useState(''),[until,setUntil]=useState(''),[archived,setArchived]=useState(false),[changes,setChanges]=useState<Change[]>([]),[contexts,setContexts]=useState<Context[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState('');
  const load=async()=>{setLoading(true);setError('');try{if(view==='changes'){const query=new URLSearchParams();if(key.trim())query.set('key',key.trim());if(type)query.set('object_type',type);if(since)query.set('since',new Date(since+'T00:00:00').toISOString());if(until)query.set('until',new Date(until+'T23:59:59.999').toISOString());setChanges(await api<Change[]>('/api/changes?'+query))}else setContexts(await api<Context[]>('/api/contexts?archived='+archived))}catch(e){setError(String(e instanceof Error?e.message:e))}finally{setLoading(false)}};
  useEffect(()=>{load()},[view,archived]);
  const safeMarkdown=(value:string)=>{const html=DOMPurify.sanitize(marked.parse(value||'',{async:false}) as string);const doc=new DOMParser().parseFromString(html,'text/html');for(const link of doc.querySelectorAll('a')){try{const url=new URL(link.href);if(!['http:','https:'].includes(url.protocol))link.removeAttribute('href');else{link.target='_blank';link.rel='noopener noreferrer'}}catch{link.removeAttribute('href')}}return doc.body.innerHTML};
  return <><div className="hero"><p className="eyebrow">ROLLBOOK / {view==='changes'?tx('最近变更','Recent changes'):tx('共享上下文','Shared context')}</p><h1>{view==='changes'?tx('最近变更','Recent changes'):tx('共享上下文','Shared context')}</h1><p className="muted">{view==='changes'?tx('按 Key、对象类型和时间查看审计记录。','Filter audit records by key, object type and time.'):tx('查看 agent 与用户写入的上下文。','View context written by agents and users.')}</p></div>
    {view==='changes'?<form className="filter-bar" onSubmit={event=>{event.preventDefault();load()}}><input value={key} onChange={event=>setKey(event.target.value)} placeholder={tx('Key ID 或名称','Key ID or name')}/><select value={type} onChange={event=>setType(event.target.value)}>{[['','全部类型','All types'],['host','主机','Host'],['network','网段','Network'],['service','服务','Service'],['endpoint','访问入口','Endpoint'],['location','站点','Site'],['ip','IP 地址','IP address']].map(([value,zh,english])=><option key={value} value={value}>{tx(zh,english)}</option>)}</select><input type="date" aria-label={tx('开始日期','Start date')} value={since} onChange={event=>setSince(event.target.value)}/><input type="date" aria-label={tx('结束日期','End date')} value={until} onChange={event=>setUntil(event.target.value)}/><Button type="submit">{tx('筛选','Filter')}</Button></form>:<label className="check-row"><input type="checkbox" checked={archived} onChange={event=>setArchived(event.target.checked)}/><span>{tx('显示已归档','Show archived')}</span></label>}
    {loading&&<div className="notice">{tx('正在加载…','Loading…')}</div>}{error&&<div className="error" role="alert">{error}</div>}
    {!loading&&!error&&(view==='changes'?changes.length?changes.map((change,index)=><div className="row" key={change.fields.object_id+index}><div className="row-main"><strong>{change.fields.action} · {change.fields.object_id}</strong><small>{change.fields.actor_key_name||change.fields.actor_key_id||change.fields.actor} · {change.fields.reason} · {change.created_at}</small></div><Button variant="outline" onClick={()=>open(change.fields.object_id,change.fields.object_type)}>{tx('查看','View')}</Button></div>):<div className="empty">{tx('暂无记录','No records yet')}</div>:contexts.length?contexts.map((context,index)=><article className="detail-entry" key={context.fields.key+index}><strong>{context.fields.key} · {context.fields.scope} · {context.fields.owner}</strong><small className="muted">{context.created_at} · {context.source} · {context.fields.project||''}</small><div className="markdown" dangerouslySetInnerHTML={{__html:safeMarkdown(context.fields.body_md||'')}}/></article>):<div className="empty">{tx('暂无记录','No records yet')}</div>)}
  </>
}
