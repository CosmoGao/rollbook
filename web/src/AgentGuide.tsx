import React, { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Button } from './components/ui/button';

type API=<T>(path:string,options?:RequestInit)=>Promise<T>;
const placeholder='<ROLLBOOK_KEY>';
export function AgentGuide({api,en}:{api:API;en:boolean}){
  const [message,setMessage]=useState('');
  const version=useQuery({queryKey:['agent-version'],queryFn:()=>api<{version:string}>('/api/version')});
  const origin=location.origin,endpoint=origin+'/mcp';
  const headers={Authorization:'Bearer '+placeholder};
  const http={type:'http',url:endpoint,headers};
  const blocks=[
    {name:'Cursor · mcp.json',value:JSON.stringify({mcpServers:{rollbook:http}},null,2)},
    {name:'Claude Desktop · claude_desktop_config.json',value:JSON.stringify({mcpServers:{rollbook:{command:'rollbook',args:['mcp']}}},null,2)},
    {name:'Claude Code · .mcp.json',value:JSON.stringify({mcpServers:{rollbook:http}},null,2)},
    {name:'Codex · config.toml',value:`[mcp_servers.rollbook]\nurl = ${JSON.stringify(endpoint)}\nhttp_headers = { Authorization = ${JSON.stringify('Bearer '+placeholder)} }`},
    {name:en?'Generic HTTP MCP JSON':'通用 HTTP MCP JSON',value:JSON.stringify({mcpServers:{rollbook:http}},null,2)},
    {name:en?'Generic stdio MCP JSON':'通用 stdio MCP JSON',value:JSON.stringify({mcpServers:{rollbook:{command:'rollbook',args:['mcp']}}},null,2)},
    {name:en?'Connectivity check · tools/list':'连通性自检 · tools/list',value:`curl -sS -X POST ${JSON.stringify(endpoint)} \\\n  -H 'Content-Type: application/json' \\\n  -H 'Accept: application/json' \\\n  -H 'Authorization: Bearer ${placeholder}' \\\n  --data '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'`}
  ];
  const copy=async(value:string)=>{try{if(navigator.clipboard?.writeText)await navigator.clipboard.writeText(value);else{const input=document.createElement('textarea');input.value=value;input.style.position='fixed';input.style.opacity='0';document.body.append(input);input.select();const ok=document.execCommand('copy');input.remove();if(!ok)throw new Error('copy failed')}setMessage(en?'Copied':'已复制')}catch{setMessage(en?'Copy failed; select the text manually':'复制失败，请手动选取文本')}};
  const tag=/^v\d+\.\d+\.\d+$/.test(version.data?.version||'')?version.data!.version:'';
  const release=tag?`https://github.com/CosmoGao/rollbook/releases/tag/${tag}`:'https://github.com/CosmoGao/rollbook/releases/latest';
  return <>
    <section className="section agent-guide"><h2>{en?'MCP setup':'MCP 接入配置'}</h2><p className="muted">{en?'HTTP connects directly to this server with a Bearer key. For stdio only clients, install the CLI, run rollbook login, then start rollbook mcp. Replace the placeholder locally.':'HTTP 直连当前清册并使用 Bearer Key。仅支持 stdio 的客户端请安装 CLI，执行 rollbook login 后启动 rollbook mcp。请在本机替换占位符。'}</p><p className="muted">{en?'Current endpoint':'当前端点'}：<code>{endpoint}</code></p>{blocks.map(block=><div className="setup-block" key={block.name}><h3>{block.name}</h3><pre className="diff">{block.value}</pre><Button variant="outline" onClick={()=>copy(block.value)}>{en?'Copy':'复制'}</Button></div>)}<span role="status">{message}</span></section>
    <section className="section agent-guide"><h2>{en?'Install the Rollbook Skill':'安装 Rollbook Skill'}</h2><p className="muted">{en?'The Skill teaches agents how to search the catalog, follow references, propose changes, and keep secrets out of records. It uses your MCP connection.':'Skill 指导 Agent 查询清册、追踪引用、提交变更提议和避免记录秘密信息；它使用上面的 MCP 连接。'}</p><p><code>skills/rollbook/SKILL.md</code></p><ol><li>{en?'Download the zip and extract its rollbook folder.':'下载 zip 并解压其中的 rollbook 目录。'}</li><li>{en?'Copy that folder to the client directory below.':'将该目录复制到下方客户端目录。'}</li><li>{en?'Configure MCP above, then restart your client.':'按上面的片段配置 MCP，然后重启客户端。'}</li></ol><ul><li>Codex: <code>~/.agents/skills/rollbook/</code></li><li>Claude Code: <code>~/.claude/skills/rollbook/</code> {en?'or':'或'} <code>.claude/skills/rollbook/</code></li><li>Cursor: <code>.cursor/skills/rollbook/</code></li><li>Claude Desktop / {en?'other clients':'其他客户端'}: {en?'Use the client’s skill directory if supported; otherwise use the MCP configuration above.':'若支持 Skill，放入客户端指定的 Skill 目录；否则使用上方 MCP 配置。'}</li></ul><a className="primary" href="/api/agent/skill.zip" download="rollbook-skill.zip">{en?'Download Skill zip':'下载 Skill zip'}</a></section>
    <section className="section agent-guide"><h2>{en?'Download CLI':'下载 CLI'}</h2><p className="muted">{en?'Choose the archive for your operating system from the matching GitHub Release.':'在对应 GitHub Release 中选择适合操作系统的安装包。'}</p><p><a href={release} target="_blank" rel="noopener noreferrer">{tag?`Rollbook ${tag}`:en?'Latest release':'最新 Release'} ↗</a></p><div className="setup-block"><h3>{en?'Login and profiles':'登录与 profile'}</h3><pre className="diff">{'rollbook login\nrollbook --profile home login\nrollbook --profile home mcp'}</pre><Button variant="outline" onClick={()=>copy('rollbook login\nrollbook --profile home login\nrollbook --profile home mcp')}>{en?'Copy':'复制'}</Button></div><div className="setup-block"><h3>{en?'Environment variables':'环境变量'}</h3><pre className="diff">{`export ROLLBOOK_URL=${JSON.stringify(origin)}\nexport ROLLBOOK_KEY='${placeholder}'\nrollbook mcp`}</pre><Button variant="outline" onClick={()=>copy(`export ROLLBOOK_URL=${JSON.stringify(origin)}\nexport ROLLBOOK_KEY='${placeholder}'\nrollbook mcp`)}>{en?'Copy':'复制'}</Button></div></section>
  </>;
}
