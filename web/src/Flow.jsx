import React from 'react';
import { createRoot } from 'react-dom/client';
import { ReactFlow, Background, Controls, MiniMap } from '@xyflow/react';
import '@xyflow/react/dist/style.css';

let root;
export function unmountFlow() {
  if (root) { root.unmount(); root = undefined; }
}

export function mountFlow(target, data, onOpen, direction="TB") {
  unmountFlow();
  const details = data.details?.length ? data.details : (data.nodes || []).map(id => ({ id, name: id, type: id.split(':')[0] }));
  const ids = new Set(details.map(item => item.id));
  const links = (data.edges || []).filter(edge => ids.has(edge.from) && ids.has(edge.to));
  const indegree = new Map(details.map(item => [item.id, 0]));
  const outgoing = new Map(details.map(item => [item.id, []]));
  for (const edge of links) { indegree.set(edge.to, indegree.get(edge.to)+1); outgoing.get(edge.from).push(edge.to); }
  const ranks = new Map(details.map(item => [item.id, 0]));
  const queue = details.filter(item => indegree.get(item.id)===0).map(item => item.id);
  while(queue.length){const id=queue.shift();for(const next of outgoing.get(id)){ranks.set(next,Math.max(ranks.get(next),ranks.get(id)+1));indegree.set(next,indegree.get(next)-1);if(indegree.get(next)===0)queue.push(next)}}
  const inRank = new Map();
  const nodes = details.map(item => {
    const rank=ranks.get(item.id),slot=inRank.get(rank)||0;inRank.set(rank,slot+1);
    return {
      id: item.id,
      position: direction==="LR"?{x:rank*270,y:slot*130}:{x:slot*240,y:rank*150},
      data: { label: `${item.name} · ${item.kind || item.type}` },
      style: { border: '1px solid #77aa96', borderRadius: 12, padding: 10, background: 'var(--flow-node-bg,#eef8f1)', color: 'var(--flow-node-text,#193b31)' },
    };
  });
  const edges = links.map((edge, index) => ({
    id: `${edge.from}-${edge.to}-${index}`, source: edge.from, target: edge.to, label: edge.kind,
  }));
  root = createRoot(target);
  root.render(<ReactFlow nodes={nodes} edges={edges} fitView nodesDraggable onNodeClick={(_, node) => onOpen(node.id, details.find(item => item.id === node.id)?.type)}><Background /><Controls /><MiniMap /></ReactFlow>);
}
