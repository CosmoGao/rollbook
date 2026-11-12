import React from 'react';
import { createRoot } from 'react-dom/client';
import { ReactFlow, Background, Controls, MiniMap } from '@xyflow/react';
import '@xyflow/react/dist/style.css';

let root;
export function unmountFlow() {
  if (root) { root.unmount(); root = undefined; }
}

export function mountFlow(target, data, onOpen) {
  unmountFlow();
  const details = data.details?.length ? data.details : (data.nodes || []).map(id => ({ id, name: id, type: id.split(':')[0] }));
  const nodes = details.map((item, index) => ({
    id: item.id,
    position: { x: (index % 4) * 240, y: Math.floor(index / 4) * 150 },
    data: { label: `${item.name} · ${item.kind || item.type}` },
    style: { border: '1px solid #77aa96', borderRadius: 12, padding: 10, background: '#eef8f1', color: '#193b31' },
  }));
  const ids = new Set(nodes.map(node => node.id));
  const edges = (data.edges || []).filter(edge => ids.has(edge.from) && ids.has(edge.to)).map((edge, index) => ({
    id: `${edge.from}-${edge.to}-${index}`, source: edge.from, target: edge.to, label: edge.kind,
  }));
  root = createRoot(target);
  root.render(<ReactFlow nodes={nodes} edges={edges} fitView nodesDraggable onNodeClick={(_, node) => onOpen(node.id, details.find(item => item.id === node.id)?.type)}><Background /><Controls /><MiniMap /></ReactFlow>);
}
