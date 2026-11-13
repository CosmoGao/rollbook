const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('分层画布在真实规模下筛选、展开、访问与影响',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({viewport:{width:1280,height:720},locale:'zh-CN'});page.setDefaultTimeout(8000);
  const nodes=[{id:'location:beijing',name:'北京家',type:'location',kind:'站点'},{id:'location:xianghe',name:'香河',type:'location',kind:'站点'},{id:'location:rack',name:'主卧机柜',type:'location',kind:'机柜',parent_id:'location:beijing'}];
  for(let i=0;i<29;i++)nodes.push({id:'host:h'+i,name:i===0?'Proxmox VE':'主机 '+i,type:'host',kind:i===0?'物理机':i===1?'虚拟机':'NAS',location_id:i<15?'location:rack':'location:xianghe',...(i===1?{parent_id:'host:h0'}:{})});
  for(let i=0;i<41;i++)nodes.push({id:'service:s'+i,name:i===0?'Immich':'服务 '+i,type:'service',host_id:i===0?'host:h1':'host:h'+(i%29)});
  for(let i=0;i<24;i++)nodes.push({id:'endpoint:e'+i,name:'入口 '+i,type:'endpoint',host_id:'service:s'+i,url:i===0?'https://photos.invalid':'https://example.invalid/'+i});
  const edges=Array.from({length:97},(_,i)=>({from:'service:s'+(i%41),to:'service:s'+((i+1)%41),kind:'依赖'}));
  let layersCalls=0,serviceTopologyCalls=0;
  await page.route('http://rollbook.test/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname),reply=json=>route.fulfill({json});
   if(p==='/'||p==='/topology')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/auth/session')return reply({login:'owner'});if(p==='/api/config')return reply({http_risk:false});if(p==='/api/version')return reply({version:'test'});
   if(p==='/api/topology/layers'){layersCalls++;return reply({nodes,edges})}
   if(p==='/api/services/service:s0/topology'){serviceTopologyCalls++;return reply({nodes:['endpoint:e0','service:s0','host:h1','host:h0'],details:['endpoint:e0','service:s0','host:h1','host:h0'].map(id=>({id,name:nodes.find(n=>n.id===id).name,type:id.split(':')[0]})),edges:[{from:'endpoint:e0',to:'service:s0',kind:'访问'},{from:'service:s0',to:'host:h1',kind:'宿主'},{from:'host:h1',to:'host:h0',kind:'父主机'}]})}
   if(p==='/api/hosts/host:h0/impact')return reply({root:{id:'host:h0',name:'Proxmox VE',type:'host'},hosts:[{object:{id:'host:h1',name:'主机 1',type:'host'},path:['host:h0','host:h1']}],services:[{object:{id:'service:s0',name:'Immich',type:'service'},path:['host:h0','host:h1','service:s0']}],endpoints:[]});
   if(p==='/api/browse'||p==='/api/changes'||p==='/api/assets/unverified'||p==='/api/versions/outdated')return reply([]);
   return route.fulfill({status:404,json:{message:p}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/topology');await page.getByRole('tab',{name:'资产分层'}).waitFor();await page.locator('.topology-flow .react-flow__node').first().waitFor();
  assert.equal(layersCalls,1);assert.equal(serviceTopologyCalls,0);assert.equal(await page.locator('.topology-flow .react-flow__node').count(),2);assert.equal(await page.locator('.topology-flow .react-flow__edge').count(),1);assert.match(await page.locator('.topology-flow .react-flow__edge-text').textContent(),/站点互通.*[0-9]+/);
  fs.mkdirSync(path.join(__dirname,'screenshots'),{recursive:true});await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-60-default.png')});
  await page.getByLabel('站点',{exact:true}).selectOption('location:beijing');await page.locator('.topology-flow .react-flow__node').filter({hasText:'Proxmox VE'}).waitFor();assert.equal(await page.getByText('该站点暂无资产').count(),0);await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-60-beijing.png')});
  await page.locator('.topology-host-card > button').filter({hasText:'Proxmox VE'}).click();await page.locator('.topology-host-children').filter({hasText:'主机 1'}).waitFor();await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-60-expanded.png')});
  await page.getByLabel('聚焦对象').selectOption('service:s0');await page.getByRole('tab',{name:'访问链'}).click();await page.locator('.topology-flow .react-flow__node').filter({hasText:'https://photos.invalid'}).waitFor();assert.equal(serviceTopologyCalls,1);await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-60-access.png')});
  await page.getByLabel('聚焦对象').selectOption('host:h0');await page.getByRole('tab',{name:'影响面'}).click();await page.locator('.topology-flow .react-flow__node').filter({hasText:'Immich'}).waitFor();await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-60-impact.png')});await page.goto('http://rollbook.test/topology?focus=host%3Ah0&view=impact');await page.getByRole('tab',{name:'影响面'}).waitFor();await page.locator('.topology-flow .react-flow__node').filter({hasText:'Immich'}).waitFor();
 }finally{await browser.close()}
});
