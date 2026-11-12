import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter, useNavigate } from 'react-router-dom';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import i18next from 'i18next';
import { initReactI18next, useTranslation } from 'react-i18next';
import * as Dialog from '@radix-ui/react-dialog';
import { Menu, Search, SunMoon } from 'lucide-react';
import { Button } from './components/ui/button';
import './style.css';

const zh = { overview:'总览', assets:'资产', topology:'拓扑', pendingHub:'待处理', agent:'Agent 接入', settings:'设置', search:'全局搜索' };
const en = { overview:'Overview', assets:'Assets', topology:'Topology', pendingHub:'To do', agent:'Agent access', settings:'Settings', search:'Search' };
const stored = localStorage.getItem('rollbook.language') || (navigator.language.startsWith('en') ? 'en' : 'zh-CN');
i18next.use(initReactI18next).init({ lng: stored, fallbackLng:'zh-CN', resources:{ 'zh-CN':{translation:zh}, en:{translation:en} }, interpolation:{escapeValue:false} });
const query = new QueryClient();
type Legacy = typeof import('./legacy.js');
const routes: Record<string,string> = { overview:'/', assets:'/assets', topology:'/topology', pendingHub:'/pending', agent:'/agent', settings:'/settings', search:'/search' };
function App(){
  const {t} = useTranslation();
  const router = useNavigate();
  const [legacy,setLegacy] = useState<Legacy|null>(null);
  const [theme,setTheme] = useState(localStorage.getItem('rollbook.theme') || 'system');
  const [mobile,setMobile] = useState(false);
  const [term,setTerm] = useState('');
  const nav = (view:string) => { router(routes[view] || '/'+view); legacy?.navigate(view); setMobile(false); };
  useEffect(() => { import('./legacy.js').then(setLegacy); }, []);
  useEffect(() => { document.documentElement.dataset.theme = theme; localStorage.setItem('rollbook.theme',theme); },[theme]);
  useEffect(() => { const listener=(event:KeyboardEvent)=>{ if((event.ctrlKey||event.metaKey)&&event.key.toLowerCase()==='k'){event.preventDefault();nav('search');setTimeout(()=>document.querySelector<HTMLInputElement>('.search-box input')?.focus(),0)} }; document.addEventListener('keydown',listener);return ()=>document.removeEventListener('keydown',listener) },[legacy]);
  const menu = Object.keys(zh).filter(key=>key!=='search');
  const navigation = <><div className="logo"><span className="brand-mark">R</span><div><strong>Rollbook</strong><small>服务与资产清册</small></div></div><div className="nav-heading">{t('overview')}</div><nav id="nav">{menu.map((key,index)=><button key={key} type="button" data-view={key} onClick={()=>nav(key)}>{['◫','▦','◇','◷','⚿','⚙'][index]} <span>{t(key)}</span></button>)}</nav><p className="http-mode-notice sidebar-http-notice" hidden>当前为 HTTP 模式</p><div className="sidebar-foot"><span id="user-name"></span><button id="change-password" title="修改密码">改密码</button><button id="logout-all" title="退出所有设备">退出所有设备</button><button id="logout" title="退出登录">退出</button><small id="build-version"></small></div></>;
  return <><div id="login" className="login-wrap" hidden><form id="login-form" className="login-card"><div className="brand-mark">R</div><p className="eyebrow">ROLLBOOK / 清册</p><h1>登录清册</h1><p className="muted">用管理员账号查看服务与资产。</p><p className="http-mode-notice" hidden>当前为 HTTP 模式：口令和 Key 会在网络中明文传输，仅限可信内网使用。</p><label>账号<input name="login" autoComplete="username" required /></label><label>密码<input name="password" type="password" autoComplete="current-password" required /></label><button className="primary" type="submit">登录</button><p id="login-error" className="error" role="alert"></p></form><footer id="login-build-version" className="login-version" aria-label="Rollbook version"></footer></div>
  <div id="app" className="shell" hidden><aside className="sidebar">{navigation}</aside><div className="main"><header className="topbar"><Dialog.Root open={mobile} onOpenChange={setMobile}><Dialog.Trigger asChild><Button variant="ghost" className="mobile-menu" aria-label="打开菜单"><Menu size={19}/></Button></Dialog.Trigger><Dialog.Portal><Dialog.Overlay className="mobile-overlay"/><Dialog.Content className="mobile-drawer"><Dialog.Title>Rollbook</Dialog.Title>{menu.map(key=><button key={key} onClick={()=>nav(key)}>{t(key)}</button>)}</Dialog.Content></Dialog.Portal></Dialog.Root><div className="crumb">清册 <span>/</span> <b id="crumb">{t('overview')}</b></div><form className="top-search" onSubmit={e=>{e.preventDefault();nav('search');setTimeout(()=>{const input=document.querySelector<HTMLInputElement>('.search-box input');if(input){input.value=term;input.dispatchEvent(new Event('input',{bubbles:true}))}},0)}}><Search size={16}/><input value={term} onChange={e=>setTerm(e.target.value)} placeholder={t('search')+'  ⌘K'} aria-label={t('search')}/></form><div className="top-controls"><select id="language" aria-label="Language / 语言"><option value="zh-CN">中文</option><option value="en">English</option></select><Button variant="ghost" type="button" onClick={()=>setTheme(theme==='dark'?'light':theme==='light'?'system':'dark')} aria-label="切换主题"><SunMoon size={18}/></Button><button type="button" onClick={()=>document.getElementById('logout')?.click()} aria-label="退出登录">退出</button></div><form id="mobile-search" className="mobile-search" role="search"><input type="search" aria-label="搜索" placeholder="搜索 IP、服务…" autoComplete="off"/><button type="submit">搜索</button></form><div className="mobile-account"><span id="mobile-user-name"></span><button id="mobile-logout" type="button">退出</button></div></header><main id="content" className="content"></main></div></div></>
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><QueryClientProvider client={query}><BrowserRouter><App/></BrowserRouter></QueryClientProvider></React.StrictMode>);
