const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('搜索显示名称和状态；危险入口及管理链接保持纯文本',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN'});page.setDefaultTimeout(5000);
  const assets={
   'location:site':{id:'location:site',type:'location',name:'云杉站点',status:'在用',fields:{kind:'站点'}},
   'network:net':{id:'network:net',type:'network',name:'测试网段',status:'停用',fields:{cidr:'192.0.2.0/24'}},
   'host:one':{id:'host:one',type:'host',name:'宿主机',status:'在用',fields:{kind:'物理机',mgmt_url:'javascript:alert(1)'}},
   'service:one':{id:'service:one',type:'service',name:'服务',status:'在用',fields:{host_id:'host:one',docs_url:'https://docs.example.invalid/path'}},
   'endpoint:bad':{id:'endpoint:bad',type:'endpoint',name:'危险入口',status:'停用',fields:{url:'javascript:alert(1)'}},
   'endpoint:good':{id:'endpoint:good',type:'endpoint',name:'安全入口',status:'在用',fields:{url:'https://service.example.invalid/'}},
  };
  const hits=[{id:'ip:126',type:'ip',name:'192.0.2.126',status:'在用',site_id:'location:site',network_id:'network:net',related_ids:['host:one','service:one','endpoint:bad','endpoint:good']}];
  await page.route('http://rollbook.test/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'index.html'))});
   if(['/app.js','/i18n.js','/topology.js','/dagre.min.js','/style.css'].includes(p))return route.fulfill({contentType:p.endsWith('.css')?'text/css':'application/javascript',body:fs.readFileSync(path.join(__dirname,p.slice(1)))});
   const reply=json=>route.fulfill({json});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({allow_insecure_http:false,http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply([assets['location:site']]);
   if(p==='/api/search')return reply(hits);
   if(p.startsWith('/api/assets/'))return reply(assets[p.slice('/api/assets/'.length)]);
   if(p==='/api/hosts/host:one/inventory')return reply({id:'host:one',ips:[],services:[]});
   if(p==='/api/services/service:one')return reply({service:assets['service:one'],host:assets['host:one'],ips:[],ports:[],endpoints:[],dependencies:[],versions:[],credentials:[],notes:[],changes:[]});
   if(p==='/api/topology')return reply({nodes:[],edges:[],details:[]});
   return route.fulfill({status:404,json:{message:'not found'}});
  });
  await page.goto('http://rollbook.test/');
  await page.locator('.search-box input').fill('126');
  await page.getByText('危险入口').waitFor();
  const content=page.locator('#content');
  assert.match(await content.locator('.search-context').first().innerText(),/云杉站点\s*在用/);
  assert.match(await content.locator('.search-context').nth(1).innerText(),/测试网段\s*停用/);
  assert.equal(await content.locator('article.card').count(),5);
  assert.equal(await content.locator('a[href^="javascript:"]').count(),0);
  assert.equal(await content.getByText('javascript:alert(1)').count(),1);
  assert.equal(await content.locator('a[href="https://service.example.invalid/"]').count(),1);
  await content.getByText('宿主机').click();
  await page.locator('.kv').getByText('javascript:alert(1)').waitFor();
  assert.equal(await content.locator('a[href^="javascript:"]').count(),0);
  await page.locator('#nav button[data-view="search"]').click();
  await page.locator('.search-box input').fill('126');
  await page.getByText('服务',{exact:true}).last().click();
  await page.locator('a[href="https://docs.example.invalid/path"]').waitFor();
 }finally{await browser.close()}
});
