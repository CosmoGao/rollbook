const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

for(const canChange of [true,false,undefined]) test('账号入口与设置：'+String(canChange),async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:390,height:844},hasTouch:true});page.setDefaultTimeout(4000);
  const calls=[];
  let loggedIn=true,failLogout=false;
  page.on('dialog',dialog=>dialog.accept());
  await page.route('http://rollbook.test/**',route=>{
   const url=new URL(route.request().url()),p=url.pathname;
   if(!p.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false}});
   if(p==='/api/version')return route.fulfill({json:{version:'test'}});
   if(p==='/api/auth/session')return route.fulfill(loggedIn?{json:{login:'owner',identity_type:canChange===false?'external':'local',auth_provider:canChange===false?'oidc':undefined,...(canChange===undefined?{}:{can_change_password:canChange})}}:{status:401,json:{}});
   if(p==='/api/auth/logout'||p==='/api/auth/logout-all'){
    calls.push(p);
    if(failLogout)return route.fulfill({status:500,json:{message:'测试失败'}});
    loggedIn=false;return route.fulfill({json:{}});
   }
   if(p==='/api/browse')return route.fulfill({json:[{id:'location:one',type:'location',name:'One'}]});
   return route.fulfill({json:[]});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  for(const theme of ['light','dark']){
   await page.evaluate(value=>localStorage.setItem('rollbook.theme',value),theme);
   await page.reload();
   await page.waitForFunction(value=>document.documentElement.dataset.theme===value,theme);
   for(const width of [390,1440]){
    await page.setViewportSize({width,height:844});
    const avatar=page.locator('.account-avatar');
    await avatar.waitFor();
    const box=await avatar.boundingBox();
    assert.ok(box&&box.width>=44&&box.height>=44);
    await avatar.click();
    await page.getByRole('menuitem',{name:'个人设置'}).click();
    await page.getByRole('heading',{name:'个人设置'}).waitFor();
    await page.locator('.account-details dd').first().waitFor();
    assert.equal(new URL(page.url()).pathname,'/settings/personal');
    assert.equal(await page.getByRole('button',{name:'改密码'}).count(),canChange===false?0:1);
    if(canChange===false)await page.getByText('此账号的密码由上游身份提供方管理').waitFor();
    await avatar.click();await page.getByRole('menuitem',{name:'退出所有设备'}).waitFor();
    await page.keyboard.press('Escape');
   }
  }
  await page.locator('.account-avatar').click();
  await page.getByRole('menuitem',{name:'个人设置'}).click();
  await page.getByRole('button',{name:'退出所有设备'}).click();
  await page.locator('#login-form').waitFor({state:'visible'});
  assert.deepEqual(calls,['/api/auth/logout-all']);
 }finally{await browser.close()}
});

test('备份入口、退出失败反馈与当前会话退出',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:1440,height:900}});
  page.setDefaultTimeout(4000);
  let signedIn=true,fail=true;const calls=[];
  page.on('dialog',dialog=>dialog.accept());
  await page.route('http://rollbook.test/**',route=>{
   const p=new URL(route.request().url()).pathname;
   if(!p.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false}});
   if(p==='/api/version')return route.fulfill({json:{version:'test'}});
   if(p==='/api/auth/session')return route.fulfill(signedIn?{json:{login:'owner',can_change_password:true}}:{status:401,json:{}});
   if(p==='/api/auth/logout'||p==='/api/auth/logout-all'){
    calls.push(p);
    if(fail)return route.fulfill({status:500,json:{message:'测试失败'}});
    signedIn=false;return route.fulfill({json:{}});
   }
   if(p==='/api/browse')return route.fulfill({json:[{id:'location:one',type:'location',name:'One'}]});
   return route.fulfill({json:[]});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#nav button[data-view=settings]').click();
  await page.getByRole('button',{name:'打开备份与恢复'}).click();
  assert.equal(new URL(page.url()).pathname,'/settings/backup');
  await page.locator('.account-avatar').click();
  await page.getByRole('menuitem',{name:'退出登录'}).click();
  await page.getByRole('alert').getByText('测试失败').waitFor();
  assert.equal(await page.locator('#login-form').isVisible(),false);
  fail=false;
  await page.getByRole('menuitem',{name:'退出登录'}).click();
  await page.locator('#login-form').waitFor({state:'visible'});
  assert.deepEqual(calls,['/api/auth/logout','/api/auth/logout']);
 }finally{await browser.close()}
});
