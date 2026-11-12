const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('390px 搜索、查看详情、复制 IP、退出，页面没有横向溢出',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const context=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true,locale:'zh-CN'});
  await context.grantPermissions(['clipboard-read','clipboard-write'],{origin:'http://localhost:8910'});
  const page=await context.newPage();
  const host={id:'host:box',type:'host',name:'测试主机',status:'在用',fields:{kind:'物理机',mgmt_url:'https://example.invalid/'+ 'long'.repeat(60)}};
  const ip={id:'ip:box',type:'ip',name:'192.0.2.126',fields:{address:'192.0.2.126',host_id:'host:box'}};
  let signedOut=false;
  await page.route('http://localhost:8910/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'index.html'))});
   if(['/app.js','/i18n.js','/topology.js','/dagre.min.js','/marked.umd.js','/purify.min.js','/style.css'].includes(p))return route.fulfill({contentType:p.endsWith('.css')?'text/css':'application/javascript',body:fs.readFileSync(path.join(__dirname,p.slice(1)))});
   if(p==='/manifest.webmanifest')return route.fulfill({contentType:'application/manifest+json',body:fs.readFileSync(path.join(__dirname,'manifest.webmanifest'))});
   const reply=json=>route.fulfill({json});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/auth/logout'){signedOut=true;return reply({})}
   if(p==='/api/config')return reply({allow_insecure_http:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply(u.searchParams.get('type')==='location'?[{id:'location:site'}]:[]);
   if(p==='/api/search')return reply([host]);
   if(p==='/api/assets/host:box')return reply(host);
   if(p==='/api/assets/ip:box')return reply(ip);
   if(p==='/api/hosts/host:box/inventory')return reply({ips:['ip:box'],hosts:[],services:[]});
   if(p==='/api/assets/host:box/detail')return reply({object:host,notes:[],credentials:[],changes:[],referenced_by:[]});
   return route.fulfill({status:404,json:{message:'unexpected '+p}});
  });
  await page.goto('http://localhost:8910/');
  await page.locator('#mobile-user-name').getByText('owner').waitFor();
  await page.locator('#mobile-search input').fill('126');
  await page.locator('#mobile-search button').click();
  await page.getByText('测试主机').first().waitFor();
  await page.locator('.search-box').waitFor();
  await page.locator('.card').first().click();
  await page.getByText('192.0.2.126').first().waitFor();
  await page.getByRole('button',{name:'复制 192.0.2.126'}).first().click();
  assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'192.0.2.126');
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  const manifest=await page.evaluate(async()=>{const r=await fetch('/manifest.webmanifest');return r.json()});
  assert.equal(manifest.display,'standalone');
  assert.equal(manifest.icons.length,2);
  await page.locator('#mobile-logout').click();
  await page.locator('#login-form').waitFor({state:'visible'});
  assert.equal(signedOut,true);
 }finally{await browser.close()}
});
