import React, {useEffect, useRef, useState} from 'react';
import {createRoot, Root} from 'react-dom/client';
import {Button} from './components/ui/button';
import {api} from './controller';
import {useTranslation} from 'react-i18next';

type Kind='ldap'|'oidc'|'feishu';
type Section='local'|Kind|'accounts'|'audit';
type Provider={kind:Kind;enabled:boolean;source:'environment'|'page';shadowed:boolean;config:Record<string,string>;secret_configured:boolean;error?:string};
type Settings={providers:Provider[];local_enabled:boolean};
type User={login:string;enabled:boolean;has_local_password:boolean};
type Binding={provider:Kind;instance:string;subject:string;login:string;allowed:boolean;verified_at:string};
type IdentityList={users:User[];bindings:Binding[]};
type Audit={page:number;entries:{id:number;occurred_at:string;action:string;target:string}[]};
type Draft={config:Record<string,string>;enabled:boolean;secret:string;action:'keep'|'replace'|'clear'};
const sections:Section[]=['local','ldap','oidc','feishu','accounts','audit'];
const names:Record<Section,string>={local:'本地登录',ldap:'LDAP / LDAPS',oidc:'通用 OIDC',feishu:'飞书',accounts:'账号与绑定',audit:'认证审计'};
const fields:Record<Kind,[string,string][]>={ldap:[['url','服务器 URL'],['instance','实例名'],['base_dn','Base DN'],['bind_dn','Bind DN'],['filter','搜索筛选器（须包含 {login}）'],['subject_attribute','稳定主体属性']],oidc:[['issuer','Issuer'],['client_id','Client ID'],['redirect_url','回调 URL']],feishu:[['client_id','App ID'],['redirect_url','回调 URL']]};
const readSection=():Section=>{const value=new URLSearchParams(location.search).get('section');return sections.includes(value as Section)?value as Section:'local'};
const path=(section:Section)=>section==='local'?'/settings/auth':`/settings/auth?section=${section}`;
const same=(a:Draft,b:Draft)=>a.enabled===b.enabled&&a.action===b.action&&a.secret===b.secret&&Object.keys({...a.config,...b.config}).every(key=>(a.config[key]||'')===(b.config[key]||''));
const initialDraft=(p:Provider):Draft=>({config:{...p.config,...(p.kind!=='ldap'&&!p.config.redirect_url?{redirect_url:`https://${location.host}/api/auth/${p.kind}/callback`}:{})},enabled:p.enabled,secret:'',action:'keep'});
let root:Root|undefined;
let guard:undefined|(()=>boolean);
let activeLocation='/settings/auth';
export function canLeaveAuthSettings(){return !guard||guard()}
export function authSettingsLocation(){return activeLocation}
export function unmountAuthSettings(){guard=undefined;root?.unmount();root=undefined}
export function mountAuthSettings(target:Element){unmountAuthSettings();root=createRoot(target);root.render(<AuthSettings/>)}
function AuthSettings(){
 const {t,i18n}=useTranslation();
 const [section,setSection]=useState<Section>(readSection),[settings,setSettings]=useState<Settings|null>(null),[identities,setIdentities]=useState<IdentityList|null>(null),[audit,setAudit]=useState<Audit|null>(null),[page,setPage]=useState(1);
 const [drafts,setDrafts]=useState<Partial<Record<Kind,Draft>>>({}),[base,setBase]=useState<Partial<Record<Kind,Draft>>>({});
 const [login,setLogin]=useState(''),[provider,setProvider]=useState<Kind>('ldap'),[instance,setInstance]=useState(''),[subject,setSubject]=useState(''),[target,setTarget]=useState(''),[allowed,setAllowed]=useState(true);
 const [busy,setBusy]=useState(''),[error,setError]=useState(''),[notice,setNotice]=useState(''),[loading,setLoading]=useState(false);
 const sectionRef=useRef(section),dirtyRef=useRef(false),lastUrl=useRef(location.pathname+location.search);
 activeLocation=lastUrl.current;
 sectionRef.current=section;
 const accountDirty=!!(login||instance||subject||target||provider!=='ldap'||!allowed);
 const dirty=Object.keys(drafts).some(k=>{const key=k as Kind;return drafts[key]&&base[key]&&!same(drafts[key]!,base[key]! )})||accountDirty;
 dirtyRef.current=dirty;
 const confirmLeave=()=>!dirtyRef.current||window.confirm(t('未保存的更改将丢失。确定离开吗？'));
 const confirmSwitch=()=>!dirtyRef.current||window.confirm(t('有未保存的更改。切换分组后草稿仍保留，确定切换吗？'));
 guard=confirmLeave;
 useEffect(()=>{const requested=new URLSearchParams(location.search).get('section');if(requested&&!sections.includes(requested as Section)){history.replaceState({},'',path('local'));lastUrl.current=location.pathname+location.search;activeLocation=lastUrl.current}},[]);
 useEffect(()=>{const before=(event:BeforeUnloadEvent)=>{if(!dirtyRef.current)return;event.preventDefault();event.returnValue=''};window.addEventListener('beforeunload',before);return()=>window.removeEventListener('beforeunload',before)},[]);
 useEffect(()=>{const pop=()=>{if(location.pathname!=='/settings/auth')return;const next=readSection();if(next===sectionRef.current)return;if(!confirmSwitch()){history.pushState({},'',lastUrl.current);return}setSection(next);lastUrl.current=location.pathname+location.search;activeLocation=lastUrl.current;setError('');setNotice('');setTimeout(()=>document.getElementById('auth-heading')?.focus(),0)};window.addEventListener('rollbook:auth-location',pop);return()=>window.removeEventListener('rollbook:auth-location',pop)},[t]);
 const go=(next:Section)=>{if(next===section)return;if(!confirmSwitch())return;history.pushState({},'',path(next));lastUrl.current=location.pathname+location.search;activeLocation=lastUrl.current;setSection(next);setError('');setNotice('');setTimeout(()=>document.getElementById('auth-heading')?.focus(),0)};
 const refresh=async()=>{setLoading(true);setError('');try{
   if(section==='local'||['ldap','oidc','feishu'].includes(section)){
    const value=await api<Settings>('/api/auth/settings');setSettings(value);
    setDrafts(old=>{const next={...old};for(const p of value.providers)if(!next[p.kind])next[p.kind]=initialDraft(p);return next});
    setBase(old=>{const next={...old};for(const p of value.providers)if(!next[p.kind])next[p.kind]=initialDraft(p);return next});
   }else if(section==='accounts')setIdentities(await api<IdentityList>('/api/auth/settings/identities'));
   else setAudit(await api<Audit>(`/api/auth/settings/audit?page=${page}`));
  }catch(cause){setError(String(cause instanceof Error?cause.message:cause))}finally{setLoading(false)}};
 useEffect(()=>{void refresh()},[section,page]);
 const run=async(key:string,work:()=>Promise<unknown>,reload:()=>Promise<void>,success='已保存')=>{setBusy(key);setError('');setNotice('');try{await work();await reload();setNotice(t(success))}catch(cause){setError(String(cause instanceof Error?cause.message:cause))}finally{setBusy('')}};
 const saveProvider=(kind:Kind)=>{const draft=drafts[kind];if(!draft)return;void run(kind,()=>api(`/api/auth/settings/${kind}`,{method:'PUT',body:JSON.stringify({enabled:draft.enabled,config:draft.config,secret_action:draft.action,secret:draft.action==='replace'?draft.secret:''})}),async()=>{const latest=await api<Settings>('/api/auth/settings');setSettings(latest);const clean={...draft,secret:'',action:'keep' as const};setDrafts(old=>({...old,[kind]:clean}));setBase(old=>({...old,[kind]:clean}))})};
 const kind=section==='ldap'||section==='oidc'||section==='feishu'?section:null;
 const current=settings?.providers.find(p=>p.kind===kind);
 const draft=kind?drafts[kind]:undefined;
 const update=(change:Partial<Draft>)=>{if(kind&&draft)setDrafts(old=>({...old,[kind]:{...draft,...change}}))};
 const action=(key:string,work:()=>Promise<unknown>,refreshWhat:'settings'|'identities',success='已保存')=>void run(key,work,async()=>{if(refreshWhat==='settings')setSettings(await api<Settings>('/api/auth/settings'));else setIdentities(await api<IdentityList>('/api/auth/settings/identities'))},success);
 const colon=i18n.language==='en'?': ':'：',separator=i18n.language==='en'?'; ':'；';
 return <div className="auth-settings"><div className="hero"><h1>{t('认证设置')}</h1><p className="muted">{t('所有已登录网页用户均有完整管理权。更改身份源会影响登录与已有清册会话。')}</p></div>
 <nav className="auth-sections" aria-label={t('认证设置分组')}>{sections.map(item=><button type="button" key={item} className={section===item?'active':''} aria-current={section===item?'page':undefined} onClick={()=>go(item)}>{t(names[item])}</button>)}</nav>
 <section className="section auth-panel" aria-labelledby="auth-heading"><h2 id="auth-heading" tabIndex={-1}>{t(names[section])}</h2>
 {loading&&<p role="status">{t('正在加载…')}</p>}{error&&<div role="alert" className="error"><p>{error}</p><Button variant="outline" type="button" onClick={()=>void refresh()}>{t('重试读取')}</Button></div>}{notice&&<p role="status">{notice}</p>}
 {section==='local'&&settings&&<><p className="muted">{t('关闭前须有当前启用身份源下、允许且用户启用、近一小时真实试登录的绑定；连通测试不算试登录。关闭会撤销本地浏览器会话。若失去入口，请在服务器运行 rollbook auth local-login enable。')}</p><p>{t('当前')}{colon}{t(settings.local_enabled?'已启用':'已关闭')}</p><Button type="button" variant="outline" disabled={!!busy} onClick={()=>{if(window.confirm(t(settings.local_enabled?'确定关闭本地登录？关闭后将撤销本地会话。':'确定启用本地登录？')))action('local',()=>api('/api/auth/settings/local',{method:'PUT',body:JSON.stringify({enabled:!settings.local_enabled})}),'settings')}}>{t(settings.local_enabled?'关闭本地登录':'启用本地登录')}</Button></>}
 {kind&&current&&draft&&<><p className="muted">{t('任一同源环境变量存在时，该源由启动环境接管，页面只读。已保存页面配置会被遮蔽；移除环境变量后须重启才会切换来源。凭据仅能覆盖或明确清除，页面与 YAML 备份均不回显或恢复凭据。')}</p><form className="auth-provider" onSubmit={event=>{event.preventDefault();saveProvider(kind)}}><p>{t('来源')}{colon}{t(current.source==='environment'?'启动环境（只读）':'页面配置')}{separator}{t('当前')}{colon}{t(current.enabled?'已启用':'已关闭')}{separator}{t('凭据')}{colon}{t(current.secret_configured?'已配置':'未配置')}{current.shadowed?`${separator}${t('已保存页面配置被环境遮蔽')}`:''}</p>{current.error&&<p className="error" role="alert">{current.error}</p>}
 {kind==='ldap'&&<p className="muted">{t('仅允许 LDAPS 或带有效证书校验的 StartTLS；不可跳过证书验证。')}</p>}{kind!=='ldap'&&<p className="muted">{t('请在身份提供方登记固定公开 HTTPS 回调：')}<code>{`${location.origin}/api/auth/${kind}/callback`.replace(/^http:/,'https:')}</code></p>}
 <div className="auth-fields">{fields[kind].map(([key,label])=><label key={key}>{t(label)}<input value={draft.config[key]||''} readOnly={current.source==='environment'||key==='redirect_url'} onChange={event=>update({config:{...draft.config,[key]:event.target.value}})}/></label>)}</div>
 {current.source==='page'&&<><label className="auth-check"><input type="checkbox" checked={draft.enabled} onChange={event=>update({enabled:event.target.checked})}/>{t('启用此身份源')}</label><fieldset><legend>{t('敏感凭据')}{colon}{t(current.secret_configured?'已配置':'未配置')}</legend>{(['keep','replace','clear'] as const).map(a=><label key={a}><input type="radio" name="secret-action" checked={draft.action===a} onChange={()=>update({action:a,secret:a==='replace'?draft.secret:''})}/>{t(({keep:'保持现有凭据',replace:'覆盖凭据',clear:'清除凭据'} as const)[a])}</label>)}{draft.action==='replace'&&<label>{t('新凭据')}<input type="password" value={draft.secret} onChange={event=>update({secret:event.target.value})} autoComplete="new-password" required/></label>}</fieldset><Button type="submit" disabled={!!busy} aria-busy={busy===kind}>{busy===kind?t('保存中…'):t('保存身份源')}</Button></>}
 {current.secret_configured&&<Button type="button" variant="outline" disabled={!!busy} onClick={()=>action('test',()=>api(`/api/auth/settings/${kind}/test`,{method:'POST'}),'settings','连通测试已完成')}>{t('测试连通')}</Button>}</form></>}
 {section==='accounts'&&identities&&<><p className="muted">{t('仅显式绑定稳定主体；不会按邮箱或同名自动关联。更改绑定会撤销相关会话并重置试登录时间。')}</p><div className="auth-table-scroll"><table><thead><tr>{['账号','状态','本地口令','操作'].map(s=><th key={s} scope="col">{t(s)}</th>)}</tr></thead><tbody>{identities.users.map(user=><tr key={user.login}><td>{user.login}</td><td>{(i18n.language==='en'?(user.enabled?'Enabled':'Disabled'):(user.enabled?'启用':'停用'))}</td><td>{t(user.has_local_password?'有':'无')}</td><td><Button variant="outline" type="button" disabled={!!busy} onClick={()=>{if(window.confirm(t('确定更改账号状态？相关会话将撤销。')))action('user',()=>api('/api/auth/settings/users',{method:'PUT',body:JSON.stringify({login:user.login,enabled:!user.enabled})}),'identities')}}>{t(user.enabled?'停用':'启用')}</Button></td></tr>)}</tbody></table></div>
 <form className="auth-inline" onSubmit={event=>{event.preventDefault();void run('user',()=>api('/api/auth/settings/users',{method:'POST',body:JSON.stringify({login})}),async()=>{setIdentities(await api<IdentityList>('/api/auth/settings/identities'));setLogin('')})}}><label>{t('新增外部账号')}<input value={login} onChange={event=>setLogin(event.target.value)} required/></label><Button type="submit" disabled={!!busy}>{t('新建用户')}</Button></form>
 <h3>{t('身份绑定')}</h3><div className="auth-table-scroll"><table><thead><tr>{['来源','实例','主体','账号','允许','最近试登录'].map(s=><th key={s} scope="col">{t(s)}</th>)}</tr></thead><tbody>{identities.bindings.map((b,i)=><tr key={i}><td>{b.provider}</td><td>{b.instance}</td><td>{b.subject}</td><td>{b.login}</td><td>{t(b.allowed?'是':'否')}</td><td>{b.verified_at||t('未验证')}</td></tr>)}</tbody></table></div>{!identities.bindings.length&&<p>{t('暂无绑定。')}</p>}
 <form className="auth-fields" onSubmit={event=>{event.preventDefault();void run('binding',()=>api('/api/auth/settings/bindings',{method:'PUT',body:JSON.stringify({provider,instance,subject,login:target,allowed})}),async()=>{setIdentities(await api<IdentityList>('/api/auth/settings/identities'));setInstance('');setSubject('');setTarget('');setProvider('ldap');setAllowed(true)})}}><label>{t('身份源')}<select value={provider} onChange={event=>setProvider(event.target.value as Kind)}><option value="ldap">LDAP</option><option value="oidc">OIDC</option><option value="feishu">{t('飞书')}</option></select></label><label>{t('实例名 / 完整 Issuer / App ID')}<input value={instance} onChange={event=>setInstance(event.target.value)} required/></label><label>{t('稳定主体')}<input value={subject} onChange={event=>setSubject(event.target.value)} required/></label><label>{t('清册账号')}<input value={target} onChange={event=>setTarget(event.target.value)} required/></label><label className="auth-check"><input type="checkbox" checked={allowed} onChange={event=>setAllowed(event.target.checked)}/>{t('允许登录')}</label><Button type="submit" disabled={!!busy}>{t('保存绑定')}</Button></form></>}
 {section==='audit'&&<><p className="muted">{t('当前审计仅记录时间、操作和目标，不记录操作者或 IP。')}</p>{audit?.entries.length?<div className="auth-table-scroll"><table><thead><tr>{['时间','操作','目标'].map(s=><th key={s} scope="col">{t(s)}</th>)}</tr></thead><tbody>{audit.entries.map(e=><tr key={e.id}><td>{e.occurred_at}</td><td>{e.action}</td><td>{e.target}</td></tr>)}</tbody></table></div>:!loading&&!error&&<p>{t('暂无认证审计记录。')}</p>}<div className="controls"><Button variant="outline" type="button" disabled={page===1||loading} onClick={()=>setPage(page-1)}>{t('上一页')}</Button><span>{t('第')} {page}{i18n.language==='en'?'':` ${t('页')}`}</span><Button variant="outline" type="button" disabled={!audit||audit.entries.length<20||loading} onClick={()=>setPage(page+1)}>{t('下一页')}</Button></div></>}
 </section></div>
}
