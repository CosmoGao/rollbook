import React, { useState } from 'react';
import { createRoot } from 'react-dom/client';
import * as Dialog from '@radix-ui/react-dialog';
import { Button } from './components/ui/button';

export const formFields:Record<string,string[]>={
 location:['kind','parent_id','city','region','address','timezone','description'],
 uplink:['location_id','isp','type','bandwidth','public_ipv4','public_ipv4_addr','public_ipv6','ddns','inbound_ports_json','edge_device_id','account_ref'],
 host:['kind','net_role','uplink_id','parent_host_id','location_id','model','os','os_version','resources_json','mgmt_url'],
 network:['cidr','location_id','router_id','uplink_id','vlan_id','gateway','dhcp_range','purpose'],
 ip:['address','network_id','host_id','interface','mac','assign'],
 service:['host_id','kind','deploy_method','upstream_json','owner','config_path','docs_url'],
 port:['service_id','bind','port','protocol','purpose'],
 endpoint:['service_id','kind','url','via_json','auth','exposure','nav_card_json'],
 dependency:['from_id','to_id','kind','note'],
 version:['service_id','current_version','current_source','latest_version','latest_source','latest_url','eol_date','pinned','pinned_reason','checked_at'],
 credential:['target_id','vault','item_ref','purpose'],
 note:['target_id','kind','body_md']
};
const requiredFields:Record<string,string[]>={location:['kind'],uplink:['location_id'],host:['kind','location_id'],network:['cidr','location_id'],ip:['address','host_id'],service:['host_id'],port:['service_id','port','protocol'],endpoint:['service_id','kind','url'],dependency:['from_id','to_id','kind'],version:['service_id'],credential:['target_id','vault','item_ref'],note:['target_id','kind','body_md']};
const choices:any={status:['在用','停用','计划中'],source:['人工','agent','自动发现'],confidence:['低','中','高'],kind:{location:['站点','房间','机柜','桌面'],host:['物理机','NAS','虚拟机','LXC','容器','网络设备'],endpoint:['内网直连','反向代理','Cloudflare Tunnel','导航页卡片'],note:['运维笔记','故障','决策','待办']},net_role:['光猫','主路由','旁路由','交换机','AP'],type:{uplink:['光纤宽带','专线','4G·5G','其他']},public_ipv4:['无或CGNAT','动态公网','固定公网','待核实'],assign:['静态','DHCP保留','动态'],protocol:['tcp','udp']};
const relationType:Record<string,string>={parent_id:'location',location_id:'location',uplink_id:'uplink',edge_device_id:'host',account_ref:'credential',parent_host_id:'host',router_id:'host',network_id:'network',host_id:'host',service_id:'service'};
const editableTypes=Object.keys(formFields);
function choiceValues(type:string,field:string){const value=choices[field];return Array.isArray(value)?value:value?.[type]||null}

type Asset={id:string;type:string;name:string;rev:number;fields?:Record<string,any>;status?:string;source?:string;confidence?:string};
type API=<T>(path:string,options?:RequestInit)=>Promise<T>;
type Props={type:string;original:Asset|null;api:API;translate:(value:string)=>string;enumLabel:(field:string,value:string)=>string;types:Record<string,string>;open:(id:string,type:string)=>void;close:()=>void};
export function openAssetForm(type:string,original:Asset|null,api:API,translate:(value:string)=>string,enumLabel:(field:string,value:string)=>string,types:Record<string,string>,open:(id:string,type:string)=>void){
  const host=document.createElement('div');document.body.append(host);const root=createRoot(host);
  const close=()=>{root.unmount();host.remove()};
  root.render(<AssetForm {...{type,original,api,translate,enumLabel,types,open,close}}/>);
}
function AssetForm({type,original,api,translate:t,enumLabel,types,open,close}:Props){
  const [related,setRelated]=useState<Record<string,Asset[]>>({}),[loading,setLoading]=useState(true),[error,setError]=useState(''),[busy,setBusy]=useState(false);
  const [id,setId]=useState(()=>type+':'+Array.from(crypto.getRandomValues(new Uint8Array(16)),value=>value.toString(16).padStart(2,'0')).join(''));
  const [name,setName]=useState(original?.name||''),[reason,setReason]=useState('');
  const [values,setValues]=useState<Record<string,any>>(()=>Object.fromEntries(['status','source','confidence',...formFields[type]].map(field=>[field,field in (original?.fields||{})?original?.fields?.[field]:(original as any)?.[field]??''])));
  React.useEffect(()=>{let active=true;const needed=new Set(formFields[type].map(field=>relationType[field]).filter(Boolean));if(['dependency','credential','note'].includes(type))editableTypes.forEach(item=>needed.add(item));if(type==='endpoint')needed.add('service');Promise.all([...needed].map(async item=>[item,await api<Asset[]>('/api/browse?type='+item)] as const)).then(result=>{if(active){setRelated(Object.fromEntries(result));setLoading(false)}}).catch(e=>{if(active){setError(String(e));setLoading(false)}});return()=>{active=false}},[type]);
  const set=(field:string,value:any)=>setValues(previous=>({...previous,[field]:value}));
  const submit=async(event:React.FormEvent)=>{event.preventDefault();setBusy(true);setError('');try{const fields:Record<string,any>={};for(const field of formFields[type]){let value=values[field];if(value===''&&!original)continue;if(field.endsWith('_json')&&typeof value==='string'&&value){try{value=JSON.parse(value)}catch{throw Error(field+'：'+t('请输入有效 JSON'))}}if(['port','vlan_id'].includes(field)&&value!=='')value=Number(value);if(field==='pinned')value=Number(Boolean(value));fields[field]=value===''?null:value}const common:Record<string,string>={};for(const field of ['status','source','confidence'])if(values[field])common[field]=values[field];if(original){await api('/api/assets/'+encodeURIComponent(original.id),{method:'PATCH',body:JSON.stringify({rev:original.rev,patch:{name:name.trim(),fields,...common},reason:reason.trim()})});close();open(original.id,type)}else{const object={id:id.trim(),type,name:name.trim(),fields,...common};await api('/api/assets',{method:'POST',body:JSON.stringify({object,base_rev:0,reason:reason.trim()})});close();open(object.id,type)}}catch(e){setError(String(e instanceof Error?e.message:e));setBusy(false)}};
  const control=(field:string)=>{const selected=values[field]??'',relation=relationType[field],options=choiceValues(type,field);const isRelation=!!relation||['from_id','to_id','target_id','via_json'].includes(field);const pool=relation?related[relation]||[]:Object.values(related).flat();const required=requiredFields[type].includes(field);let multiple:string[]=[];if(field==='via_json'){try{const value=typeof selected==='string'&&selected?JSON.parse(selected):selected;multiple=Array.isArray(value)?value:[]}catch{multiple=[]}}if(isRelation)return <select required={required} multiple={field==='via_json'} size={field==='via_json'?4:undefined} value={field==='via_json'?multiple:selected} onChange={event=>set(field,field==='via_json'?[...event.target.selectedOptions].map(option=>option.value):event.target.value)}>{field!=='via_json'&&<option value="">{t('请选择')}</option>}{pool.filter(asset=>asset.id!==original?.id).map(asset=><option value={asset.id} key={asset.id}>{t(types[asset.type]||asset.type)} · {asset.name} ({asset.id})</option>)}</select>;
    if(options)return <select required={required} value={selected} onChange={event=>set(field,event.target.value)}>{!required&&<option value="">{t('请选择')}</option>}{options.map((value:string)=><option value={value} key={value}>{enumLabel(field,value)}</option>)}</select>;
    if(field.endsWith('_json')||['description','note','body_md'].includes(field))return <textarea required={required} value={typeof selected==='object'?JSON.stringify(selected):selected} onChange={event=>set(field,event.target.value)}/>;
    if(field==='pinned')return <input type="checkbox" checked={Boolean(selected)} onChange={event=>set(field,event.target.checked)}/>;
    return <input type={['port','vlan_id'].includes(field)?'number':'text'} required={required} value={selected} onChange={event=>set(field,event.target.value)}/>};
  return <Dialog.Root open onOpenChange={open=>{if(!open)close()}}><Dialog.Portal><Dialog.Overlay className="modal-backdrop"/><Dialog.Content className="modal radix-modal"><Dialog.Title asChild><h2>{t((original?'编辑':'新建')+(types[type]||type))}</h2></Dialog.Title>{loading?<div className="notice">{t('正在加载…')}</div>:<form className="asset-form" onSubmit={submit}>{!original&&<label>{t('ID（可修改）')}<input value={id} onChange={event=>setId(event.target.value)} required/></label>}<label>{t('名称')}<input value={name} onChange={event=>setName(event.target.value)} required/></label>{['status','source','confidence',...formFields[type]].map(field=><label key={field}>{field}{requiredFields[type].includes(field)?' *':''}{control(field)}</label>)}<label>{t('操作原因 *')}<input required value={reason} placeholder={t('例如：登记新主机、更新服务配置')} onChange={event=>setReason(event.target.value)}/></label>{error&&<div className="error" role="alert">{error}</div>}<div className="modal-actions"><Button type="button" variant="ghost" className="ghost" onClick={close}>{t('取消')}</Button><Button type="submit" disabled={busy}>{t(original?'保存修改':'创建')}</Button></div></form>}</Dialog.Content></Dialog.Portal></Dialog.Root>
}
