const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('browser language, manual choice, and refresh',async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  const context=await browser.newContext({locale:'en-US'});
  const page=await context.newPage();
  await page.route('http://rollbook.test/**',route=>{
   const p=new URL(route.request().url()).pathname;
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false}});
   if(p==='/api/auth/session')return route.fulfill({status:401,json:{code:'unauthorized',message:'Authentication required'}});
   if(p==='/api/version')return route.fulfill({json:{version:'test',commit:'test'}});
   return route.fulfill({status:404,json:{code:'not_found',message:'Not found'}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.getByRole('heading',{name:'Sign in to Rollbook'}).waitFor();
  assert.equal(await page.locator('html').getAttribute('lang'),'en');
  await page.locator('#login-language').selectOption('zh-CN');
  await page.getByRole('heading',{name:'登录清册'}).waitFor();
  await page.reload();
  await page.getByRole('heading',{name:'登录清册'}).waitFor();
  assert.equal(await page.locator('html').getAttribute('lang'),'zh-CN');
  assert.equal(await page.evaluate(()=>localStorage.getItem('rollbook.language')),'zh-CN');
 } finally {await browser.close()}
});

test('English workbench and creation form use translated copy',async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  const page=await browser.newPage({locale:'en-US'});page.setDefaultTimeout(5000);
  await page.route('http://rollbook.test/**',route=>{
   const p=new URL(route.request().url()).pathname;
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false}});
   if(p==='/api/version')return route.fulfill({json:{version:'test',commit:'test'}});
   return route.fulfill({json:[]});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.getByRole('button',{name:'Maybe later'}).click();
  await page.locator('#nav button[data-view="assets"]').click();
  await page.getByRole('heading',{name:'Manage assets'}).waitFor();
  await page.getByRole('button',{name:'New',exact:true}).click();
  await page.getByRole('heading',{name:'New site'}).waitFor();
  await page.locator('.modal').getByText('Name',{exact:true}).waitFor();
  assert.equal(await page.locator('.modal option[value="在用"]').textContent(),'Active');
 }finally{await browser.close()}
});

test('English interface preserves asset names and IDs while translating kind labels',async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  const page=await browser.newPage({locale:'en-US'});page.setDefaultTimeout(5000);
  const assets=[
   {id:'location:zh',type:'location',name:'中文站点',status:'在用',fields:{kind:'站点'}},
   {id:'location:exact',type:'location',name:'站点',status:'在用',fields:{kind:'站点'}},
   {id:'host:sample',type:'host',name:'【示例】演示主机',status:'在用',fields:{kind:'物理机',location_id:'location:zh'}},
  ];
  await page.route('http://rollbook.test/**',route=>{
   const url=new URL(route.request().url()),p=url.pathname;
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/auth/session')return route.fulfill({json:{login:'站点'}});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false}});
   if(p==='/api/version')return route.fulfill({json:{version:'test',commit:'test'}});
   if(p==='/api/browse')return route.fulfill({json:assets.filter(o=>o.type===url.searchParams.get('type'))});
   return route.fulfill({json:[]});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#user-name').getByText('站点').waitFor();
  await page.locator('#nav button[data-view="assets"]').click();
  await page.locator('.tree-title').first().waitFor();
  assert.deepEqual(await page.locator('.tree-title').allTextContents(),['中文站点','站点']);
  await page.getByRole('tab',{name:'Hosts'}).click();
  await page.locator('.table-name').first().waitFor();
  assert.deepEqual(await page.locator('.table-name').allTextContents(),['【示例】演示主机']);
  await page.getByRole('button',{name:'New',exact:true}).click();
  assert.equal(await page.locator('.asset-form label').filter({hasText:'Location'}).locator('option[value="location:zh"]').textContent(),'Sites · 中文站点 (location:zh)');
  await page.locator('.modal .ghost').click();
  await page.getByRole('tab',{name:'Sites'}).click();
  await page.getByRole('button',{name:'New',exact:true}).click();
  assert.equal(await page.locator('.asset-form label').filter({hasText:'Kind'}).locator('option[value="站点"]').textContent(),'Site');
  await page.locator('.asset-form label').filter({hasText:'Name'}).locator('input').fill('中文站点');
  assert.equal(await page.locator('.asset-form label').filter({hasText:'Name'}).locator('input').inputValue(),'中文站点');
 }finally{await browser.close()}
});
