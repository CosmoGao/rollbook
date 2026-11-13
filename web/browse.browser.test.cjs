const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('主机树逐层显示 IP/状态并可折叠；网段显示冲突和空闲地址',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN'});page.setDefaultTimeout(5000);
  const assets=[
   {id:'location:site',type:'location',name:'站点',status:'在用',fields:{kind:'站点'}},
   {id:'host:p',type:'host',name:'物理机',status:'在用',fields:{kind:'物理机'}},
   {id:'host:v',type:'host',name:'虚拟机',status:'在用',fields:{kind:'虚拟机',parent_host_id:'host:p'}},
   {id:'host:c',type:'host',name:'容器',status:'停用',fields:{kind:'容器',parent_host_id:'host:v'}},
   {id:'service:s',type:'service',name:'服务',status:'在用',fields:{host_id:'host:c'}},
   {id:'ip:p',type:'ip',name:'物理机 IP',fields:{host_id:'host:p',address:'192.0.2.2'}},
   {id:'ip:v',type:'ip',name:'虚机 IP',fields:{host_id:'host:v',address:'192.0.2.3'}},
   {id:'ip:c',type:'ip',name:'容器 IP',fields:{host_id:'host:c',address:'192.0.2.4'}},
   {id:'network:n',type:'network',name:'文档网段',status:'在用',fields:{cidr:'192.0.2.0/29'}}
  ];
  await page.route('http://rollbook.test/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   const reply=json=>route.fulfill({json});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({allow_insecure_http:false,http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply(assets.filter(a=>a.type===u.searchParams.get('type')));
   if(p==='/api/networks/network:n/inventory')return reply({id:'network:n',ips:[],hosts:[],occupancy:{cidr:'192.0.2.0/29',gateway:'192.0.2.1',dhcp_range:'',next_available:'192.0.2.6',used:[{address:'192.0.2.5',ip_id:'ip:a',host_id:'host:p',conflict:true},{address:'192.0.2.5',ip_id:'ip:b',host_id:'host:v',conflict:true}],free:[{start:'192.0.2.6',end:'192.0.2.6'}]}});
   if(p.startsWith('/api/assets/'))return reply(assets.find(a=>a.id===p.slice('/api/assets/'.length)));
   return route.fulfill({status:404,json:{message:'not found'}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#nav button[data-view="assets"]').click();await page.getByRole('tab',{name:'主机'}).click();await page.getByRole('group',{name:'主机视图'}).getByRole('button',{name:'树'}).click();
  await page.locator('.tree-node').first().waitFor();
  assert.equal(await page.locator('.tree-node').count(),4);
  assert.match(await page.locator('.tree-node').last().locator('summary').first().textContent(),/192\.0\.2\.4.*在用/);
  assert.match(await page.locator('.tree-node').nth(2).locator('summary').first().textContent(),/192\.0\.2\.4.*停用/);
  await page.locator('.tree-node').first().locator('summary').first().click();
  assert.equal(await page.locator('.tree-node').first().evaluate(el=>el.open),false);
  await page.locator('#nav button[data-view="assets"]').click();await page.getByRole('tab',{name:'网段'}).click();
  await page.getByText('文档网段').click();
  await page.getByText('冲突').first().waitFor();
  assert.equal(await page.locator('.tag.conflict').count(),2);
  assert.match(await page.locator('#content').innerText(),/下一个可分配 IP：192\.0\.2\.6/);
 }finally{await browser.close()}
});

test('网页重复登记 IP 时显示占用主机',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN'});page.setDefaultTimeout(5000);
  const assets=[
   {id:'location:site',type:'location',name:'站点',status:'在用',fields:{kind:'站点'}},
   {id:'network:net',type:'network',name:'网段',status:'在用',fields:{cidr:'192.0.2.0/24',location_id:'location:site'}},
   {id:'host:owner',type:'host',name:'已占用主机',status:'在用',fields:{kind:'物理机',location_id:'location:site'}},
   {id:'host:other',type:'host',name:'新主机',status:'在用',fields:{kind:'物理机',location_id:'location:site'}}
  ];
  let submitted=false;
  await page.route('http://rollbook.test/**',route=>{
   const req=route.request(),u=new URL(req.url()),p=decodeURIComponent(u.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   const reply=(json,status=200)=>route.fulfill({status,json});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({allow_insecure_http:false,http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply(assets.filter(a=>a.type===u.searchParams.get('type')));
   if(p==='/api/assets'&&req.method()==='POST'){
    submitted=true;
    return reply({code:'validation',message:'校验失败',fields:[{field:'fields.address',reason:'IP 地址已被主机占用',references:['192.0.2.5：已占用主机 (host:owner)']}]},422);
   }
   return reply({message:'not found'},404);
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#nav button[data-view="assets"]').click();
  await page.locator('.asset-type-select').selectOption('ip');
  await page.getByRole('button',{name:'新建'}).click();
  const form=page.locator('.asset-form');
  await form.locator('label').filter({hasText:'ID（可修改）'}).locator('input').fill('ip:duplicate');
  await form.locator('label').filter({hasText:'名称'}).locator('input').fill('重复 IP');
  await form.locator('label').filter({hasText:'地址'}).first().locator('input').fill('192.0.2.5');
  await form.locator('label').filter({hasText:'宿主'}).locator('select').selectOption('host:other');
  await form.locator('label').filter({hasText:'操作原因'}).locator('input').fill('登记测试');
  await form.getByRole('button',{name:'创建'}).click();
  await form.locator('.error').getByText(/已占用主机/).waitFor();
  assert.equal(submitted,true);
  assert.match(await form.locator('.error').textContent(),/192\.0\.2\.5.*host:owner/);
 }finally{await browser.close()}
});
