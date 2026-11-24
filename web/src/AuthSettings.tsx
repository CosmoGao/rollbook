import React, {useEffect,useState} from 'react';
import {createRoot,Root} from 'react-dom/client';
import {Button} from './components/ui/button';
import {api} from './controller';

type Kind='ldap'|'oidc'|'feishu';
type Provider={kind:Kind;enabled:boolean;source:'environment'|'page';shadowed:boolean;config:Record<string,string>;secret_configured:boolean;error?:string};
type Settings={providers:Provider[];local_enabled:boolean};
type User={login:string;enabled:boolean};
type Binding={provider:Kind;instance:string;subject:string;login:string;allowed:boolean;verified_at:string};
type IdentityList={users:User[];bindings:Binding[]};
type Audit={page:number;entries:{id:number;occurred_at:string;action:string;target:string}[]};
const names:Record<Kind,string>={ldap:'LDAP / LDAPS',oidc:'通用 OIDC',feishu:'飞书'};
const fields:Record<Kind,[string,string][]>={ldap:[['url','服务器 URL'],['instance','实例名'],['base_dn','Base DN'],['bind_dn','Bind DN'],['filter','搜索筛选器（须包含 {login}）'],['subject_attribute','稳定主体属性']],oidc:[['issuer','Issuer'],['client_id','Client ID'],['redirect_url','回调 URL']],feishu:[['client_id','App ID'],['redirect_url','回调 URL']]};
let root:Root|undefined;
export function unmountAuthSettings(){root?.unmount();root=undefined}
export function mountAuthSettings(target:Element){unmountAuthSettings();root=createRoot(target);root.render(<AuthSettings/>)}
function AuthSettings(){
 const [settings,setSettings]=useState<Settings|null>(null),[identities,setIdentities]=useState<IdentityList|null>(null),[audit,setAudit]=useState<Audit|null>(null),[page,setPage]=useState(1),[busy,setBusy]=useState(false),[error,setError]=useState(''),[notice,setNotice]=useState('');
 const refresh=async()=>{setError('');try{const [s,i,a]=await Promise.all([api<Settings>('/api/auth/settings'),api<IdentityList>('/api/auth/settings/identities'),api<Audit>(`/api/auth/settings/audit?page=${page}`)]);setSettings(s);setIdentities(i);setAudit(a)}catch(cause){setError(String(cause instanceof Error?cause.message:cause))}};
 useEffect(()=>{void refresh()},[page]);
 const run=async(work:()=>Promise<unknown>)=>{setBusy(true);setError('');setNotice('');try{await work();setNotice('已保存');await refresh()}catch(cause){setError(String(cause instanceof Error?cause.message:cause))}finally{setBusy(false)}};
 return <div className="auth-settings"><div className="hero"><h1>认证设置</h1><p className="muted">所有已登录网页用户均有完整管理权。更改身份源会影响登录与已有清册会话。</p></div>
 {error&&<div role="alert" className="error"><p>{error}</p><Button variant="outline" type="button" onClick={()=>void refresh()}>重试读取</Button></div>}{notice&&<p role="status">{notice}</p>}
 {!settings&&!error&&<p role="status">正在加载认证设置…</p>}
 {settings&&<>
  <section className="section"><h2>身份源</h2><p className="muted">任一同源环境变量存在时，该源由启动环境接管，页面只读。已保存页面配置会被遮蔽；移除环境变量后须重启才会切换来源。凭据仅能覆盖或明确清除，页面与 YAML 备份均不回显或恢复凭据。</p>
   {settings.providers.map(provider=><ProviderForm key={provider.kind} value={provider} busy={busy} run={run}/>)}
  </section>
  <section className="section"><h2>本地登录</h2><p className="muted">关闭前须有当前启用身份源下、允许且用户启用、近一小时真实试登录的绑定；连通测试不算试登录。关闭会撤销本地浏览器会话。若失去入口，请在服务器运行 rollbook auth local-login enable。</p><p>当前：{settings.local_enabled?'已启用':'已关闭'}</p><Button type="button" variant="outline" disabled={busy} onClick={()=>{if(window.confirm(`确定${settings.local_enabled?'关闭':'启用'}本地登录？关闭后将撤销本地会话。`))void run(()=>api('/api/auth/settings/local',{method:'PUT',body:JSON.stringify({enabled:!settings.local_enabled})}))}}>{settings.local_enabled?'关闭本地登录':'启用本地登录'}</Button></section>
  <section className="section"><h2>用户与身份绑定</h2><p className="muted">仅显式绑定稳定主体；不会按邮箱或同名自动关联。更改绑定会撤销相关会话并重置试登录时间。</p><IdentityManager values={identities} busy={busy} run={run}/></section>
  <section className="section"><h2>认证审计</h2><p className="muted">当前审计仅记录时间、操作和目标，不记录操作者或 IP。</p>{audit?.entries.length?<div className="auth-table-scroll"><table><thead><tr><th scope="col">时间</th><th scope="col">操作</th><th scope="col">目标</th></tr></thead><tbody>{audit.entries.map(entry=><tr key={entry.id}><td>{entry.occurred_at}</td><td>{entry.action}</td><td>{entry.target}</td></tr>)}</tbody></table></div>:<p>暂无认证审计记录。</p>}<div className="controls"><Button variant="outline" type="button" disabled={page===1} onClick={()=>setPage(page-1)}>上一页</Button><span>第 {page} 页</span><Button variant="outline" type="button" disabled={!audit||audit.entries.length<20} onClick={()=>setPage(page+1)}>下一页</Button></div></section>
 </>}</div>
}
function ProviderForm({value,busy,run}:{value:Provider;busy:boolean;run:(work:()=>Promise<unknown>)=>Promise<void>}){
 const [config,setConfig]=useState(value.config),[enabled,setEnabled]=useState(value.enabled),[secret,setSecret]=useState(''),[action,setAction]=useState<'keep'|'replace'|'clear'>('keep');
 useEffect(()=>{setConfig({...value.config,...(value.kind!=='ldap'&&!value.config.redirect_url?{redirect_url:`https://${location.host}/api/auth/${value.kind}/callback`}:{})});setEnabled(value.enabled);setSecret('');setAction('keep')},[value]);
 const callback=value.kind==='ldap'?'':`${location.origin}/api/auth/${value.kind}/callback`;
 const submit=(event:React.FormEvent)=>{event.preventDefault();void run(()=>api(`/api/auth/settings/${value.kind}`,{method:'PUT',body:JSON.stringify({enabled,config,secret_action:action,secret:action==='replace'?secret:''})}))};
 return <form className="auth-provider" onSubmit={submit}><h3>{names[value.kind]}</h3><p>来源：{value.source==='environment'?'启动环境（只读）':'页面配置'}；当前：{value.enabled?'启用':'关闭'}；凭据：{value.secret_configured?'已配置':'未配置'}{value.shadowed?'；已保存页面配置被环境遮蔽':''}</p>{value.error&&<p className="error" role="alert">{value.error}</p>}
  {value.kind==='ldap'&&<p className="muted">仅允许 LDAPS 或带有效证书校验的 StartTLS；不可跳过证书验证。</p>}
  {callback&&<p className="muted">请在身份提供方登记固定公开 HTTPS 回调：<code>{callback.replace(/^http:/,'https:')}</code></p>}
  <div className="auth-fields">{fields[value.kind].map(([key,label])=><label key={key}>{label}<input type="text" value={config[key]||''} readOnly={value.source==='environment'||key==='redirect_url'} onChange={event=>setConfig({...config,[key]:event.target.value})} /></label>)}</div>
  {value.source==='page'&&<><label className="auth-check"><input type="checkbox" checked={enabled} onChange={event=>setEnabled(event.target.checked)}/>启用此身份源</label><fieldset><legend>敏感凭据：{value.secret_configured?'已配置':'未配置'}</legend><label><input type="radio" name={`${value.kind}-secret-action`} checked={action==='keep'} onChange={()=>setAction('keep')}/>保持现有凭据</label><label><input type="radio" name={`${value.kind}-secret-action`} checked={action==='replace'} onChange={()=>setAction('replace')}/>覆盖凭据</label><label><input type="radio" name={`${value.kind}-secret-action`} checked={action==='clear'} onChange={()=>setAction('clear')}/>清除凭据</label>{action==='replace'&&<label>新凭据<input type="password" value={secret} onChange={event=>setSecret(event.target.value)} autoComplete="new-password" required/></label>}</fieldset><div className="controls"><Button type="submit" disabled={busy} aria-busy={busy}>{busy?'保存中…':`保存${names[value.kind]}`}</Button></div></>}
  {value.secret_configured&&<Button type="button" variant="outline" disabled={busy} onClick={()=>void run(()=>api(`/api/auth/settings/${value.kind}/test`,{method:'POST'}))}>测试连通</Button>}
 </form>
}
function IdentityManager({values,busy,run}:{values:IdentityList|null;busy:boolean;run:(work:()=>Promise<unknown>)=>Promise<void>}){
 const [login,setLogin]=useState(''),[provider,setProvider]=useState<Kind>('ldap'),[instance,setInstance]=useState(''),[subject,setSubject]=useState(''),[target,setTarget]=useState(''),[allowed,setAllowed]=useState(true);
 return <>{!values&&<p role="status">正在加载…</p>}{values&&<>
 <div className="auth-table-scroll"><table><thead><tr><th>账号</th><th>状态</th><th>操作</th></tr></thead><tbody>{values.users.map(user=><tr key={user.login}><td>{user.login}</td><td>{user.enabled?'启用':'停用'}</td><td><Button variant="outline" type="button" disabled={busy} onClick={()=>{if(window.confirm(`确定${user.enabled?'停用':'启用'}账号 ${user.login}？相关会话将撤销。`))void run(()=>api('/api/auth/settings/users',{method:'PUT',body:JSON.stringify({login:user.login,enabled:!user.enabled})}))}}>{user.enabled?'停用':'启用'}</Button></td></tr>)}</tbody></table></div>
 <form className="auth-inline" onSubmit={event=>{event.preventDefault();void run(()=>api('/api/auth/settings/users',{method:'POST',body:JSON.stringify({login})}));setLogin('')}}><label>新增外部账号<input value={login} onChange={event=>setLogin(event.target.value)} required/></label><Button type="submit" disabled={busy}>新建用户</Button></form>
 <h3>身份绑定</h3><div className="auth-table-scroll"><table><thead><tr><th>来源</th><th>实例</th><th>主体</th><th>账号</th><th>允许</th><th>最近试登录</th></tr></thead><tbody>{values.bindings.map((binding,index)=><tr key={index}><td>{binding.provider}</td><td>{binding.instance}</td><td>{binding.subject}</td><td>{binding.login}</td><td>{binding.allowed?'是':'否'}</td><td>{binding.verified_at||'未验证'}</td></tr>)}</tbody></table></div>{values.bindings.length===0&&<p>暂无绑定。</p>}
 <form className="auth-fields" onSubmit={event=>{event.preventDefault();void run(()=>api('/api/auth/settings/bindings',{method:'PUT',body:JSON.stringify({provider,instance,subject,login:target,allowed})}))}}><label>身份源<select value={provider} onChange={event=>setProvider(event.target.value as Kind)}><option value="ldap">LDAP</option><option value="oidc">OIDC</option><option value="feishu">飞书</option></select></label><label>实例名 / 完整 Issuer / App ID<input value={instance} onChange={event=>setInstance(event.target.value)} required/></label><label>稳定主体<input value={subject} onChange={event=>setSubject(event.target.value)} required/></label><label>清册账号<input value={target} onChange={event=>setTarget(event.target.value)} required/></label><label className="auth-check"><input type="checkbox" checked={allowed} onChange={event=>setAllowed(event.target.checked)}/>允许登录</label><Button type="submit" disabled={busy}>保存绑定</Button></form>
 </>}</>
}
