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
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'index.html'))});
   for(const name of ['app.js','i18n.js','style.css'])if(p==='/'+name)return route.fulfill({contentType:name.endsWith('.css')?'text/css':'application/javascript',body:fs.readFileSync(path.join(__dirname,name))});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false}});
   if(p==='/api/auth/session')return route.fulfill({status:401,json:{code:'unauthorized',message:'Authentication required'}});
   if(p==='/api/version')return route.fulfill({json:{version:'test',commit:'test'}});
   return route.fulfill({status:404,json:{code:'not_found',message:'Not found'}});
  });
  await page.goto('http://rollbook.test/');
  await page.getByRole('heading',{name:'Sign in to Rollbook'}).waitFor();
  assert.equal(await page.locator('html').getAttribute('lang'),'en');
  await page.locator('#language').selectOption('zh-CN');
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
  const page=await browser.newPage({locale:'en-US'});
  await page.route('http://rollbook.test/**',route=>{
   const p=new URL(route.request().url()).pathname;
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'index.html'))});
   for(const name of ['app.js','i18n.js','style.css'])if(p==='/'+name)return route.fulfill({contentType:name.endsWith('.css')?'text/css':'application/javascript',body:fs.readFileSync(path.join(__dirname,name))});
   if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false}});
   if(p==='/api/version')return route.fulfill({json:{version:'test',commit:'test'}});
   return route.fulfill({json:[]});
  });
  await page.goto('http://rollbook.test/');
  await page.getByRole('button',{name:'Maybe later'}).click();
  await page.locator('#nav button[data-view="assets"]').click();
  await page.getByRole('heading',{name:'Manage assets'}).waitFor();
  await page.getByRole('button',{name:'New',exact:true}).click();
  await page.getByRole('heading',{name:'New site'}).waitFor();
  await page.locator('.modal').getByText('Name',{exact:true}).waitFor();
  assert.equal(await page.locator('.modal option[value="在用"]').textContent(),'Active');
 }finally{await browser.close()}
});
