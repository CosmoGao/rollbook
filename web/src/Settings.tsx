import React, { useEffect, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { useTranslation } from 'react-i18next';
import { Button } from './components/ui/button';
import { api } from './controller';
import { AuthSettings, authDirtySections, resetAuthSettings } from './AuthSettings';
import { Backup, backupHasDraft } from './Backup';

type SessionInfo={login:string;identity_type?:string;auth_provider?:string;can_change_password?:boolean};
type Profile={email:string;feishu_enabled:boolean;feishu_bound:boolean;local_session:boolean};
let root:Root|undefined;
let activeLocation='/settings';
export function settingsLocation(){return activeLocation}
export function canLeaveSettings(){return (!backupHasDraft()&&!authDirtySections().length)||window.confirm(document.documentElement.lang.startsWith('en')?'Discard unsaved changes and leave?':'未保存的更改将丢失。确定离开吗？')}
export function unmountSettings(){if(root){root.unmount();root=undefined;resetAuthSettings()}}
export function mountSettings(target:Element,apiClient:typeof api,onRestored:()=>void){
  unmountSettings();root=createRoot(target);root.render(<Settings apiClient={apiClient} onRestored={onRestored}/>);
}
export function mountPersonalSettings(target:Element){
  unmountSettings();root=createRoot(target);root.render(<PersonalSettings/>);
}
const groups=['local','ldap','oidc','feishu','accounts','audit','backup'] as const;
type Group=typeof groups[number];
const groupNames:Record<Group,string>={local:'本地登录',ldap:'LDAP / LDAPS',oidc:'通用 OIDC',feishu:'飞书',accounts:'账号与绑定',audit:'认证审计',backup:'备份与恢复'};
function readGroup():Group{const value=new URLSearchParams(location.search).get('section');return groups.includes(value as Group)?value as Group:'local'}
function groupPath(group:Group){return group==='local'?'/settings':`/settings?section=${group}`}
function Settings({apiClient,onRestored}:{apiClient:typeof api;onRestored:()=>void}){
  const {t}=useTranslation();
  const [group,setGroup]=useState<Group>(readGroup);
  const [dirty,setDirty]=useState<string[]>([]);
  const select=(next:Group,push=true)=>{if(next===group)return;const path=groupPath(next);if(push)history.pushState({},'',path);activeLocation=path;setGroup(next);setTimeout(()=>{document.querySelector<HTMLButtonElement>('.settings-nav [aria-current="page"]')?.scrollIntoView({block:'nearest',inline:'nearest'});document.getElementById('settings-panel-heading')?.focus()},0)};
  useEffect(()=>{if(location.pathname==='/settings'&&location.pathname+location.search!==groupPath(readGroup()))history.replaceState({},'',groupPath(readGroup()));activeLocation=location.pathname+location.search;const onLocation=()=>{const next=readGroup();if(location.pathname+location.search!==groupPath(next))history.replaceState({},'',groupPath(next));setGroup(next);activeLocation=location.pathname+location.search;setTimeout(()=>document.getElementById('settings-panel-heading')?.focus(),0)};window.addEventListener('rollbook:settings-location',onLocation);const beforeUnload=(event:BeforeUnloadEvent)=>{if(!backupHasDraft())return;event.preventDefault();event.returnValue=''};window.addEventListener('beforeunload',beforeUnload);const onDrafts=()=>setDirty([...authDirtySections(),...(backupHasDraft()?['backup']:[])]);window.addEventListener('rollbook:settings-drafts',onDrafts);return()=>{window.removeEventListener('rollbook:settings-location',onLocation);window.removeEventListener('rollbook:settings-drafts',onDrafts);window.removeEventListener('beforeunload',beforeUnload)}},[]);
  return <div className="settings-page"><div className="hero"><h1>{t('settings')}</h1><p className="muted">{t('系统配置与数据管理')}</p></div><div className="settings-layout"><nav className="settings-nav" aria-label={t('设置分组')}><div className="settings-nav-group"><span>{t('认证与登录')}</span>{groups.slice(0,6).map(item=><button key={item} type="button" aria-current={group===item?'page':undefined} onClick={()=>select(item)}>{t(groupNames[item])}{dirty.includes(item)&&<span className="settings-unsaved">● {t('未保存')}</span>}</button>)}</div><div className="settings-nav-group"><span>{t('数据与备份')}</span><button type="button" aria-current={group==='backup'?'page':undefined} onClick={()=>select('backup')}>{t('备份与恢复')}{dirty.includes('backup')&&<span className="settings-unsaved">● {t('未保存')}</span>}</button></div></nav><div className={'settings-panel '+(group==='accounts'||group==='audit'?'settings-panel-wide':'')}><h2 id="settings-panel-heading" tabIndex={-1}>{t(groupNames[group])}</h2>{dirty.some(item=>item!==group)&&<p className="settings-draft-hint" role="status">{t('其他分组未保存')}：{dirty.filter(item=>item!==group).map(item=>t(groupNames[item as Group])).join('、')}</p>}<div hidden={group==='backup'}><AuthSettings section={group==='backup'?'local':group} /></div><div hidden={group!=='backup'}><Backup api={apiClient} onRestored={onRestored}/></div></div></div></div>;
}
function PersonalSettings(){
  const {t}=useTranslation();
  const [session,setSession]=useState<SessionInfo|null>(null),[profile,setProfile]=useState<Profile|null>(null),[email,setEmail]=useState(''),[notice,setNotice]=useState(''),[error,setError]=useState('');
  useEffect(()=>{let active=true;api<SessionInfo>('/api/auth/session').then(value=>{if(active)setSession(value)}).catch(cause=>{if(active)setError(String(cause instanceof Error?cause.message:cause))});api<Profile>('/api/auth/profile').then(value=>{if(active){setProfile(value);setEmail(value.email)}}).catch(cause=>{if(active)setError(String(cause instanceof Error?cause.message:cause))});return()=>{active=false}},[]);
  const canChange=session?.can_change_password!==false;
  return <div className="personal-settings">
    <div className="hero"><h1>{t('个人设置')}</h1><p className="muted">{t('查看账号与管理清册会话')}</p></div>
    {error&&<p className="error" role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}{new URLSearchParams(location.search).get('bind')==='ok'&&<p role="status">{t('飞书绑定成功')}</p>}{new URLSearchParams(location.search).has('bind_error')&&<p className="error" role="alert">{t(new URLSearchParams(location.search).get('bind_error')==='session'?'绑定会话已失效，请重新登录后重试':new URLSearchParams(location.search).get('bind_error')==='disabled'?'此飞书绑定已禁用':new URLSearchParams(location.search).get('bind_error')==='failed'?'飞书授权失败，请重试':'飞书身份已被其他账号绑定或当前账号已有飞书身份')}</p>}
    {!session&&!error&&<p role="status">{t('正在加载')}</p>}
    {session&&<>
      <section className="section"><h2>{t('账号信息')}</h2><dl className="account-details"><div><dt>{t('账号')}</dt><dd>{session.login}</dd></div>{session.identity_type&&<div><dt>{t('登录身份')}</dt><dd><span className="status-badge">{t(({local:'本地账号',external:'外部账号',mixed:'混合账号'} as Record<string,string>)[session.identity_type]||session.identity_type)}</span></dd></div>}{session.auth_provider&&<div><dt>{t('认证提供方')}</dt><dd>{session.auth_provider}</dd></div>}</dl></section>
      <section className="section"><h2>{t('邮箱与飞书')}</h2><form onSubmit={async event=>{event.preventDefault();setError('');try{await api('/api/auth/profile',{method:'PUT',body:JSON.stringify({email})});setNotice(t('邮箱已保存'))}catch(cause){setError(String(cause instanceof Error?cause.message:cause))}}}><label>{t('邮箱')}<input type="email" value={email} onChange={event=>setEmail(event.target.value)} /></label><Button type="submit">{t('保存邮箱')}</Button></form>{profile?.feishu_enabled&&<div className="settings-subsection"><h3>{t('飞书绑定')}</h3><span className="status-badge">{t(profile.feishu_bound?'已绑定飞书':'尚未绑定飞书')}</span></div>}{profile?.feishu_enabled&&!profile.feishu_bound&&profile.local_session&&<Button asChild variant="outline"><a href="/api/auth/feishu/bind/start">{t('绑定飞书')}</a></Button>}</section>
      <section className="section"><h2>{t('安全与会话')}</h2><h3>{t('密码')}</h3>{canChange?<Button variant="outline" onClick={()=>window.dispatchEvent(new Event('rollbook:open-password'))}>{t('改密码')}</Button>:<p className="muted">{t('此账号的密码由上游身份提供方管理，请在上游修改。')}</p>}<div className="settings-danger-zone"><h3>{t('清册会话')}</h3><p className="muted">{t('退出所有设备会撤销所有清册浏览器会话，包括当前会话；不会退出上游 SSO，也不会撤销 API Key。')}</p><Button variant="outline" className="outline-danger" onClick={()=>window.dispatchEvent(new Event('rollbook:logout-all'))}>{t('退出所有设备')}</Button></div></section>
    </>}
  </div>;
}
