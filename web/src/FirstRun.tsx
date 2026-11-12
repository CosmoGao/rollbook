import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import * as Dialog from '@radix-ui/react-dialog';
import { Button } from './components/ui/button';

type API=<T>(path:string,options?:RequestInit)=>Promise<T>;
export function openFirstRun(api:API,t:(value:string)=>string,onSite:()=>void,onImported:()=>void,onClosed:()=>void){
  const host=document.createElement('div');document.body.append(host);const root=createRoot(host);
  const close=()=>{root.unmount();host.remove();onClosed()};
  root.render(<FirstRun {...{api,t,close,onSite,onImported}}/>);
}
function FirstRun({api,t,close,onSite,onImported}:{api:API;t:(value:string)=>string;close:()=>void;onSite:()=>void;onImported:()=>void}){
  const [busy,setBusy]=useState(false),[error,setError]=useState('');
  const sample=async()=>{setBusy(true);setError('');try{const suffix=Array.from(crypto.getRandomValues(new Uint8Array(16)),value=>value.toString(16).padStart(2,'0')).join('');const id=(type:string)=>type+':sample-'+suffix;const examples=[
    {type:'location',name:'【示例】云杉站点',fields:{kind:'站点'}},
    {type:'network',name:'【示例】文档网段',fields:{cidr:'192.0.2.0/24',location_id:id('location')}},
    {type:'host',name:'【示例】演示主机',fields:{kind:'虚拟机',location_id:id('location')}},
    {type:'ip',name:'【示例】192.0.2.10',fields:{address:'192.0.2.10',network_id:id('network'),host_id:id('host')}},
    {type:'service',name:'【示例】演示服务',fields:{host_id:id('host')}},
    {type:'endpoint',name:'【示例】演示入口',fields:{service_id:id('service'),kind:'内网直连',url:'https://demo.example.invalid'}}
  ];for(const item of examples)await api('/api/assets',{method:'POST',body:JSON.stringify({object:{...item,id:id(item.type),source:'人工'},base_rev:0,reason:'导入虚构示例数据'})});close();onImported()}catch(e){setError(String(e instanceof Error?e.message:e));setBusy(false)}};
  return <Dialog.Root open onOpenChange={open=>{if(!open)close()}}><Dialog.Portal><Dialog.Overlay className="modal-backdrop"/><Dialog.Content className="modal radix-modal"><Dialog.Title asChild><h2>{t('欢迎使用清册')}</h2></Dialog.Title><p className="muted">{t('清册还是空的。可以建立第一个站点，或导入明确标记的虚构示例数据。示例使用文档专用 IP 和 .invalid 域名，不代表真实资产。')}</p><div className="controls"><Button onClick={()=>{close();onSite()}}>{t('建立第一个站点')}</Button><Button variant="outline" disabled={busy} onClick={sample}>{t('导入虚构示例数据')}</Button></div>{error&&<div className="error" role="alert">{error}</div>}<Button variant="ghost" onClick={close}>{t('稍后再说')}</Button></Dialog.Content></Dialog.Portal></Dialog.Root>
}
