import React from 'react';

export function Tabs({items,value,onChange}:{items:{value:string;label:string;count?:number}[];value:string;onChange:(value:string)=>void}){
  return <div className="catalog-tabs" role="tablist">{items.map(item=><button key={item.value} type="button" role="tab" aria-selected={item.value===value} onClick={()=>onChange(item.value)}>{item.label}{item.count!==undefined&&<span className="tab-count">{item.count}</span>}</button>)}</div>;
}

export function CatalogTable({headers,children}:{headers:string[];children:React.ReactNode}){
  return <div className="table-scroll"><table className="catalog-table"><thead><tr>{headers.map(header=><th key={header} scope="col">{header}</th>)}</tr></thead><tbody>{children}</tbody></table></div>;
}

export function SkeletonRows({columns=4}:{columns?:number}){
  return <div className="table-skeleton" role="status" aria-label="Loading">{[0,1,2,3].map(row=><div key={row}>{Array.from({length:columns},(_,cell)=><span key={cell}/>)}</div>)}</div>;
}
