import React from 'react';

export function Tabs({items,value,onChange}:{items:{value:string;label:string;count?:number}[];value:string;onChange:(value:string)=>void}){
  const move=(event:React.KeyboardEvent<HTMLButtonElement>,index:number)=>{const next=event.key==='ArrowRight'?(index+1)%items.length:event.key==='ArrowLeft'?(index+items.length-1)%items.length:event.key==='Home'?0:event.key==='End'?items.length-1:-1;if(next<0)return;event.preventDefault();onChange(items[next].value);const buttons=event.currentTarget.closest('[role=tablist]')?.querySelectorAll<HTMLButtonElement>('[role=tab]');buttons?.[next]?.focus()};
  return <div className="catalog-tabs" role="tablist" aria-label={document.documentElement.lang.startsWith('en')?'Categories':'分类'}>{items.map((item,index)=><button key={item.value} type="button" role="tab" tabIndex={item.value===value?0:-1} aria-selected={item.value===value} onClick={()=>onChange(item.value)} onKeyDown={event=>move(event,index)}>{item.label}{item.count!==undefined&&<span className="tab-count">{item.count}</span>}</button>)}</div>;
}

export function CatalogTable({headers,children}:{headers:string[];children:React.ReactNode}){
  return <div className="table-scroll"><table className="catalog-table" aria-label={document.documentElement.lang.startsWith('en')?'Catalog':'清册列表'}><thead><tr>{headers.map(header=><th key={header} scope="col">{header}</th>)}</tr></thead><tbody>{children}</tbody></table></div>;
}

export function SkeletonRows({columns=4}:{columns?:number}){
  return <div className="table-skeleton" role="status" aria-label={document.documentElement.lang.startsWith('en')?'Loading':'正在加载'} aria-busy="true">{[0,1,2,3].map(row=><div key={row}>{Array.from({length:columns},(_,cell)=><span key={cell}/>)}</div>)}</div>;
}
