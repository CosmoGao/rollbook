import React, { useEffect, useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { useTranslation } from 'react-i18next';
import { Button } from './components/ui/button';
import { api } from './controller';

type SessionInfo={login:string;identity_type?:string;auth_provider?:string;can_change_password?:boolean};
type Profile={email:string;feishu_enabled:boolean;feishu_bound:boolean;local_session:boolean};
let root:Root|undefined;
export function unmountSettings(){if(root){root.unmount();root=undefined}}
export function mountSettings(target:Element,onBackup:()=>void,onAuth:()=>void){
  unmountSettings();root=createRoot(target);root.render(<Settings onBackup={onBackup} onAuth={onAuth}/>);
}
export function mountPersonalSettings(target:Element){
  unmountSettings();root=createRoot(target);root.render(<PersonalSettings/>);
}
function Settings({onBackup,onAuth}:{onBackup:()=>void;onAuth:()=>void}){
  const {t}=useTranslation();
  return <>
    <div className="hero"><h1>{t('settings')}</h1><p className="muted">{t('系统配置与数据管理')}</p></div>
    <section className="section"><h2>{t('备份与恢复')}</h2><div className="controls"><Button variant="outline" onClick={onBackup}>{t('打开备份与恢复')}</Button></div></section>
    <section className="section"><h2>{t('认证设置')}</h2><p className="muted">{t('配置身份源、用户绑定和本地登录。')}</p><Button variant="outline" onClick={onAuth}>{t('打开认证设置')}</Button></section>
  </>;
}
function PersonalSettings(){
  const {t}=useTranslation();
  const [session,setSession]=useState<SessionInfo|null>(null),[profile,setProfile]=useState<Profile|null>(null),[email,setEmail]=useState(''),[notice,setNotice]=useState(''),[error,setError]=useState('');
  useEffect(()=>{let active=true;api<SessionInfo>('/api/auth/session').then(value=>{if(active)setSession(value)}).catch(cause=>{if(active)setError(String(cause instanceof Error?cause.message:cause))});api<Profile>('/api/auth/profile').then(value=>{if(active){setProfile(value);setEmail(value.email)}}).catch(cause=>{if(active)setError(String(cause instanceof Error?cause.message:cause))});return()=>{active=false}},[]);
  const canChange=session?.can_change_password!==false;
  return <>
    <div className="hero"><h1>{t('个人设置')}</h1><p className="muted">{t('查看账号与管理清册会话')}</p></div>
    {error&&<p className="error" role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}{new URLSearchParams(location.search).get('bind')==='ok'&&<p role="status">{t('飞书绑定成功')}</p>}{new URLSearchParams(location.search).has('bind_error')&&<p className="error" role="alert">{t(new URLSearchParams(location.search).get('bind_error')==='session'?'绑定会话已失效，请重新登录后重试':new URLSearchParams(location.search).get('bind_error')==='disabled'?'此飞书绑定已禁用':new URLSearchParams(location.search).get('bind_error')==='failed'?'飞书授权失败，请重试':'飞书身份已被其他账号绑定或当前账号已有飞书身份')}</p>}
    {!session&&!error&&<p role="status">{t('正在加载')}</p>}
    {session&&<>
      <section className="section"><h2>{t('账号信息')}</h2><dl className="account-details"><div><dt>{t('账号')}</dt><dd>{session.login}</dd></div>{session.identity_type&&<div><dt>{t('登录身份')}</dt><dd>{t(({local:'本地账号',external:'外部账号',mixed:'混合账号'} as Record<string,string>)[session.identity_type]||session.identity_type)}</dd></div>}{session.auth_provider&&<div><dt>{t('认证提供方')}</dt><dd>{session.auth_provider}</dd></div>}</dl></section>
      <section className="section"><h2>{t('邮箱与飞书')}</h2><form onSubmit={async event=>{event.preventDefault();setError('');try{await api('/api/auth/profile',{method:'PUT',body:JSON.stringify({email})});setNotice(t('邮箱已保存'))}catch(cause){setError(String(cause instanceof Error?cause.message:cause))}}}><label>{t('邮箱')}<input type="email" value={email} onChange={event=>setEmail(event.target.value)} /></label><Button type="submit">{t('保存邮箱')}</Button></form>{profile?.feishu_enabled&&<p>{t(profile.feishu_bound?'已绑定飞书':'尚未绑定飞书')}</p>}{profile?.feishu_enabled&&!profile.feishu_bound&&profile.local_session&&<a className="external-login" href="/api/auth/feishu/bind/start">{t('绑定飞书')}</a>}</section>
      <section className="section"><h2>{t('密码')}</h2>{canChange?<Button variant="outline" onClick={()=>window.dispatchEvent(new Event('rollbook:open-password'))}>{t('改密码')}</Button>:<p className="muted">{t('此账号的密码由上游身份提供方管理，请在上游修改。')}</p>}</section>
      <section className="section"><h2>{t('清册会话')}</h2><p className="muted">{t('退出所有设备会撤销所有清册浏览器会话，包括当前会话；不会退出上游 SSO，也不会撤销 API Key。')}</p><Button variant="destructive" onClick={()=>window.dispatchEvent(new Event('rollbook:logout-all'))}>{t('退出所有设备')}</Button></section>
    </>}
  </>;
}
