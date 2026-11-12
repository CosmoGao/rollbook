import React, { useState } from 'react';
import { createRoot, Root } from 'react-dom/client';
import { QueryClient, QueryClientProvider, useQuery } from '@tanstack/react-query';
import * as Dialog from '@radix-ui/react-dialog';
import { Button } from './components/ui/button';
import { useTranslation } from 'react-i18next';
import { AgentGuide } from './AgentGuide';

type Key={id:string;name:string;level:'read'|'propose'|'write';issuer_name?:string;note?:string;created_at?:string;expires_at?:string;last_used_at?:string;revoked_at?:string};
type API=<T>(path:string,options?:RequestInit)=>Promise<T>;
const client=new QueryClient({defaultOptions:{queries:{retry:1,staleTime:0}}});
let root:Root|undefined;
export function unmountKeys(){if(root){root.unmount();root=undefined}}
export function mountKeys(target:Element,api:API){unmountKeys();root=createRoot(target);root.render(<QueryClientProvider client={client}><Keys api={api}/></QueryClientProvider>)}
function Modal({title,children,onClose}:{title:string;children:React.ReactNode;onClose:()=>void}){
  return <Dialog.Root open onOpenChange={open=>{if(!open)onClose()}}><Dialog.Portal><Dialog.Overlay className="modal-backdrop"/><Dialog.Content className="modal radix-modal"><Dialog.Title asChild><h2>{title}</h2></Dialog.Title>{children}</Dialog.Content></Dialog.Portal></Dialog.Root>
}
function Keys({api}:{api:API}){
  const {i18n}=useTranslation(),en=i18n.language.startsWith('en');
  const query=useQuery({queryKey:['keys'],queryFn:()=>api<Key[]>('/api/keys')});
  const [dialog,setDialog]=useState<'create'|'edit'|'revoke'|null>(null),[current,setCurrent]=useState<Key|null>(null),[token,setToken]=useState(''),[includeToken,setIncludeToken]=useState(false),[name,setName]=useState(''),[note,setNote]=useState(''),[level,setLevel]=useState<'read'|'propose'|'write'>('read'),[expiry,setExpiry]=useState(''),[error,setError]=useState(''),[busy,setBusy]=useState(false);
  const active=(query.data||[]).filter(key=>!key.revoked_at),revoked=(query.data||[]).filter(key=>key.revoked_at);
  const labels={read:en?'Read only':'只读',propose:en?'Propose':'提议',write:en?'Direct write':'直接写'};
  const time=(value?:string)=>value?new Date(value).toLocaleString(en?'en-US':'zh-CN'):'—';
  const copy=async(value:string)=>{try{if(navigator.clipboard?.writeText)await navigator.clipboard.writeText(value);else{const input=document.createElement('textarea');input.value=value;input.style.position='fixed';input.style.opacity='0';document.body.append(input);input.select();if(!document.execCommand('copy'))throw new Error('copy failed');input.remove()}}catch{setError(en?'Copy failed; copy manually':'复制失败，请手动复制')}};
  const close=()=>{setDialog(null);setCurrent(null);setToken('');setIncludeToken(false);setError('');setBusy(false)};
  const startCreate=()=>{setName('');setNote('');setLevel('read');setExpiry('');setToken('');setError('');setDialog('create')};
  const startEdit=(key:Key)=>{setCurrent(key);setName(key.name);setNote(key.note||'');setLevel(key.level);setExpiry(key.expires_at?new Date(new Date(key.expires_at).getTime()-new Date(key.expires_at).getTimezoneOffset()*60000).toISOString().slice(0,16):'');setError('');setDialog('edit')};
  const submit=async(event:React.FormEvent)=>{event.preventDefault();setBusy(true);setError('');try{
    const payload={name,level,note,expires_at:expiry?new Date(expiry).toISOString():''};
    if(dialog==='create'){const result=await api<{token:string}>('/api/keys',{method:'POST',body:JSON.stringify(payload)});setToken(result.token)}
    else if(dialog==='edit'&&current){await api('/api/keys/'+encodeURIComponent(current.id)+'/metadata',{method:'PATCH',body:JSON.stringify({name,note,expires_at:payload.expires_at})});close();client.invalidateQueries({queryKey:['keys']})}
  }catch(e){setError(String(e instanceof Error?e.message:e))}finally{setBusy(false)}};
  const changeLevel=async(key:Key,value:Key['level'])=>{if(!window.confirm(en?`Change ${key.name} to ${labels[value]}?`:`将 ${key.name} 的权限改为${labels[value]}？`))return;setError('');try{await api('/api/keys/'+encodeURIComponent(key.id)+'/level',{method:'PATCH',body:JSON.stringify({level:value})});client.invalidateQueries({queryKey:['keys']})}catch(e){setError(String(e))}};
  const revoke=async()=>{if(!current)return;setBusy(true);setError('');try{await api('/api/keys/'+encodeURIComponent(current.id)+'/revoke',{method:'POST'});close();client.invalidateQueries({queryKey:['keys']})}catch(e){setError(String(e))}finally{setBusy(false)}};
  const row=(key:Key)=><div className="row" key={key.id}><div className="row-main"><strong>{key.name}</strong><small>{labels[key.level]} · {en?'Issued by':'下发人'} {key.issuer_name||'—'}</small><small>{en?'Note':'备注'}：{key.note||'—'} · {en?'Created':'创建时间'}：{time(key.created_at)}</small><small>{en?'Expires':'到期时间'}：{time(key.expires_at)} · {en?'Last used':'最后使用时间'}：{key.last_used_at?time(key.last_used_at):(en?'Never':'从未')}</small>{key.revoked_at&&<small>{en?'Revoked':'已吊销'}：{time(key.revoked_at)}</small>}</div>{!key.revoked_at&&<div className="controls"><select className="field" aria-label={en?'Access level':'权限级别'} value={key.level} onChange={event=>changeLevel(key,event.target.value as Key['level'])}>{Object.entries(labels).map(([value,label])=><option key={value} value={value}>{label}</option>)}</select><Button variant="outline" onClick={()=>startEdit(key)}>{en?'Edit':'编辑'}</Button><Button variant="destructive" onClick={()=>{setCurrent(key);setError('');setDialog('revoke')}}>{en?'Revoke':'吊销'}</Button></div>}</div>;
  const snippetKey=includeToken?token:'<ROLLBOOK_KEY>';
  const snippets=token?{
    [en?'CLI environment':'CLI 环境变量']:`export ROLLBOOK_URL='${location.origin}'\nexport ROLLBOOK_KEY='${snippetKey}'`,
    'Claude Code（.mcp.json）':JSON.stringify({mcpServers:{rollbook:{type:'http',url:location.origin+'/mcp',headers:{Authorization:'Bearer '+snippetKey}}}},null,2),
    'Codex（config.toml）':`[mcp_servers.rollbook]\nurl = ${JSON.stringify(location.origin+'/mcp')}\nhttp_headers = { Authorization = ${JSON.stringify('Bearer '+snippetKey)} }`,
    'Cursor（mcp.json）':JSON.stringify({mcpServers:{rollbook:{url:location.origin+'/mcp',headers:{Authorization:'Bearer '+snippetKey}}}},null,2),
    [en?'stdio client (mcp.json)':'stdio 客户端（mcp.json）']:JSON.stringify({mcpServers:{rollbook:{command:'rollbook',args:['mcp']}}},null,2)
  }:{};
  return <><div className="hero"><p className="eyebrow">ROLLBOOK / {en?'Agent access':'Agent 接入'}</p><h1>{en?'Key management':'Key 管理'}</h1><p className="muted">{en?'Issue, update or revoke access keys for agents.':'下发、调整或吊销 agent 使用的访问 key。'}</p></div><Button onClick={startCreate}>{en?'Create key':'创建 Key'}</Button>{error&&!dialog&&<p className="error" role="alert">{error}</p>}
    <section className="section" style={{marginTop:22}}><h2>{en?'Issued keys':'已下发的 Key'}</h2>{query.isLoading?<p className="notice">{en?'Loading…':'正在加载…'}</p>:query.isError?<p className="error">{en?'Failed to load':'加载失败'} <Button variant="outline" onClick={()=>query.refetch()}>{en?'Retry':'重试'}</Button></p>:active.length?active.map(row):<div className="empty">{en?'No records yet':'暂无记录'}</div>}</section>
    {!!revoked.length&&<details className="section"><summary>{en?'Show revoked keys':'查看已吊销的 Key'} ({revoked.length})</summary>{revoked.map(row)}</details>}
    <AgentGuide api={api} en={en}/>
    {dialog&&<Modal title={dialog==='create'?(token?(en?'Key created':'Key 已创建'):(en?'Create key':'创建 Key')):dialog==='edit'?(en?'Edit key':'编辑 Key'):(en?'Revoke key':'吊销 Key')} onClose={close}>
      {dialog==='revoke'?<><p className="muted">{en?'Revoking this key takes effect immediately.':'吊销后立即失效。'} {current?.name}</p>{error&&<p className="error">{error}</p>}<div className="modal-actions"><Button variant="outline" onClick={close}>{en?'Cancel':'取消'}</Button><Button variant="destructive" disabled={busy} onClick={revoke}>{en?'Confirm revocation':'确认吊销'}</Button></div></>:token?<><p className="muted">{en?'Copy now. You cannot see this key again after closing.':'请立即复制。关闭后将无法再次查看。'}</p><div className="secret">{token}</div><label className="check-row"><input type="checkbox" checked={includeToken} onChange={event=>setIncludeToken(event.target.checked)}/>{en?'Temporarily include this key in the snippets below':'临时把此 Key 填入下方配置片段'}</label><p className="muted">{en?'Install the Skill at skills/rollbook/SKILL.md. MCP uses Streamable HTTP. Keep snippets containing keys in protected local files.':'将 Skill 安装到客户端的 skills/rollbook/SKILL.md；MCP 使用 Streamable HTTP。含 Key 的配置片段只存于本机受保护的配置中。'}</p>{Object.entries(snippets).map(([title,value])=><div className="setup-block" key={title}><h3>{title}</h3><pre className="diff">{value}</pre><Button variant="outline" onClick={()=>copy(value)}>{en?'Copy':'复制'}</Button></div>)}{error&&<p className="error">{error}</p>}<Button onClick={()=>{close();client.invalidateQueries({queryKey:['keys']})}}>{en?'Done':'完成'}</Button></>:<form onSubmit={submit}><label>{en?'Name':'名称'}<input value={name} onChange={event=>setName(event.target.value)} required /></label>{dialog==='create'&&<label>{en?'Access level':'权限级别'}<select value={level} onChange={event=>setLevel(event.target.value as Key['level'])}>{Object.entries(labels).map(([value,label])=><option value={value} key={value}>{label}</option>)}</select></label>}<label>{en?'Expiration (optional)':'到期时间（可选）'}<input type="datetime-local" value={expiry} onChange={event=>setExpiry(event.target.value)}/></label><label>{en?'Note (optional)':'备注（可选）'}<textarea value={note} onChange={event=>setNote(event.target.value)}/></label>{error&&<p className="error">{error}</p>}<div className="modal-actions"><Button type="button" variant="outline" onClick={close}>{en?'Cancel':'取消'}</Button><Button type="submit" disabled={busy}>{dialog==='create'?(en?'Create':'创建'):(en?'Save changes':'保存修改')}</Button></div></form>}
    </Modal>}
  </>
}
