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
   if(p==='/api/auth/profile')return route.fulfill({json:{email:'owner@example.test',feishu_enabled:false,feishu_bound:false,local_session:true}});
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
  const settingsLeft=await page.locator('#content h1').first().evaluate(el=>el.getBoundingClientRect().left);
  await page.getByRole('button',{name:'账号与绑定'}).click();
  const binding=await page.locator('.settings-binding-form').evaluate(el=>el.getBoundingClientRect().width);
  const saveBinding=await page.getByRole('button',{name:'保存绑定'}).evaluate(el=>el.getBoundingClientRect().width);
  assert.ok(binding<=800,`binding form too wide: ${binding}`);
  const accountForm=await page.locator('.auth-inline').evaluate(el=>el.getBoundingClientRect().width);
  assert.ok(accountForm<=800,`account form too wide: ${accountForm}`);
  assert.ok(saveBinding<200,`save button stretched: ${saveBinding}`);
  for(const width of [1440,390]){
   await page.setViewportSize({width,height:900});
   const geometry=await page.locator('.settings-binding-form .auth-check').evaluate(label=>{
    const input=label.querySelector('input[type=checkbox]');
    const range=document.createRange();range.selectNodeContents(label.lastChild);
    const box=input.getBoundingClientRect(),text=range.getBoundingClientRect();
    return {display:getComputedStyle(label).display,width:box.width,height:box.height,inputRight:box.right,inputMidY:box.top+box.height/2,textLeft:text.left,textMidY:text.top+text.height/2};
   });
   assert.equal(geometry.display,'flex',`checkbox label layout at ${width}px`);
   assert.ok(geometry.width>=16&&geometry.width<=20&&geometry.height>=16&&geometry.height<=20,`checkbox size at ${width}px: ${JSON.stringify(geometry)}`);
   assert.ok(geometry.textLeft>=geometry.inputRight&&Math.abs(geometry.textMidY-geometry.inputMidY)<=4,`checkbox and text alignment at ${width}px: ${JSON.stringify(geometry)}`);
  }
  await page.setViewportSize({width:1440,height:900});
  for(const name of ['LDAP / LDAPS','通用 OIDC','飞书']){
   await page.getByRole('button',{name,exact:true}).click();
   const sizes=await page.locator('.auth-provider input[type=checkbox],.auth-provider input[type=radio]').evaluateAll(inputs=>inputs.map(input=>({type:input.type,width:input.getBoundingClientRect().width,height:input.getBoundingClientRect().height})));
   assert.ok(sizes.some(size=>size.type==='checkbox')&&sizes.some(size=>size.type==='radio'),`missing controls for ${name}`);
   assert.ok(sizes.every(size=>size.width<=20&&size.height<=20),`oversized controls for ${name}: ${JSON.stringify(sizes)}`);
  }
  await page.goto('http://rollbook.test/settings/personal');
  await page.getByRole('heading',{name:'个人设置'}).waitFor();
  const personalLeft=await page.locator('#content h1').first().evaluate(el=>el.getBoundingClientRect().left);
  assert.ok(Math.abs(settingsLeft-personalLeft)<=2,`personal title shifted: ${settingsLeft} vs ${personalLeft}`);
  await page.goto('http://rollbook.test/settings');
  await page.getByRole('heading',{name:'本地登录'}).waitFor();
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
