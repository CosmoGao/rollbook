import { mountOverview, unmountOverview } from './Overview';
import { mountAssets, unmountAssets } from './Assets';
import { mountSettings, mountPersonalSettings, unmountSettings } from './Settings';
import {mountAuthSettings,unmountAuthSettings,canLeaveAuthSettings,authSettingsLocation} from './AuthSettings';
export {canLeaveAuthSettings} from './AuthSettings';
import { mountSearch, unmountSearch } from './Search';
import { mountPending, unmountPending } from './Pending';
import { mountKeys, unmountKeys } from './Keys';
import { mountBackup, unmountBackup } from './Backup';
import { mountAudit, unmountAudit } from './Audit';
import { mountTopology, unmountTopology } from './Topology';
import { openAssetForm } from './AssetForm';
import { mountDetail, unmountDetail } from './AssetDetail';
import { openFirstRun } from './FirstRun';
import { unmountBrowse } from './Browse';
import { t, enumLabel, setLanguage, language } from './catalog-i18n';

const root=document.getElementById('content')!;
const labels:Record<string,string>={overview:'总览',topology:'拓扑',pendingHub:'待处理',agent:'Agent 接入',settings:'设置',search:'全局搜索',assets:'资产维护',sites:'站点',hosts:'设备与主机',networks:'网段',outdated:'有新版本',pending:'待确认',changes:'最近变更',contexts:'共享上下文',unverified:'待核实',keys:'Key 管理',backup:'备份与恢复',authSettings:'认证设置'};
const types:Record<string,string>={location:'站点',uplink:'网络接入',host:'设备与主机',network:'网段',ip:'IP 地址',service:'服务',port:'端口',endpoint:'访问入口',version:'版本',credential:'凭据引用',note:'笔记',change:'变更',dependency:'依赖'};
const routePaths:Record<string,string>={overview:'/',assets:'/assets',services:'/assets/services',sites:'/assets/sites',hosts:'/assets/hosts',networks:'/assets/networks',topology:'/topology',pendingHub:'/pending',pending:'/pending',unverified:'/pending?tab=unverified',outdated:'/pending?tab=outdated',agent:'/agent',keys:'/agent',settings:'/settings',personal:'/settings/personal',backup:'/settings/backup',authSettings:'/settings/auth',changes:'/changes',contexts:'/contexts',search:'/search'};
let pendingSearchTerm='',firstRunOpen=false;
const cookie=(name:string)=>document.cookie.split('; ').find(value=>value.startsWith(name+'='))?.slice(name.length+1)||'';
export async function api<T=any>(path:string,options:RequestInit={}):Promise<T>{
  const headers=new Headers(options.headers);
  headers.set('Accept','application/json');headers.set('X-Rollbook-Language',language);
  if(options.body&&!headers.has('Content-Type'))headers.set('Content-Type','application/json');
  if(options.method&&options.method!=='GET')headers.set('X-CSRF-Token',cookie('rollbook_csrf'));
  const response=await fetch(path,{...options,credentials:'same-origin',headers});
  const data=await response.json().catch(()=>({}));
  if(!response.ok){if(response.status===401&&path!=='/api/auth/login')window.dispatchEvent(new Event('rollbook:unauthorized'));const fieldNames:Record<string,string>={client_id:path.includes('/feishu')?'App ID':'Client ID',redirect_url:'回调 URL',secret_action:'敏感凭据'};const fields=(data.fields||[]).map((field:{field:string;reason:string;expected?:string;references?:string[]})=>`${t(fieldNames[field.field]||field.field)}: ${field.reason}${field.expected?' ('+t('期望：')+field.expected+')':''}${field.references?.length?'; '+t('引用方：')+field.references.join(', '):''}`);throw new Error(t(fields.join('\n')||data.message||`请求失败 (${response.status})`))}
  return data as T;
}
const unmountAll=()=>{unmountOverview();unmountAssets();unmountSettings();unmountAuthSettings();unmountSearch();unmountPending();unmountKeys();unmountBackup();unmountAudit();unmountTopology();unmountDetail();unmountBrowse()};
const error=(value:unknown)=>{const element=document.createElement('div');element.className='error';element.textContent=value instanceof Error?value.message:String(value);root.replaceChildren(element)};
export function openObject(id:string,type?:string){unmountAll();document.querySelectorAll('#nav button').forEach(button=>button.classList.remove('active'));document.querySelectorAll('#nav button').forEach(button=>button.removeAttribute('aria-current'));root.replaceChildren();mountDetail(root,{id,type,api,t,enumLabel,types,open:openObject,navigate})}
const assetForm=(type:string)=>openAssetForm(type,null,api,t,enumLabel,types,openObject);
export async function navigate(view:string,push=true,confirmed=false){
  if(!confirmed&&root.querySelector('.auth-settings')&&!canLeaveAuthSettings())return;
  unmountAll();const target=routePaths[view];if(push&&target&&target!==location.pathname+location.search)history.pushState({},'',target);
  const menuView=['pending','unverified','outdated'].includes(view)?'pendingHub':['services','sites','hosts','networks'].includes(view)?'assets':view==='authSettings'||view==='backup'?'settings':view;

  document.querySelectorAll<HTMLButtonElement>('#nav button[data-view]').forEach(button=>{const active=button.dataset.view===menuView;button.classList.toggle('active',active);if(active)button.setAttribute('aria-current','page');else button.removeAttribute('aria-current')});window.dispatchEvent(new CustomEvent('rollbook:view',{detail:menuView}));
  root.replaceChildren();
  try{switch(view){
    case 'overview':mountOverview(root,api,navigate,(term)=>{setSearchQuery(term);navigate('search')},()=>assetForm('location'),(siteID)=>{history.pushState({},'',routePaths.topology+'?site='+encodeURIComponent(siteID));navigate('topology',false)});break;
    case 'assets':mountAssets(root,api,assetForm,openObject,enumLabel);break;
    case 'services':mountAssets(root,api,assetForm,openObject,enumLabel,'service');break;
    case 'sites':mountAssets(root,api,assetForm,openObject,enumLabel,'location');break;
    case 'hosts':mountAssets(root,api,assetForm,openObject,enumLabel,'host','tree');break;
    case 'networks':mountAssets(root,api,assetForm,openObject,enumLabel,'network');break;
    case 'topology':mountTopology(root,api,openObject);break;
    case 'pendingHub':case 'pending':case 'unverified':case 'outdated':mountPending(root,api,openObject,navigate,view==='pendingHub'?'pending':view);break;
    case 'agent':case 'keys':mountKeys(root,api);break;
    case 'settings':mountSettings(root,()=>navigate('backup'),()=>navigate('authSettings'));break;
    case 'authSettings':mountAuthSettings(root);break;
    case 'personal':mountPersonalSettings(root);break;
    case 'backup':mountBackup(root,api,()=>window.dispatchEvent(new Event('rollbook:unauthorized')));break;
    case 'search':mountSearch(root,api,openObject,pendingSearchTerm);pendingSearchTerm='';break;
    case 'changes':case 'contexts':mountAudit(root,view,api,openObject);break;
  }}catch(cause){error(cause)}
}
export function setSearchQuery(value:string){pendingSearchTerm=value}
export function buildVersionLabel(info:{version?:string;commit?:string}){const version=String(info?.version||'').trim(),commit=String(info?.commit||'').trim().match(/^[0-9a-f]{7}/i)?.[0].toLowerCase();if(/^v\d+\.\d+\.\d+$/.test(version))return `Rollbook ${version}`;if(version==='main'||version==='dev')return `Rollbook ${version}${commit?' '+commit:''}`;return 'Rollbook'}
export async function showFirstRun(){if(firstRunOpen)return;try{const sites=await api<any[]>('/api/browse?type=location');if(sites.length)return;firstRunOpen=true;openFirstRun(api,t,()=>assetForm('location'),()=>navigate('sites'),()=>{firstRunOpen=false})}catch(cause){console.error('首次引导检查失败',cause)}}
function routeFromLocation(){const path=location.pathname,tab=new URLSearchParams(location.search).get('tab');if(path==='/pending')return tab==='unverified'?'unverified':tab==='outdated'?'outdated':'pendingHub';return Object.keys(routePaths).find(key=>routePaths[key]===path)||'overview'}
let lastLocation=location.pathname+location.search;
window.addEventListener('popstate',()=>{
  const next=location.pathname+location.search;
  if(location.pathname==='/settings/auth'&&root.querySelector('.auth-settings')){window.dispatchEvent(new Event('rollbook:auth-location'));lastLocation=location.pathname+location.search;return}
  if(root.querySelector('.auth-settings')&&!canLeaveAuthSettings()){history.pushState({},'',authSettingsLocation());return}
  lastLocation=next;void navigate(routeFromLocation(),false,true);
});
