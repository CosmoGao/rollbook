import React from 'react';
import { Button } from './ui/button';

export type TreeAsset={id:string;type:string;name:string;status?:string;site_id?:string;site_name?:string;primary_ip?:string;kind?:string;fields?:Record<string,any>};

export function HostTree({hosts,services,ips,open,enumLabel,en}:{hosts:TreeAsset[];services:TreeAsset[];ips:TreeAsset[];open:(id:string,type:string)=>void;enumLabel:(field:string,value:string)=>string;en:boolean}){
  const byHost=(list:TreeAsset[],host:string)=>list.filter(item=>item.fields?.host_id===host);
  const rank=(item:TreeAsset)=>({物理机:0,NAS:0,虚拟机:1,LXC:2,容器:2,网络设备:3} as Record<string,number>)[item.fields?.kind]??4;
  const children=(host:TreeAsset)=>[...hosts.filter(item=>item.fields?.parent_host_id===host.id).sort((a,b)=>rank(a)-rank(b)||a.name.localeCompare(b.name)),...byHost(services,host.id)];
  const seen=new Set<string>();
  const node=(item:TreeAsset,depth=0):React.ReactNode=>{if(seen.has(item.id))return null;seen.add(item.id);const ipHost=item.type==='service'?item.fields?.host_id:item.id;const own=item.primary_ip||byHost(ips,ipHost).map(address=>address.fields?.address).join(', ')||'—';return <details className="tree-node" key={item.id} open={depth<2}><summary><span className="tree-title" onClick={event=>{event.preventDefault();open(item.id,item.type)}}>{item.name}</span><span className="tree-meta">{item.type==='host'?enumLabel('kind',item.kind||item.fields?.kind):en?'Service':'服务'} · IP {own} · {enumLabel('status',item.status||'')}</span><Button variant="ghost" onClick={event=>{event.preventDefault();open(item.id,item.type)}}>{en?'View':'查看'}</Button></summary><div className="tree-children">{children(item).map(child=>node(child,depth+1))}</div></details>};
  const roots=hosts.filter(item=>!item.fields?.parent_host_id||!hosts.some(host=>host.id===item.fields?.parent_host_id)).sort((a,b)=>rank(a)-rank(b)||a.name.localeCompare(b.name));
  const sites=[...new Map(roots.map(item=>[item.site_id||'',item.site_name||(en?'Unknown site':'未指定站点')])).entries()].sort((a,b)=>a[1].localeCompare(b[1]));
  return <div className="host-tree">{sites.map(([id,name])=><section className="host-site" key={id}><h2>{name}</h2>{roots.filter(item=>(item.site_id||'')===id).map(item=>node(item))}</section>)}{hosts.filter(item=>!seen.has(item.id)).map(item=>node(item))}</div>;
}
