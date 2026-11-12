import { mountOverview, unmountOverview } from './Overview';
import { mountAssets, unmountAssets } from './Assets';
import { mountSettings, unmountSettings } from './Settings';
import { mountSearch, unmountSearch } from './Search';
import { mountPending, unmountPending } from './Pending';
import { mountKeys, unmountKeys } from './Keys';
import { mountBackup, unmountBackup } from './Backup';
import { mountAudit, unmountAudit } from './Audit';
import { mountTopology, unmountTopology } from './Topology';
import { openAssetForm } from './AssetForm';
import { mountDetail, unmountDetail } from './AssetDetail';
import { openFirstRun } from './FirstRun';
import { mountBrowse, unmountBrowse } from './Browse';
import { t, enumLabel, setLanguage, language } from './catalog-i18n';

const root=document.getElementById('content')!;
const labels:Record<string,string>={overview:'总览',topology:'拓扑',pendingHub:'待处理',agent:'Agent 接入',settings:'设置',search:'全局搜索',assets:'资产维护',sites:'站点',hosts:'主机',networks:'网段',outdated:'有新版本',pending:'待确认',changes:'最近变更',contexts:'共享上下文',unverified:'待核实',keys:'Key 管理',backup:'备份与恢复'};
const types:Record<string,string>={location:'站点',uplink:'网络接入',host:'主机',network:'网段',ip:'IP 地址',service:'服务',port:'端口',endpoint:'访问入口',version:'版本',credential:'凭据引用',note:'笔记',change:'变更',dependency:'依赖'};
const routePaths:Record<string,string>={overview:'/',assets:'/assets',services:'/assets/services',sites:'/assets/sites',hosts:'/assets/hosts',networks:'/assets/networks',topology:'/topology',pendingHub:'/pending',pending:'/pending',unverified:'/pending?tab=unverified',outdated:'/pending?tab=outdated',agent:'/agent',keys:'/agent',settings:'/settings',backup:'/settings/backup',changes:'/changes',contexts:'/contexts',search:'/search'};
let pendingSearchTerm='',firstRunOpen=false;
const cookie=(name:string)=>document.cookie.split('; ').find(value=>value.startsWith(name+'='))?.slice(name.length+1)||'';
export async function api<T=any>(path:string,options:RequestInit={}):Promise<T>{
  const headers=new Headers(options.headers);
  headers.set('Accept','application/json');headers.set('X-Rollbook-Language',language);
  if(options.body&&!headers.has('Content-Type'))headers.set('Content-Type','application/json');
  if(options.method&&options.method!=='GET')headers.set('X-CSRF-Token',cookie('rollbook_csrf'));
  const response=await fetch(path,{...options,credentials:'same-origin',headers});
  const data=await response.json().catch(()=>({}));
  if(!response.ok){if(response.status===401&&path!=='/api/auth/login')window.dispatchEvent(new Event('rollbook:unauthorized'));const fields=(data.fields||[]).map((field:{field:string;reason:string;expected?:string;references?:string[]})=>`${field.field}: ${field.reason}${field.expected?' ('+t('期望：')+field.expected+')':''}${field.references?.length?'; '+t('引用方：')+field.references.join(', '):''}`);throw new Error(t(fields.join('\n')||data.message||`请求失败 (${response.status})`))}
  return data as T;
}
const unmountAll=()=>{unmountOverview();unmountAssets();unmountSettings();unmountSearch();unmountPending();unmountKeys();unmountBackup();unmountAudit();unmountTopology();unmountDetail();unmountBrowse()};
const error=(value:unknown)=>{const element=document.createElement('div');element.className='error';element.textContent=value instanceof Error?value.message:String(value);root.replaceChildren(element)};
export function openObject(id:string,type?:string){unmountAll();document.getElementById('crumb')!.textContent=t('资产详情');document.querySelectorAll('#nav button').forEach(button=>button.classList.remove('active'));root.replaceChildren();mountDetail(root,{id,type,api,t,enumLabel,types,open:openObject,navigate})}
const assetForm=(type:string)=>openAssetForm(type,null,api,t,enumLabel,types,openObject);
export async function navigate(view:string,push=true){
  unmountAll();const target=routePaths[view];if(push&&target&&target!==location.pathname+location.search)history.pushState({},'',target);
  const menuView=['pending','unverified','outdated'].includes(view)?'pendingHub':view==='services'?'assets':view;
  document.getElementById('crumb')!.textContent=t(labels[menuView]||'资产详情');
  document.querySelectorAll<HTMLButtonElement>('#nav button[data-view]').forEach(button=>button.classList.toggle('active',button.dataset.view===menuView));
  root.replaceChildren();
  try{switch(view){
    case 'overview':mountOverview(root,api,navigate);break;
    case 'assets':mountAssets(root,api,assetForm,openObject,navigate);break;
    case 'services':mountAssets(root,api,assetForm,openObject,navigate,'service');break;
    case 'sites':case 'hosts':case 'networks':mountBrowse(root,view,api,openObject,enumLabel);break;
    case 'topology':mountTopology(root,api,openObject);break;
    case 'pendingHub':case 'pending':case 'unverified':case 'outdated':mountPending(root,api,openObject,navigate,view==='pendingHub'?'pending':view);break;
    case 'agent':case 'keys':mountKeys(root,api);break;
    case 'settings':mountSettings(root,()=>navigate('backup'),()=>document.getElementById('change-password')?.click(),()=>document.getElementById('logout-all')?.click(),setLanguage);break;
    case 'backup':mountBackup(root,api,()=>window.dispatchEvent(new Event('rollbook:unauthorized')));break;
    case 'search':mountSearch(root,api,openObject,pendingSearchTerm);pendingSearchTerm='';break;
    case 'changes':case 'contexts':mountAudit(root,view,api,openObject);break;
  }}catch(cause){error(cause)}
}
export function setSearchQuery(value:string){pendingSearchTerm=value}
export function buildVersionLabel(info:{version?:string;commit?:string}){const version=String(info?.version||'').trim(),commit=String(info?.commit||'').trim().match(/^[0-9a-f]{7}/i)?.[0].toLowerCase();if(/^v\d+\.\d+\.\d+$/.test(version))return `Rollbook ${version}`;if(version==='main'||version==='dev')return `Rollbook ${version}${commit?' '+commit:''}`;return 'Rollbook'}
export async function showFirstRun(){if(firstRunOpen)return;try{const sites=await api<any[]>('/api/browse?type=location');if(sites.length)return;firstRunOpen=true;openFirstRun(api,t,()=>assetForm('location'),()=>navigate('sites'),()=>{firstRunOpen=false})}catch(cause){console.error('首次引导检查失败',cause)}}
function routeFromLocation(){const path=location.pathname,tab=new URLSearchParams(location.search).get('tab');if(path==='/pending')return tab==='unverified'?'unverified':tab==='outdated'?'outdated':'pendingHub';return Object.keys(routePaths).find(key=>routePaths[key]===path)||'overview'}
window.addEventListener('popstate',()=>navigate(routeFromLocation(),false));
