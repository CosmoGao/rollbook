import type { ReactNode } from 'react';
export type LayerNode={id:string;name:string;type:'location'|'host'|'service'|'endpoint';kind?:string;parent_id?:string;location_id?:string;host_id?:string;url?:string;ip?:string};
export type LayerEdge={from:string;to:string;kind:string};
export type Layers={nodes:LayerNode[];edges:LayerEdge[]};
export type DrawNode={id:string;name:string;type:string;kind?:string;count?:number;focus?:boolean;label?:ReactNode;width?:number;height?:number};
export type DrawEdge={from:string;to:string;kind:string;count?:number};
export type DrawGraph={details:DrawNode[];edges:DrawEdge[]};
const instances=new Set(['虚拟机','LXC','容器']);
export const isInstance=(kind?:string)=>instances.has(kind||'');
export function siteOf(node:LayerNode,byID:Map<string,LayerNode>):string|undefined{
 const seen=new Set<string>();let current:LayerNode|undefined=node;
 while(current&&!seen.has(current.id)){seen.add(current.id);if(current.type==='location'&&current.kind==='站点')return current.id;const next:string|undefined=current.type==='service'||current.type==='endpoint'?current.host_id:current.type==='host'?current.location_id:current.parent_id;current=next?byID.get(next):undefined}
 return undefined;
}
export function parentOf(node:LayerNode,byID:Map<string,LayerNode>):string|undefined{
 if(node.type==='endpoint'||node.type==='service')return node.host_id;
 if(node.type==='host')return node.parent_id&&byID.has(node.parent_id)?node.parent_id:node.location_id;
 return node.parent_id;
}
export function drawLayers(data:Layers,site:string,level:'physical'|'virtual'|'all',expanded:Set<string>,focus?:string):DrawGraph{
 const byID=new Map(data.nodes.map(n=>[n.id,n]));
 const ancestors=(id:string)=>{const chain=new Set<string>();while(id&&!chain.has(id)){chain.add(id);const node=byID.get(id);id=node?parentOf(node,byID)||'':''}return chain};
 const focusAncestors=focus?ancestors(focus):undefined;
 const neighbors=new Set<string>();if(focus)for(const edge of data.edges){if(edge.from===focus)neighbors.add(edge.to);if(edge.to===focus)neighbors.add(edge.from)}
 const related=new Set<string>();for(const id of neighbors)for(const ancestor of ancestors(id))related.add(ancestor);
 const selected=data.nodes.filter(n=>{
  const isRelated=related.has(n.id);
  const inSite=!site||(site==='__unassigned__'?!siteOf(n,byID):siteOf(n,byID)===site);
  return (inSite||isRelated)&&(!focus||focusAncestors?.has(n.id)||ancestors(n.id).has(focus)||isRelated);
 });
 const scope=new Set(selected.map(n=>n.id));
 const visible=new Set<string>();
 const isVisible=(node:LayerNode):boolean=>{
  if(node.type==='location')return true;
  if(node.type==='host'){
   if(!isInstance(node.kind))return true;
   if(!node.parent_id||!byID.has(node.parent_id))return true;
   // 循环部署链没有可折叠的根；保留节点供定位。
   const seen=new Set([node.id]);let parent:string|undefined=node.parent_id;
   while(parent&&byID.has(parent)&&!seen.has(parent)){seen.add(parent);parent=byID.get(parent)?.parent_id}
   if(parent&&seen.has(parent))return true;
   return level!=='physical'||expanded.has(node.parent_id);
  }
  if(node.type==='service')return level==='all'||!!node.host_id&&expanded.has(node.host_id);
  return level==='all'||!!node.host_id&&expanded.has(node.host_id);
 };
 selected.forEach(n=>{if(isVisible(n))visible.add(n.id)});
 // 聚焦对象的祖先始终可见，方便从详情直接定位。
 if(focus&&scope.has(focus)){let id:string|undefined=focus;const seen=new Set<string>();while(id&&!seen.has(id)){seen.add(id);visible.add(id);const n=byID.get(id);id=n?parentOf(n,byID):undefined}}
 const representative=(id:string):string|undefined=>{const seen=new Set<string>();while(id&&!seen.has(id)){if(visible.has(id))return id;seen.add(id);const n=byID.get(id);id=n?parentOf(n,byID)||'':''}return undefined};
 const count=new Map<string,number>();for(const n of selected){const r=representative(n.id);if(r&&r!==n.id)count.set(r,(count.get(r)||0)+1)}
 const details=selected.filter(n=>visible.has(n.id)).map(n=>({id:n.id,name:n.url||n.name,type:n.type,kind:n.kind,count:count.get(n.id),focus:n.id===focus}));
 const aggregated=new Map<string,DrawEdge>();
 const add=(from:string,to:string,kind:string)=>{const a=representative(from),b=representative(to);if(!a||!b||a===b)return;const key=[a,b,kind].join('\u0000');const old=aggregated.get(key);if(old)old.count=(old.count||1)+1;else aggregated.set(key,{from:a,to:b,kind,count:1})};
 for(const n of selected){const parent=parentOf(n,byID);if(parent&&scope.has(parent))add(parent,n.id,'包含')}
 for(const edge of data.edges)if(scope.has(edge.from)&&scope.has(edge.to))add(edge.from,edge.to,edge.kind);
 return {details,edges:[...aggregated.values()]};
}
