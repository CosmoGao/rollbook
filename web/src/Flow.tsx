import React from 'react';
import { createRoot, Root } from 'react-dom/client';
import { ReactFlow, Background, Controls, MiniMap, Node, Edge, MarkerType, Position } from '@xyflow/react';
import dagre from 'dagre';
import '@xyflow/react/dist/style.css';

type GraphDetail={id:string;name:string;type:string;kind?:string;count?:number;focus?:boolean;label?:React.ReactNode;width?:number;height?:number};
type GraphEdge={from:string;to:string;kind:string;count?:number};
type Graph={nodes?:string[];details?:GraphDetail[];edges?:GraphEdge[]};
let root:Root|undefined;
export function unmountFlow(){if(root){root.unmount();root=undefined}}
export function mountFlow(target:Element,data:Graph,onOpen:(id:string,type:string)=>void,direction:'TB'|'LR'='TB'){
 unmountFlow();
 const details:GraphDetail[]=data.details?.length?data.details:(data.nodes||[]).map(id=>({id,name:id,type:id.split(':')[0]}));
 const ids=new Set(details.map(item=>item.id));
 const links=(data.edges||[]).filter(edge=>ids.has(edge.from)&&ids.has(edge.to));
 const graph=new dagre.graphlib.Graph();graph.setGraph({rankdir:direction,nodesep:42,ranksep:100,marginx:48,marginy:48,ranker:'network-simplex'});graph.setDefaultEdgeLabel(()=>({}));
 for(const item of details)graph.setNode(item.id,{width:item.width||190,height:item.height||66});
 for(const edge of links)if(!links.some(link=>link.kind==='包含')||['包含','宿主','父主机'].includes(edge.kind))graph.setEdge(edge.from,edge.to);
 dagre.layout(graph);
 const english=document.documentElement.lang.startsWith('en');const typeLabel=(value:string)=>english?({location:'Site',host:'Host',service:'Service',endpoint:'Endpoint','物理机':'Physical','虚拟机':'VM','容器':'Container','网络设备':'Network device','站点':'Site','机柜':'Rack','房间':'Room','桌面':'Desk','服务':'Service','入口':'Endpoint'} as Record<string,string>)[value]||value:({location:'站点',host:'主机',service:'服务',endpoint:'入口'} as Record<string,string>)[value]||value;
 const nodes:Node[]=details.map(item=>{const p=graph.node(item.id),width=item.width||190,height=item.height||66;return{id:item.id,sourcePosition:direction==='LR'?Position.Right:Position.Bottom,targetPosition:direction==='LR'?Position.Left:Position.Top,position:{x:p.x-width/2,y:p.y-height/2},data:{label:item.label||<span className="flow-label"><strong>{item.name}</strong><small>{typeLabel(item.kind||item.type)}{item.count?' · +'+item.count:''}</small></span>},style:{width,height,border:item.focus?'2px solid var(--primary)':'1px solid var(--border)',borderRadius:10,padding:item.label?0:8,background:'var(--flow-node-bg,#eff6ff)',color:'var(--flow-node-text,#1e3a8a)'}}});
 const edges:Edge[]=links.map((edge,index)=>({id:`${edge.from}-${edge.to}-${index}`,source:edge.from,target:edge.to,label:edge.kind+(edge.count&&edge.count>1?' · '+edge.count:''),className:['包含','宿主','父主机'].includes(edge.kind)?'flow-structural':'flow-relation',markerEnd:{type:MarkerType.ArrowClosed},style:{stroke:'var(--muted)',strokeWidth:edge.count&&edge.count>1?2:1}}));
 const bounds=graph.graph();const width=target.clientWidth||960,height=target.clientHeight||620;const zoom=Math.min(1,width/((bounds.width||width)+100),height/((bounds.height||height)+100));const defaultViewport={x:(width-(bounds.width||width)*zoom)/2,y:(height-(bounds.height||height)*zoom)/2,zoom};
 root=createRoot(target);
 root.render(<ReactFlow nodes={nodes} edges={edges} minZoom={0.05} defaultViewport={defaultViewport} nodesDraggable={false} onNodeClick={(_,node)=>onOpen(node.id,details.find(item=>item.id===node.id)?.type||node.id.split(':')[0])}><Background/><Controls/>{!details.some(item=>item.label)&&<MiniMap/>}</ReactFlow>);
}
