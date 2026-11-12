import React from 'react';
import { createRoot, Root } from 'react-dom/client';
import { ReactFlow, Background, Controls, MiniMap, Node, Edge } from '@xyflow/react';
import '@xyflow/react/dist/style.css';

type GraphDetail={id:string;name:string;type:string;kind?:string};
type GraphEdge={from:string;to:string;kind:string};
type Graph={nodes?:string[];details?:GraphDetail[];edges?:GraphEdge[]};
let root:Root|undefined;
export function unmountFlow(){if(root){root.unmount();root=undefined}}
export function mountFlow(target:Element,data:Graph,onOpen:(id:string,type:string)=>void,direction:'TB'|'LR'='TB'){
  unmountFlow();
  const details:GraphDetail[]=data.details?.length?data.details:(data.nodes||[]).map(id=>({id,name:id,type:id.split(':')[0]}));
  const ids=new Set(details.map(item=>item.id));
  const links=(data.edges||[]).filter(edge=>ids.has(edge.from)&&ids.has(edge.to));
  const indegree=new Map<string,number>(details.map(item=>[item.id,0]));
  const outgoing=new Map<string,string[]>(details.map(item=>[item.id,[]]));
  for(const edge of links){indegree.set(edge.to,(indegree.get(edge.to)||0)+1);outgoing.get(edge.from)?.push(edge.to)}
  const ranks=new Map<string,number>(details.map(item=>[item.id,0])),queue=details.filter(item=>indegree.get(item.id)===0).map(item=>item.id);
  while(queue.length){const id=queue.shift()!;for(const next of outgoing.get(id)||[]){ranks.set(next,Math.max(ranks.get(next)||0,(ranks.get(id)||0)+1));indegree.set(next,(indegree.get(next)||0)-1);if(indegree.get(next)===0)queue.push(next)}}
  const inRank=new Map<number,number>();
  const nodes:Node[]=details.map(item=>{const rank=ranks.get(item.id)||0,slot=inRank.get(rank)||0;inRank.set(rank,slot+1);return{id:item.id,position:direction==='LR'?{x:rank*270,y:slot*130}:{x:slot*240,y:rank*150},data:{label:`${item.name} · ${item.kind||item.type}`},style:{border:'1px solid #77aa96',borderRadius:12,padding:10,background:'var(--flow-node-bg,#eef8f1)',color:'var(--flow-node-text,#193b31)'}}});
  const edges:Edge[]=links.map((edge,index)=>({id:`${edge.from}-${edge.to}-${index}`,source:edge.from,target:edge.to,label:edge.kind}));
  root=createRoot(target);
  root.render(<ReactFlow nodes={nodes} edges={edges} fitView nodesDraggable onNodeClick={(_,node)=>onOpen(node.id,details.find(item=>item.id===node.id)?.type||node.id.split(':')[0])}><Background/><Controls/><MiniMap/></ReactFlow>);
}
