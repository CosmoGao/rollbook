import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { mountFlow, unmountFlow } from './Flow.jsx';
import { mountOverview, unmountOverview } from './Overview';
import { mountAssets, unmountAssets } from './Assets';
import { mountSettings, unmountSettings } from './Settings';
import { mountSearch, unmountSearch } from './Search';
import { mountPending, unmountPending } from './Pending';
import { mountKeys, unmountKeys } from './Keys';
import { mountBackup, unmountBackup } from './Backup';
import { mountAudit, unmountAudit } from './Audit';
import { mountTopology, unmountTopology } from './Topology';
import { openAssetForm, formFields } from './AssetForm';
// 界面文案以现有中文为键；这里不更改 API 取值。
const translations = {
'备注':'Note','创建时间':'Created','到期时间':'Expires','最后使用时间':'Last used','查看已吊销的 Key':'Show revoked keys','编辑 Key':'Edit key',
'搜索 IP、服务…':'Search IP or service…',
'最近变更':'Recent changes','共享上下文':'Shared context','按 Key、对象类型和时间查看审计记录。':'Filter audit records by key, object type, and time.','查看 agent 与用户写入的上下文。':'View context written by agents and users.','Key ID 或名称':'Key ID or name','全部类型':'All types','开始日期':'Start date','结束日期':'End date','筛选':'Filter','显示已归档':'Show archived','基本字段':'Core fields','反向引用':'Referenced by',
'网段占用':'Network occupancy','已用地址':'Used addresses','空闲地址范围':'Available address ranges','下一个可分配 IP':'Next assignable IP','无空闲地址':'No available address','网关':'Gateway','冲突':'Conflict',
'拓扑与访问链':'Topology and access paths','分层拓扑':'Layered topology','访问链':'Access path',
'[停用]':'[Inactive]','中文':'中文','无主机':'No host','变更':'Changes','核实':'Verify','吊销':'Revoke','完成':'Done','导出':'Export','低':'Low','中':'Medium','高':'High','光纤宽带':'Fiber broadband','专线':'Dedicated line','其他':'Other','DHCP保留':'DHCP reservation','无':'None','暂无记录，可从这里新建':'No records yet; create one here','吊销 ${k.name} 后，它会立即失效。':'Revoking this key takes effect immediately.','清册导出用于迁移资产；完整备份含账号和 Key 哈希，仅网页用户可下载。恢复会清除所有登录会话。':'Catalog exports are for migration. Full backups include account and key hashes and are available only to web users. Restoration invalidates all sessions.','请选择与 YAML kind 一致的导入方式；旧备份请选择恢复':'Choose an import mode matching the YAML kind. Choose restore for older backups.','第二次确认：删除 ':'Confirm again: delete ','首次引导检查失败':'First-run check failed','将 Skill 安装到客户端的 skills/rollbook/SKILL.md；MCP 使用 Streamable HTTP。配置片段含 Key，请只放在本机受保护的配置中。':'Install the Skill at skills/rollbook/SKILL.md. MCP uses Streamable HTTP. Keep configuration snippets containing keys in protected local files.','stdio 客户端（mcp.json）':'stdio client (mcp.json)',
'ROLLBOOK / 清册':'ROLLBOOK / Catalog','清册':'Catalog','服务与资产清册':'Service and asset catalog','登录清册':'Sign in to Rollbook','用管理员账号查看服务与资产。':'Sign in with your administrator account to view services and assets.','当前为 HTTP 模式：口令和 Key 会在网络中明文传输，仅限可信内网使用。':'HTTP mode: passwords and keys travel unencrypted. Use only on a trusted network.','当前为 HTTP 模式':'HTTP mode','账号':'Username','密码':'Password','登录':'Sign in','工作台':'Workspace','全局搜索':'Search','资产维护':'Manage assets','站点':'Sites','主机':'Hosts','网段':'Networks','关注':'Watch','有新版本':'New versions','待确认':'Pending review','待核实':'Needs verification','管理':'Manage','Key 管理':'Key management','备份与恢复':'Backup and restore','改密码':'Change password','修改密码':'Change password','退出所有设备':'Sign out everywhere','退出登录':'Sign out','退出':'Sign out','内网工作台':'Private workspace','资产详情':'Asset details',
'找到你需要的服务':'Find a service or asset','搜索 IP、域名、端口、服务名称或别名。':'Search by IP, domain, port, service name, or alias.','例如 126、Overleaf、香河':'For example 126, Overleaf, or a site name','搜索':'Search','试试搜索 “126” 或 “香河”':'Try searching for “126” or a site name','正在搜索…':'Searching…','正在加载…':'Loading…','没有匹配的记录':'No matching records','暂无记录':'No records yet','相关主机、服务与入口':'Related hosts, services, and endpoints','找到 ':'Found ',' 条结果':' results','按资产浏览清册。':'Browse the catalog by asset.','查看接入、网段与路由设备。':'View uplinks, networks, and routers.','查看':'View','停用':'Inactive','在用':'Active','计划中':'Planned',
'站点 · ':'Site · ','服务 · ':'Service · ','网络接入 · ':'Uplink · ','网关 ':'Gateway ','路由器':'Routers','IP 地址':'IP addresses','服务信息':'Service information','资产信息':'Asset information','访问入口':'Endpoints','凭据引用':'Credential references','笔记':'Notes','变更记录':'Change history','两层拓扑':'Two-level topology','依赖':'Dependencies','版本':'Versions','服务':'Services','端口':'Ports','宿主':'Host','宿主IP':'Host IP','来源 ':'Source ','原始 tag ':'Original tag ','检查失败':'Check failed','固定版本':'Pinned version','当前 ':'Current ','最新 ':'Latest ','上次核实 ':'Last verified ','从未核实':'Never verified','经由 ':'Via ',
'有新版本与待确认':'New versions and pending review','每天自动检查。无法可靠比较的版本会标记为待人工确认。':'Checked daily. Versions that cannot be compared reliably are marked for manual review.','立即检查':'Check now','没有落后或待确认的服务':'No outdated or pending services','已检查 ':'Checked ',' 项，失败 ':' items, failed ','待确认':'Pending review','按字段核对原值、新值与当前版本；批量操作逐条反馈结果。':'Compare old and new values with the current revision. Batch actions report each result.','请先勾选提议':'Select proposals first','批量批准':'Approve selected','批量驳回':'Reject selected','批准':'Approve','驳回':'Reject','字段':'Field','原值':'Old value','新值':'New value','冲突字段：':'Conflicting fields: ','基于 rev ':'Based on rev ','当前 rev ':'Current rev ','成功':'Success','失败 ':'Failed ','按主机和服务查看；主机可连同 IP 与端口核实。':'Review hosts and services. You can verify a host with its IPs and ports.','批量核实所选':'Verify selected','连同该主机的 IP 和端口一起核实':'Also verify this host’s IPs and ports','核实主机':'Verify host','核实原因':'Reason for verification','已核实':'Verified',
'按类型查看、新建和维护清册。':'View, create, and maintain catalog assets by type.','新建':'New','编辑':'Edit','保存修改':'Save changes','创建':'Create','取消':'Cancel','名称':'Name','类型':'Type','请选择':'Select…','ID（可修改）':'ID (editable)','操作原因 *':'Reason *','操作原因':'Reason','例如：登记新主机、更新服务配置':'For example: register a host or update a service','请输入有效 JSON':'Enter valid JSON','标记核实':'Mark verified','加笔记':'Add note','添加笔记':'Add note','内容':'Content','原因':'Reason','保存笔记':'Save note','运维笔记':'Operations note','故障':'Incident','决策':'Decision','待办':'Todo','删除原因':'Reason for deletion','继续删除':'Continue deletion','删除后无法撤销。请确认两次。被引用时会列出引用方。':'Deletion cannot be undone. Confirm twice. Referencing assets will be listed.','停用主机':'Deactivate host','恢复主机':'Restore host','停用主机 · 预览影响':'Deactivate host · impact preview','连带停用服务':'Also deactivate services','释放 IP':'Release IPs','标记入口不可用':'Mark endpoints unavailable','停用原因':'Reason for deactivation','确认停用':'Confirm deactivation','恢复原因':'Reason for restoration','恢复':'Restore','没有可恢复的停用记录':'No deactivation record to restore','以下对象后来被修改，已跳过恢复：':'Skipped assets changed since deactivation: ',
'Key 管理':'Key management','下发、调整或吊销 agent 使用的访问 key。':'Issue, adjust, or revoke agent access keys.','创建 Key':'Create key','已下发的 Key':'Issued keys','只读':'Read only','提议':'Propose','直接写':'Direct write','权限级别':'Access level','备注（可选）':'Note (optional)','到期时间（可选）':'Expiration (optional)','最近使用 ':'Last used ','从未':'Never','下发人 ':'Issued by ','吊销 Key':'Revoke key','确认吊销':'Confirm revocation','已吊销':'Revoked','已创建':'Created','请立即复制。关闭后将无法再次查看。':'Copy now. You cannot see this key again after closing.','复制':'Copy','已复制':'Copied','复制失败，请手动复制':'Copy failed; copy manually','环境变量':'Environment variables',
'服务器备份':'Server backups','日备份':'Daily backup','周备份':'Weekly backup','下载':'Download','选择恢复':'Select restore','导入审批':'Import approvals','合并导入':'Merge import','完整恢复':'Full restore','清册恢复':'Catalog restore','新增':'Added','修改':'Modified','删除':'Deleted','将丢失的变更：':'Changes that will be lost: ','批准合并':'Approve merge','批准恢复':'Approve restore','已提交待审批：':'Submitted for approval: ','提交这份备份供审批恢复？':'Submit this backup for restore approval?','确认合并这些对象？审批时将重新校验 rev 和关联关系。':'Merge these assets? Revisions and references will be checked again.','确认整库恢复？恢复前会保存当前库备份，所有登录会话将失效。若数据有变化，系统会要求重新提交预览。':'Restore the entire database? A backup will be saved and all sessions will expire. If data has changed, submit a new preview.','下载清册 YAML':'Download catalog YAML','下载完整备份 YAML':'Download full backup YAML','合并导入：只新增或更新，保留其他资产和历史':'Merge: add or update, preserving other assets and history','完整恢复：替换清册、账号与 Key':'Full restore: replace catalog, accounts, and keys','清册恢复：替换当前清册':'Catalog restore: replace current catalog','下载模板':'Download template','提交待确认导入':'Submit import for review','预览并提交审批':'Preview and submit for approval','校验并提交':'Validate and submit','与所选恢复方式不一致':'Does not match the selected restore mode',
'欢迎使用清册':'Welcome to Rollbook','稍后再说':'Maybe later','建立第一个站点':'Create your first site','导入虚构示例数据':'Import fictional sample data','清册还是空的。可以建立第一个站点，或导入明确标记的虚构示例数据。示例使用文档专用 IP 和 .invalid 域名，不代表真实资产。':'The catalog is empty. Create your first site or import fictional examples using documentation IPs and .invalid domains.','修改并重新登录':'Change password and sign in again','当前密码':'Current password','新密码':'New password','至少 8 个字符；大写、小写、数字、符号至少三类；不得包含用户名或使用常见弱口令；最多 72 字节。':'At least 8 characters; use three of uppercase, lowercase, digits and symbols; do not include your username or use a common password; at most 72 bytes.','退出所有设备，包括当前会话？':'Sign out of all devices, including this one?',
'Webhook 提醒':'Webhook alerts','启用 Webhook':'Enable webhook','机器人 Webhook URL':'Bot webhook URL','机器人类型':'Bot type','飞书':'Feishu','企业微信':'WeCom','保存提醒设置':'Save alert settings','已保存':'Saved','网络接入':'Uplinks','网络设备':'Network device','物理机':'Physical host','虚拟机':'Virtual machine','容器':'Container','内网直连':'Direct access','反向代理':'Reverse proxy','导航页卡片':'Navigation card','人工':'Manual','自动发现':'Auto-discovered','无或CGNAT':'None or CGNAT','动态公网':'Dynamic public IP','固定公网':'Static public IP','静态':'Static','动态':'Dynamic','光猫':'Modem','主路由':'Main router','旁路由':'Secondary router','交换机':'Switch','房间':'Room','机柜':'Rack','桌面':'Desk','地址':'Address','入口':'Endpoint','网段 ':'Network ','主机 ':'Host ','服务 ':'Service ','站点 ':'Site ','引用方：':'Referenced by: ','期望：':'Expected: ','请求失败 ':'Request failed ',
};
for(const [cn,en] of Object.entries({站点:'site',网络接入:'uplink',主机:'host',网段:'network',服务:'service',端口:'port',访问入口:'endpoint',笔记:'note',凭据引用:'credential reference',版本:'version',依赖:'dependency','IP 地址':'IP address'})){
 translations['新建'+cn]='New '+en;
 translations['编辑'+cn]='Edit '+en;
}
const localeKey='rollbook.language';
let language=localStorage.getItem(localeKey)||((navigator.language||'').toLowerCase().startsWith('en')?'en':'zh-CN');
function t(value){
 const text=String(value??'');
 if(language!=='en')return text;
 if(Object.hasOwn(translations,text))return translations[text];
 const trimmed=text.trim();
 return trimmed!==text && Object.hasOwn(translations,trimmed) ? text.replace(trimmed,translations[trimmed]) : text;
}
// 业务枚举的显示文案与导航文案分开；选项 value 仍为中文规范值。
const enumTranslations={
 kind:{站点:'Site',房间:'Room',机柜:'Rack',桌面:'Desk',物理机:'Physical host',NAS:'NAS',虚拟机:'Virtual machine',LXC:'LXC',容器:'Container',网络设备:'Network device',内网直连:'Direct access',反向代理:'Reverse proxy','Cloudflare Tunnel':'Cloudflare Tunnel',导航页卡片:'Navigation card',运维笔记:'Operations note',故障:'Incident',决策:'Decision',待办:'Todo'},
 status:{在用:'Active',停用:'Inactive',计划中:'Planned'}
};
function enumLabel(field,value){
 if(language!=='en')return String(value??'');
 return enumTranslations[field]?.[value]??t(value);
}
// 静态页面文本只在初始化时翻译；动态资产数据不会进入此路径。
function translateStatic(node){
 if(node.nodeType===3){node.nodeValue=t(node.nodeValue);return}
 if(node.nodeType!==1||['SCRIPT','STYLE'].includes(node.tagName)||['user-name','build-version','login-error'].includes(node.id))return;
 for(const attr of ['placeholder','title','aria-label'])if(node.hasAttribute(attr))node.setAttribute(attr,t(node.getAttribute(attr)));
 for(const child of node.childNodes)translateStatic(child);
}
const nativeConfirm=window.confirm.bind(window),nativePrompt=window.prompt.bind(window),nativeAlert=window.alert.bind(window);
window.confirm=(message)=>nativeConfirm(t(message));window.prompt=(message,defaultValue)=>nativePrompt(t(message),defaultValue);window.alert=(message)=>nativeAlert(t(message));
function setLanguage(value){language=value;localStorage.setItem(localeKey,value);location.reload()}
document.addEventListener('DOMContentLoaded',()=>{
 document.documentElement.lang=language;
 const select=document.querySelector('#language');select.value=language;select.onchange=()=>setLanguage(select.value);
 document.documentElement.lang=language;
for(const selector of ['#login','.sidebar','.topbar'])translateStatic(document.querySelector(selector));
});

const $=s=>document.querySelector(s);
const root=$('#content');
const labels={overview:'总览',topology:'拓扑',pendingHub:'待处理',agent:'Agent 接入',settings:'设置',search:'全局搜索',assets:'资产维护',sites:'站点',hosts:'主机',networks:'网段',outdated:'有新版本',pending:'待确认',changes:'最近变更',contexts:'共享上下文',unverified:'待核实',keys:'Key 管理',backup:'备份与恢复'};
const types={location:'站点',uplink:'网络接入',host:'主机',network:'网段',ip:'IP 地址',service:'服务',port:'端口',endpoint:'访问入口',version:'版本',credential:'凭据引用',note:'笔记',change:'变更',dependency:'依赖'};
let current='search';
const literalMarker=Symbol('literal');
function literal(value){return {[literalMarker]:String(value??'')}}
function el(tag,cls,text){const n=document.createElement(tag);if(cls)n.className=cls;if(text!=null)n.textContent=text?.[literalMarker]??t(text);return n}
function add(parent,...nodes){nodes.forEach(n=>parent.append(n));return parent}
function clear(parent){parent.replaceChildren();return parent}
function cookie(name){return document.cookie.split('; ').find(x=>x.startsWith(name+'='))?.slice(name.length+1)||''}
async function api(path,options={}){const opts={...options,credentials:'same-origin',headers:{Accept:'application/json',...(options.body?{'Content-Type':'application/json'}:{}),'X-Rollbook-Language':language,...(options.method&&options.method!=='GET'?{'X-CSRF-Token':cookie('rollbook_csrf')}:{}),...options.headers}};const r=await fetch(path,opts);const data=await r.json().catch(()=>({}));if(!r.ok){if(r.status===401&&path!=='/api/auth/login')showLogin();const fields=(data.fields||[]).map(f=>`${f.field}: ${f.reason}${f.expected?' ('+t('期望：')+f.expected+')':''}${f.references?.length?'; '+t('引用方：')+f.references.join(', '):''}`);throw new Error(t(fields.join('\n')||data.message||`请求失败 (${r.status})`))}return data}
function showLogin(){ $('#app').hidden=true;$('#login').hidden=false }
function showApp(login){$('#login').hidden=true;$('#app').hidden=false;$('#user-name').textContent=login;$('#mobile-user-name').textContent=login;window.dispatchEvent(new Event('rollbook:session'))}
async function copyText(value,button){try{if(navigator.clipboard?.writeText)await navigator.clipboard.writeText(value);else{const input=el('textarea');input.value=value;input.style.position='fixed';input.style.opacity='0';document.body.append(input);input.select();const copied=document.execCommand('copy');input.remove();if(!copied)throw Error('copy failed')}button.textContent=t('已复制')}catch{button.textContent=t('复制失败，请手动复制')}}
function copyButton(value){const button=el('button','ghost copy-button','复制');button.type='button';button.setAttribute('aria-label',t('复制')+' '+value);button.onclick=e=>{e.stopPropagation();copyText(value,button)};return button}
function heading(title,desc){return add(el('div','hero'),el('p','eyebrow',literal('ROLLBOOK / '+t(labels[current]||'资产详情'))),el('h1','',title),el('p','muted',desc))}
function section(title){const s=el('section','section');add(s,add(el('div','section-header'),el('h2','',title)));return s}
function message(text,kind='notice'){return el('div',kind,text)}
function empty(text='暂无记录'){return el('div','empty',text)}
function fmt(v){if(v==null||v==='')return '—';if(typeof v==='object')return JSON.stringify(v);return String(v)}
function safeLink(value){try{const u=new URL(String(value));return ['http:','https:'].includes(u.protocol)&&u.host?u.href:null}catch{return null}}
function fieldRows(o,exclude=[],type=''){const box=el('div');for(const [key,val] of Object.entries(o||{})){if(exclude.includes(key)||val==null||val==='')continue;const text=fmt(val);let value=el('span','',literal((key==='kind'?['location','host','endpoint','note'].includes(type):['status','source','confidence','assign','net_role','public_ipv4'].includes(key))?enumLabel(key,text):text));if(['url','mgmt_url','docs_url','latest_url'].includes(key)){const href=safeLink(val);if(href){value=el('a','',literal(text));value.href=href;value.target='_blank';value.rel='noopener noreferrer'}}const content=el('span','field-value');content.append(value);if(['address','cidr','url','mgmt_url','docs_url','latest_url','宿主IP'].includes(key)&&text!=='—')content.append(copyButton(text));add(box,add(el('div','kv'),el('span','',key.replace(/_json$/,'').replaceAll('_',' ')),content))}return box}
function badge(type){return el('span','tag',types[type]||type)}
function card(o,sub=''){const c=el('article','card');c.tabIndex=0;add(c,badge(o.type),el('div','card-title',literal(o.name||o.id)),el('div','muted',literal(`${o.status==='停用'?t('[停用]')+' · ':''}${sub||o.id}`)));c.addEventListener('click',()=>openObject(o.id,o.type));c.addEventListener('keydown',e=>{if(e.key==='Enter')openObject(o.id,o.type)});return c}
function row(o,sub=''){const r=el('div','row');const b=el('button','ghost','查看');b.onclick=()=>openObject(o.id,o.type);const actions=el('div','row-actions');if(o.type==='ip'&&o.fields?.address)actions.append(copyButton(o.fields.address));actions.append(b);add(r,add(el('div','row-main'),el('strong','',literal(o.name||o.id)),el('small','',literal(`${o.status==='停用'?t('[停用]')+' · ':''}${sub||o.id}`))),actions);return r}
async function getObject(id){return api('/api/assets/'+encodeURIComponent(id))}
async function objects(ids){return Promise.all((ids||[]).map(getObject))}
function error(e){clear(root).append(message(e.message,'error'))}
async function navigate(view){current=view;const menuView=['pending','unverified','outdated'].includes(view)?'pendingHub':view==='services'?'assets':view;$('#crumb').textContent=t(labels[menuView]||'资产详情');document.querySelectorAll('#nav button[data-view]').forEach(b=>b.classList.toggle('active',b.dataset.view===menuView));clear(root);try{switch(view){case 'overview':await renderOverview();break;case 'services':await renderAssets('service');break;case 'topology':await renderTopologyPage();break;case 'pendingHub':await renderPendingHub('pending');break;case 'agent':await renderKeys();break;case 'settings':renderSettings();break;case 'search':renderSearch();break;case 'assets':await renderAssets();break;case 'sites':case 'hosts':case 'networks':await renderBrowse(view);break;case 'outdated':case 'unverified':case 'pending':await renderPendingHub(view);break;case 'changes':await renderRecentChanges();break;case 'contexts':await renderContexts();break;case 'keys':await renderKeys();break;case 'backup':await renderBackup();break}}catch(e){error(e)}}
async function renderBrowse(view){const type={sites:'location',hosts:'host',networks:'network'}[view];add(root,heading(labels[view],view==='sites'?'查看接入、网段与路由设备。':'按资产浏览清册。'));let list=await api('/api/browse?type='+type);if(view==='sites')list=list.filter(o=>o.fields?.kind==='站点');if(!list.length){root.append(empty());return}if(view==='hosts'){await renderHostTree(list);return}const grid=el('div','grid');root.append(grid);for(const o of list)grid.append(card(o,view==='networks'?fmt(o.fields?.cidr):o.id))}
async function renderHostTree(hosts){const [services,ips]=await Promise.all([api('/api/browse?type=service'),api('/api/browse?type=ip')]);const hostIds=new Set(hosts.map(h=>h.id));const children=new Map(),servicesByHost=new Map(),ipsByHost=new Map();for(const h of hosts){const parent=h.fields?.parent_host_id;if(parent&&hostIds.has(parent)){if(!children.has(parent))children.set(parent,[]);children.get(parent).push(h)}}for(const s of services){const id=s.fields?.host_id;if(!servicesByHost.has(id))servicesByHost.set(id,[]);servicesByHost.get(id).push(s)}for(const ip of ips){const id=ip.fields?.host_id;if(!ipsByHost.has(id))ipsByHost.set(id,[]);ipsByHost.get(id).push(ip.fields?.address)}const container=el('div','host-tree');root.append(container);const visited=new Set();const rank=h=>({物理机:0,NAS:0,虚拟机:1,LXC:2,容器:2,网络设备:3})[h.fields?.kind]??4;const node=(o,depth=0)=>{if(visited.has(o.id))return null;visited.add(o.id);const ipHost=o.type==='service'?o.fields?.host_id:o.id;const ownIPs=(ipsByHost.get(ipHost)||[]).join(', ')||'—';const kids=[...(children.get(o.id)||[]).sort((a,b)=>rank(a)-rank(b)||a.name.localeCompare(b.name)),...(servicesByHost.get(o.id)||[])];const details=el('details','tree-node');details.open=depth<2;const summary=el('summary');const title=el('span','tree-title',literal(o.name));title.onclick=e=>{e.preventDefault();e.stopPropagation();openObject(o.id,o.type)};const meta=el('span','tree-meta',literal(`${o.type==='host'?enumLabel('kind',o.fields?.kind):t('服务')} · IP ${ownIPs} · ${enumLabel('status',o.status)}`));const link=el('button','ghost','查看');link.type='button';link.onclick=e=>{e.preventDefault();e.stopPropagation();openObject(o.id,o.type)};add(summary,title,meta,link);details.append(summary);const body=el('div','tree-children');for(const child of kids){const item=node(child,depth+1);if(item)body.append(item)}details.append(body);return details};for(const h of hosts.filter(h=>!h.fields?.parent_host_id||!hostIds.has(h.fields.parent_host_id)).sort((a,b)=>rank(a)-rank(b)||a.name.localeCompare(b.name))){const item=node(h);if(item)container.append(item)}for(const h of hosts)if(!visited.has(h.id)){const item=node(h);if(item)container.append(item)}}
function renderOccupancy(inv){const o=inv.occupancy;if(!o)return;const summary=section('网段占用');add(summary,el('p','muted',literal(`${o.cidr} · ${t('网关')} ${o.gateway||'—'} · DHCP ${o.dhcp_range||'—'}`)),el('p','',literal(`${t('下一个可分配 IP')}：${o.next_available||t('无空闲地址')}`)));if(o.next_available)summary.append(copyButton(o.next_available));root.append(summary);const used=section('已用地址');if(!o.used?.length)used.append(empty());for(const entry of o.used||[]){const r=el('div','row occupancy-row');add(r,el('strong','',literal(entry.address)),copyButton(entry.address),el('span','muted',literal(`${entry.host_id} · ${entry.ip_id}`)));if(entry.gateway)r.append(el('span','tag warn','网关'));if(entry.dhcp)r.append(el('span','tag warn','DHCP'));if(entry.conflict)r.append(el('span','tag conflict','冲突'));used.append(r)}root.append(used);const free=section('空闲地址范围');if(!o.free?.length)free.append(empty('无空闲地址'));for(const range of o.free||[])free.append(el('div','row',literal(range.start===range.end?range.start:`${range.start} – ${range.end}`)));root.append(free)}
async function openObject(id,type){current='detail';$('#crumb').textContent=t('资产详情');document.querySelectorAll('#nav button').forEach(b=>b.classList.remove('active'));clear(root).append(message('正在加载…'));try{if(type==='service'||id.startsWith('service:'))return renderService(id);if(type==='location')return renderSite(id);if(type==='host'||type==='network')return renderInventory(id,type);const o=await getObject(id);if(o.type==='service')return renderService(id);if(o.type==='location')return renderSite(id);if(o.type==='host'||o.type==='network')return renderInventory(id,o.type);await renderBasic(o)}catch(e){error(e)}}
function renderBasic(o){clear(root);add(root,heading(literal(o.name||o.id),literal(o.id)));const s=section(types[o.type]||o.type);s.append(fieldRows({...o, ...o.fields},['fields','aliases','tags'],o.type));root.append(s)}
function listSection(title,arr,formatter){const s=section(title);if(!arr?.length){s.append(empty());return s}for(const o of arr){if(typeof o==='string'){const r=el('div','row');const b=el('button','ghost','查看');b.onclick=()=>openObject(o);add(r,el('span','',literal(o)),b);s.append(r)}else s.append(row(o,formatter?formatter(o):o.id))}return s}
async function renderSite(id){const site=await api('/api/sites/'+encodeURIComponent(id));const ids=[...(site.uplinks||[]),...(site.networks||[]),...(site.routers||[]),...(site.hosts||[])];const got=await objects([...new Set(ids)]);const byId=Object.fromEntries(got.map(o=>[o.id,o]));const pick=arr=>(arr||[]).map(x=>byId[x]).filter(Boolean);clear(root);add(root,heading(literal(site.name||site.Name),literal(`${t('站点 · ')}${site.id}`)));const uplinks=pick(site.uplinks||site.Uplinks);const networks=pick(site.networks||site.Networks);const routers=pick(site.routers||site.Routers);const hosts=pick(site.hosts||site.Hosts);for(const u of uplinks){const s=section(literal(t('网络接入 · ')+u.name));s.append(fieldRows(u.fields,[],u.type));root.append(s)}root.append(listSection('网段',networks,o=>`${fmt(o.fields?.cidr)} · 网关 ${fmt(o.fields?.gateway)}`));root.append(listSection('路由器',routers,o=>fmt(o.fields?.net_role)));root.append(listSection('主机',hosts,o=>fmt(o.fields?.kind)))}
async function renderInventory(id,type){const [o,inv]=await Promise.all([getObject(id),api(`/api/${type==='host'?'hosts':'networks'}/${encodeURIComponent(id)}/inventory`)]);const ids=[...(inv.hosts||[]),...(inv.services||[]),...(inv.ips||[]),...(inv.router_id?[inv.router_id]:[])];const got=await objects([...new Set(ids)]);const byId=Object.fromEntries(got.map(x=>[x.id,x]));const pick=arr=>(arr||[]).map(x=>byId[x]).filter(Boolean);clear(root);add(root,heading(literal(o.name),literal(o.id)));const s=section('资产信息');s.append(fieldRows(o.fields,[],o.type));root.append(s);if(type==='network'&&inv.router_id)root.append(listSection('路由器',pick([inv.router_id])));if(type==='network')renderOccupancy(inv);else root.append(listSection('IP 地址',pick(inv.ips),x=>fmt(x.fields?.address)));if(type==='network')root.append(listSection('主机',pick(inv.hosts)));else root.append(listSection('服务',pick(inv.services)))}
async function renderService(id){const d=await api('/api/services/'+encodeURIComponent(id));const o=d.service;clear(root);add(root,heading(literal(o.name),literal(`${t('服务 · ')}${o.id}`)));const overview=section('服务信息');overview.append(fieldRows({...o.fields,宿主:d.host?.name,宿主IP:(d.ips||[]).map(x=>x.fields?.address).join('、')||'—'},[],o.type));root.append(overview);const pair=el('div','detail-grid');add(pair,listSection('端口',d.ports,x=>`${fmt(x.fields?.bind)}:${fmt(x.fields?.port)}/${fmt(x.fields?.protocol)} · ${fmt(x.fields?.purpose)}`),listSection('访问入口',d.endpoints,x=>`${fmt(x.fields?.url)} · 经由 ${fmt(x.fields?.via_json)}`));root.append(pair);root.append(listSection('依赖',d.dependencies,x=>`${fmt(x.fields?.from_id)} → ${fmt(x.fields?.to_id)} · ${fmt(x.fields?.kind)}`));root.append(listSection('版本',d.versions,x=>`当前 ${fmt(x.fields?.current_version)} · 最新 ${fmt(x.fields?.latest_version)} · ${fmt(x.fields?.version_state)} · 来源 ${fmt(x.fields?.upstream_source)} · 原始 tag ${fmt(x.fields?.upstream_tag)}${x.fields?.check_error?' · 检查失败: '+x.fields.check_error:''}${x.fields?.pinned?' · 固定版本: '+fmt(x.fields?.pinned_reason):''}`));await appendTopology(id,'service')}
function diffText(change){let d=change.fields?.diff_json;try{if(typeof d==='string')d=JSON.parse(d)}catch{}return JSON.stringify(d||{},null,2)}
function modal(title,build){const overlay=el('div','modal-backdrop');const box=el('div','modal');add(box,el('h2','',title));const close=el('button','ghost','取消');close.onclick=()=>overlay.remove();build(box,close,overlay);overlay.append(box);overlay.onclick=e=>{if(e.target===overlay)overlay.remove()};document.body.append(overlay)}
async function renderKeys(){mountKeys(root,api)}
$('#login-form').onsubmit=async e=>{e.preventDefault();$('#login-error').textContent='';const f=new FormData(e.target);try{const p=await api('/api/auth/login',{method:'POST',body:JSON.stringify({login:f.get('login'),password:f.get('password')})});e.target.reset();showApp(p.login);navigate('overview');showFirstRun()}catch(err){$('#login-error').textContent=err.message}};
async function logout(){try{await api('/api/auth/logout',{method:'POST'})}finally{showLogin()}}
$('#logout').onclick=logout;
$('#mobile-logout').onclick=logout;
$('#mobile-search').onsubmit=e=>{e.preventDefault();pendingSearchTerm=$('#mobile-search input').value.trim();navigate('search')};
$('#change-password').onclick=()=>modal('修改密码',(box,close,overlay)=>{const form=el('form');const old=el('input');old.type='password';old.required=true;old.autocomplete='current-password';const next=el('input');next.type='password';next.required=true;next.minLength=8;next.autocomplete='new-password';const err=el('p','error');const save=el('button','primary','修改并重新登录');save.type='submit';add(form,add(el('label','','当前密码'),old),add(el('label','','新密码'),next),el('p','hint','至少 8 个字符；大写、小写、数字、符号至少三类；不得包含用户名或使用常见弱口令；最多 72 字节。'),err,add(el('div','modal-actions'),close,save));box.append(form);form.onsubmit=async e=>{e.preventDefault();save.disabled=true;try{await api('/api/auth/change-password',{method:'POST',body:JSON.stringify({old_password:old.value,new_password:next.value})});overlay.remove();showLogin()}catch(e){err.textContent=e.message;save.disabled=false}}});
$('#logout-all').onclick=async()=>{if(!confirm('退出所有设备，包括当前会话？'))return;try{await api('/api/auth/logout-all',{method:'POST'});showLogin()}catch(e){alert(e.message)}};
document.querySelectorAll('#nav button[data-view]').forEach(b=>b.onclick=()=>{navigate(b.dataset.view);document.querySelector('.sidebar')?.classList.remove('mobile-open')});
api('/api/config').then(p=>{document.querySelectorAll('.http-mode-notice').forEach(el=>{el.hidden=!p.http_risk})}).catch(err=>{$('#login-error').textContent=err.message});
function buildVersionLabel(info){const version=String(info?.version||'').trim();const commit=String(info?.commit||'').trim().match(/^[0-9a-f]{7}/i)?.[0].toLowerCase();if(/^v\d+\.\d+\.\d+$/.test(version))return `Rollbook ${version}`;if(version==='main'||version==='dev')return `Rollbook ${version}${commit?' '+commit:''}`;return 'Rollbook'}
api('/api/version').then(p=>{const label=buildVersionLabel(p);$('#build-version').textContent=label;$('#mobile-build-version').textContent=label;$('#login-build-version').textContent=label}).catch(()=>{$('#build-version').textContent='Rollbook';$('#mobile-build-version').textContent='Rollbook';$('#login-build-version').textContent='Rollbook'});
api('/api/auth/session').then(p=>{showApp(p.login);navigate(routeFromLocation());showFirstRun()}).catch(()=>showLogin());

async function renderBackup(){mountBackup(root,api,showLogin)}

// 表单字段与数据库中的业务模型保持一致；关联项从当前清册选择。
async function assetForm(type,original=null){openAssetForm(type,original,api,t,enumLabel,types,openObject)}
async function refreshObject(o){await openObject(o.id,o.type)}
function detailActions(o){const bar=el('div','controls detail-actions');const button=(label,fn,cls='ghost')=>{const b=el('button',cls,label);b.onclick=fn;bar.append(b)};
 if(formFields[o.type]&&o.type!=='note')button('编辑',()=>assetForm(o.type,o));
 button('标记核实',async()=>{const reason=prompt('核实原因');if(!reason)return;try{await api('/api/assets/'+encodeURIComponent(o.id)+'/verify',{method:'POST',body:JSON.stringify({rev:o.rev,reason})});refreshObject(o)}catch(e){alert(e.message)}});
 button('加笔记',()=>modal('添加笔记',(box,close,overlay)=>{const kind=el('select');for(const value of ['运维笔记','故障','决策','待办']){const opt=el('option','',literal(enumLabel('kind',value)));opt.value=value;kind.append(opt)}const body=el('textarea');body.required=true;const reason=el('input');reason.required=true;const form=el('form');add(form,add(el('label','','类型'),kind),add(el('label','','内容'),body),add(el('label','','原因'),reason));const err=el('div','error');const save=el('button','primary','保存笔记');save.type='submit';add(form,err,add(el('div','modal-actions'),close,save));box.append(form);form.onsubmit=async event=>{event.preventDefault();try{await api('/api/notes',{method:'POST',body:JSON.stringify({id:o.id,rev:o.rev,kind:kind.value,body_md:body.value,reason:reason.value})});overlay.remove();refreshObject(o)}catch(e){err.textContent=e.message}}}));
 if(o.type==='host'){if(o.status!=='停用')button('停用主机',()=>offlineDialog(o));else button('恢复主机',()=>restoreDialog(o))}
 button('删除',()=>deleteDialog(o),'danger');return bar}
function deleteDialog(o){modal('删除'+o.name,(box,close,overlay)=>{box.append(el('p','muted','删除后无法撤销。请确认两次。被引用时会列出引用方。'));const reason=el('input');reason.placeholder=t('删除原因');reason.required=true;const form=el('form');add(form,add(el('label','','操作原因'),reason));const err=el('div','error');const yes=el('button','danger','继续删除');yes.type='submit';add(form,err,add(el('div','modal-actions'),close,yes));box.append(form);form.onsubmit=async event=>{event.preventDefault();if(!confirm(`第二次确认：删除 ${o.name}？`))return;yes.disabled=true;try{await api('/api/assets/'+encodeURIComponent(o.id),{method:'DELETE',body:JSON.stringify({rev:o.rev,reason:reason.value})});overlay.remove();navigate('assets')}catch(e){err.textContent=e.message;yes.disabled=false}}})}
async function offlineDialog(o){try{const data=await api('/api/hosts/'+encodeURIComponent(o.id)+'/offline');modal('停用主机 · 预览影响',(box,close,overlay)=>{const preview=data.preview;for(const [key,label] of [['services','服务'],['ips','IP'],['endpoints','入口']])box.append(el('p','muted',`${label}：${preview[key].map(x=>x.name).join('、')||'无'}`));const options={};for(const [key,label] of [['services','连带停用服务'],['ips','释放 IP'],['endpoints','标记入口不可用']]){const input=el('input');input.type='checkbox';options[key]=input;add(box,add(el('label','check-row',''),input,el('span','',label)))}const reason=el('input');reason.required=true;reason.placeholder=t('停用原因');const err=el('div','error');const yes=el('button','danger','确认停用');const form=el('form');add(form,add(el('label','','操作原因'),reason),err,add(el('div','modal-actions'),close,yes));box.append(form);form.onsubmit=async event=>{event.preventDefault();yes.disabled=true;try{await api('/api/hosts/'+encodeURIComponent(o.id)+'/offline',{method:'POST',body:JSON.stringify({rev:o.rev,reason:reason.value,options:Object.fromEntries(Object.entries(options).map(([k,v])=>[k,v.checked]))})});overlay.remove();refreshObject(o)}catch(e){err.textContent=e.message;yes.disabled=false}}})}catch(e){alert(e.message)}}
async function restoreDialog(o){try{const data=await api('/api/hosts/'+encodeURIComponent(o.id)+'/offline');if(!data.operation_id)throw Error('没有可恢复的停用记录');modal('恢复主机',(box,close,overlay)=>{const reason=el('input');reason.required=true;reason.placeholder=t('恢复原因');const err=el('div','error');const form=el('form');const yes=el('button','primary','恢复');add(form,add(el('label','','操作原因'),reason),err,add(el('div','modal-actions'),close,yes));box.append(form);form.onsubmit=async event=>{event.preventDefault();yes.disabled=true;try{const result=await api('/api/hosts/'+encodeURIComponent(o.id)+'/restore',{method:'POST',body:JSON.stringify({operation_id:data.operation_id,reason:reason.value})});overlay.remove();await refreshObject(o);if(result.skipped.length)root.prepend(message('以下对象后来被修改，已跳过恢复：'+result.skipped.join('、')))}catch(e){err.textContent=e.message;yes.disabled=false}}})}catch(e){alert(e.message)}}
const originalBasic=renderBasic;renderBasic=async function(o){originalBasic(o);root.insertBefore(detailActions(o),root.children[1]);await appendAssetDetail(o.id)};
const originalSite=renderSite;renderSite=async function(id){await originalSite(id);root.insertBefore(detailActions(await getObject(id)),root.children[1]);await appendAssetDetail(id)};
const originalInventory=renderInventory;renderInventory=async function(id,type){await originalInventory(id,type);root.insertBefore(detailActions(await getObject(id)),root.children[1]);if(type==='host')await appendTopology(id,'host');await appendAssetDetail(id)};
const originalService=renderService;renderService=async function(id){await originalService(id);root.insertBefore(detailActions(await getObject(id)),root.children[1]);await appendAssetDetail(id)};

// 初次登录只在空清册显示引导。示例全部使用保留的文档地址与虚构名称。
let firstRunOpen=false;
async function showFirstRun(){if(firstRunOpen)return;try{const sites=await api('/api/browse?type=location');if(sites.length)return;firstRunOpen=true;modal('欢迎使用清册',(box,close,overlay)=>{close.textContent=t('稍后再说');close.onclick=()=>{firstRunOpen=false;overlay.remove()};box.append(el('p','muted','清册还是空的。可以建立第一个站点，或导入明确标记的虚构示例数据。示例使用文档专用 IP 和 .invalid 域名，不代表真实资产。'));const actions=el('div','controls');const site=el('button','primary','建立第一个站点');site.onclick=()=>{firstRunOpen=false;overlay.remove();assetForm('location')};const sample=el('button','ghost','导入虚构示例数据');const err=el('div','error');sample.onclick=async()=>{sample.disabled=true;try{await importSample();firstRunOpen=false;overlay.remove();await navigate('sites')}catch(e){err.textContent=e.message;sample.disabled=false}};add(actions,site,sample);add(box,actions,err,close)})}catch(e){console.error('首次引导检查失败',e)}}
async function importSample(){const suffix=Array.from(crypto.getRandomValues(new Uint8Array(16)),b=>b.toString(16).padStart(2,'0')).join('');const id=type=>type+':sample-'+suffix;const examples=[
 {type:'location',name:'【示例】云杉站点',fields:{kind:'站点'}},
 {type:'network',name:'【示例】文档网段',fields:{cidr:'192.0.2.0/24',location_id:id('location')}},
 {type:'host',name:'【示例】演示主机',fields:{kind:'虚拟机',location_id:id('location')}},
 {type:'ip',name:'【示例】192.0.2.10',fields:{address:'192.0.2.10',network_id:id('network'),host_id:id('host')}},
 {type:'service',name:'【示例】演示服务',fields:{host_id:id('host')}},
 {type:'endpoint',name:'【示例】演示入口',fields:{service_id:id('service'),kind:'内网直连',url:'https://demo.example.invalid'}}
 ];for(const item of examples){await api('/api/assets',{method:'POST',body:JSON.stringify({object:{...item,id:id(item.type),source:'人工'},base_rev:0,reason:'导入虚构示例数据'})})}}
// Markdown 先解析再过滤；仅保留 http(s) 链接，数据里的 HTML 不获得执行机会。
function safeMarkdown(value){
 const box=el('div','markdown');
 const html=marked.parse(String(value||''),{async:false});
 box.innerHTML=DOMPurify.sanitize(html,{FORBID_TAGS:['img','iframe','form','input','button','style','svg','math']});
 for(const link of box.querySelectorAll('a')){
  const href=safeLink(link.getAttribute('href'));
  if(!href){link.replaceWith(document.createTextNode(link.textContent));continue}
  link.href=href;link.target='_blank';link.rel='noopener noreferrer';
 }
 return box;
}
function detailRow(o,description){return row(o,description)}
async function appendAssetDetail(id){
 const d=await api('/api/assets/'+encodeURIComponent(id)+'/detail');
 const common=section('基本字段');
 common.append(fieldRows(d.object,['fields','aliases','tags'],d.object.type));
 root.insertBefore(common,root.children[2]||null);
 const notes=section('笔记');
 if(!d.notes.length)notes.append(empty());
 for(const n of d.notes){const item=el('article','detail-entry');add(item,el('strong','',literal(`${enumLabel('kind',n.fields.kind)} · ${n.fields.author||n.created_by} · ${n.created_at}`)),safeMarkdown(n.fields.body_md));notes.append(item)}
 root.append(notes);
 const creds=section('凭据引用');
 if(!d.credentials.length)creds.append(empty());
 for(const c of d.credentials)creds.append(detailRow(c,`${fmt(c.fields.vault)} / ${fmt(c.fields.item_ref)} · ${fmt(c.fields.purpose)}`));
 root.append(creds);
 const history=section('变更记录');
 if(!d.changes.length)history.append(empty());
 for(const c of d.changes)history.append(detailRow(c,`${fmt(c.fields.action)} · ${fmt(c.fields.actor_key_name||c.fields.actor)} · ${fmt(c.fields.reason)} · ${c.created_at}`));
 root.append(history);
 root.append(listSection('反向引用',d.referenced_by,x=>`${t(types[x.type]||x.type)} · ${x.name}`));
}
async function renderRecentChanges(){mountAudit(root,'changes',api,openObject)}
async function renderContexts(){mountAudit(root,'contexts',api,openObject)}

// 使用随程序嵌入的 Dagre 计算分层布局，SVG 由安全的 DOM API 构造。
async function appendTopology(id, type) {
  const sectionNode = section('拓扑与访问链');
  const controls = el('div', 'controls topology-controls');
  const graphButton = el('button', 'ghost', '分层拓扑');
  const accessButton = el('button', 'ghost', '访问链');
  const canvas = el('div', 'topology-canvas');
  add(controls, graphButton, accessButton);
  add(sectionNode, controls, canvas);
  root.append(sectionNode);
  const base = `/api/${type === 'host' ? 'hosts' : 'services'}/${encodeURIComponent(id)}/topology`;
  async function load(view) {
    graphButton.classList.toggle('selected', view === 'graph');
    accessButton.classList.toggle('selected', view === 'access');
    clear(canvas).append(message('正在加载…'));
    try { drawTopology(canvas, await api(base + (view === 'access' ? '?view=access' : '')), view); }
    catch (e) { clear(canvas).append(message(e.message, 'error')); }
  }
  graphButton.onclick = () => load('graph');
  accessButton.onclick = () => load('access');
  await load('graph');
}

function drawTopology(target, data, view) {
  target.replaceChildren();
  mountFlow(target, data, openObject, view==='access'?'LR':'TB');
}

async function renderTopologyPage(){mountTopology(root,api,openObject)}
let renderOverview=async function(){mountOverview(root,api,navigate)};
let renderAssets=async function(initialType='location'){mountAssets(root,api,assetForm,openObject,navigate,initialType)};
let renderSettings=function(){mountSettings(root,()=>navigate('backup'),()=>$('#change-password').click(),()=>$('#logout-all').click(),setLanguage)};
let pendingSearchTerm='';
function setSearchQuery(value){pendingSearchTerm=value}
let renderSearch=function(){mountSearch(root,api,openObject,pendingSearchTerm);pendingSearchTerm=''};
let renderPendingHub=async function(tab='pending'){mountPending(root,api,openObject,navigate,tab)};
const routePaths={overview:'/',assets:'/assets',services:'/assets/services',sites:'/assets/sites',hosts:'/assets/hosts',networks:'/assets/networks',topology:'/topology',pendingHub:'/pending',pending:'/pending',unverified:'/pending?tab=unverified',outdated:'/pending?tab=outdated',agent:'/agent',keys:'/agent',settings:'/settings',backup:'/settings/backup',changes:'/changes',contexts:'/contexts',search:'/search'};
function routeFromLocation(){const path=location.pathname;const tab=new URLSearchParams(location.search).get('tab');if(path==='/pending')return tab==='unverified'?'unverified':tab==='outdated'?'outdated':'pendingHub';return Object.keys(routePaths).find(key=>routePaths[key]===path)||'overview'}
const originalOpenObject=openObject;
openObject=async function(...args){unmountFlow();unmountOverview();unmountAssets();unmountSettings();unmountSearch();unmountPending();unmountKeys();unmountBackup();unmountAudit();unmountTopology();return originalOpenObject(...args)};
const originalNavigate=navigate;
navigate=async function(view,push=true){unmountFlow();unmountOverview();unmountAssets();unmountSettings();unmountSearch();unmountPending();unmountKeys();unmountBackup();unmountAudit();unmountTopology();const target=routePaths[view];if(push&&target&&target!==location.pathname+location.search)history.pushState({},'',target);return originalNavigate(view)};
window.addEventListener('popstate',()=>navigate(routeFromLocation(),false));
document.documentElement.lang=language;
for(const selector of ['#login','.sidebar','.topbar'])translateStatic(document.querySelector(selector));
const languageSelect=document.querySelector('#language');if(languageSelect){languageSelect.value=language;languageSelect.onchange=()=>setLanguage(languageSelect.value)}
const loginLanguageSelect=document.querySelector('#login-language');if(loginLanguageSelect){loginLanguageSelect.value=language;loginLanguageSelect.onchange=()=>setLanguage(loginLanguageSelect.value)}
export { navigate, api, buildVersionLabel, setSearchQuery };
