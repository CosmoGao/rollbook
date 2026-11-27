const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

const providers=['ldap','oidc','feishu'].map(kind=>({kind,enabled:false,source:'page',shadowed:false,config:{},secret_configured:false}));
const preview={kind:'merge',counts:{create:2,update:1,conflict:1},objects:[{id:'host:a',type:'host',action:'create'},{id:'host:b',type:'host',action:'conflict',reason:'rev 不一致'}]};
test('统一设置分组草稿、结构化预览与窄屏导航',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:1440,height:900}});page.setDefaultTimeout(5000);
  await page.route('http://rollbook.test/**',route=>{
   const url=new URL(route.request().url()),p=url.pathname;
   if(!p.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/config')return route.fulfill({json:{http_risk:false,login_methods:{local:true}}});
   if(p==='/api/version')return route.fulfill({json:{version:'test'}});
   if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
   if(p==='/api/browse')return route.fulfill({json:[{id:'site:a',type:'location',name:'A'}]});
   if(p==='/api/auth/settings')return route.fulfill({json:{providers,local_enabled:true}});
   if(p==='/api/auth/settings/identities')return route.fulfill({json:{users:[],bindings:[]}});
   if(p==='/api/auth/settings/audit')return route.fulfill({json:{page:1,entries:[]}});
   if(p==='/api/backup/files')return route.fulfill({json:[]});
   if(p==='/api/backup/imports')return route.fulfill({json:[{id:'import:1',kind:'merge',proposer:'owner',created_at:'2026-01-01',state:'pending',preview}]});
   return route.fulfill({json:[]});
  });
  await installBundle(page);await page.goto('http://rollbook.test/settings?section=unknown');
  await page.getByRole('heading',{name:'本地登录'}).waitFor();assert.equal(new URL(page.url()).pathname+new URL(page.url()).search,'/settings');
  assert.equal(await page.locator('#content h1').count(),1);
  await page.getByRole('button',{name:'LDAP / LDAPS'}).click();await page.getByLabel('服务器 URL').fill('ldaps://draft.example');
  await page.getByRole('button',{name:'备份与恢复'}).click();
  assert.match(await page.locator('.settings-nav button').filter({hasText:'LDAP / LDAPS'}).innerText(),/未保存/);
  assert.match(await page.locator('.settings-preview-totals').innerText(),/新增 2[\s\S]*更新 1[\s\S]*删除 0[\s\S]*冲突 1/);
  await page.getByText('查看原始预览 JSON').click();await page.locator('.settings-preview pre').waitFor();
  await page.getByLabel('导入方式').selectOption('catalog');
  assert.match(await page.locator('.settings-nav button').filter({hasText:'备份与恢复'}).innerText(),/未保存/);
  for(const width of [768,390,320]){
   await page.setViewportSize({width,height:900});
   await page.getByText('其他分组未保存',{exact:false}).waitFor();
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`overflow ${width}`);
   const rows=await page.locator('.settings-nav').evaluate(el=>({scrollHeight:el.scrollHeight,clientHeight:el.clientHeight}));
   assert.ok(rows.scrollHeight<=rows.clientHeight+2,`nav wrapped ${width}`);
  }
  await page.getByRole('button',{name:/LDAP \/ LDAPS/}).click();assert.equal(await page.getByLabel('服务器 URL').inputValue(),'ldaps://draft.example');
  await page.setViewportSize({width:1440,height:900});
  page.once('dialog',dialog=>dialog.dismiss());await page.locator('#nav button[data-view=overview]').click();
  assert.equal(new URL(page.url()).pathname,'/settings');
  await page.emulateMedia({forcedColors:'active'});assert.match(await page.locator('.settings-unsaved').first().innerText(),/未保存/);
 }finally{await browser.close()}
});
