const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

const emptyProvider=kind=>({kind,enabled:false,source:'page',shadowed:false,config:{},secret_configured:false});
test('认证设置：浅深主题、320/1280、键盘、只写凭据和失败反馈',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:1280,height:900}});page.setDefaultTimeout(5000);
  const secret='FAKE-DOM-SECRET-123';let fail=false,signedIn=true;const writes=[];
  let providers=['ldap','oidc','feishu'].map(emptyProvider);
  await page.route('http://rollbook.test/**',route=>{
   const req=route.request(),url=new URL(req.url()),p=url.pathname;
   if(!p.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false,login_methods:{local:true,ldap:false,oidc:providers.some(item=>item.kind==='oidc'&&item.enabled),feishu:false}}});
   if(p==='/api/version')return route.fulfill({json:{version:'test'}});
   if(p==='/api/auth/session')return route.fulfill(signedIn?{json:{login:'owner',can_change_password:true}}:{status:401,json:{code:'unauthorized'}});
   if(p==='/api/auth/logout'&&req.method()==='POST'){signedIn=false;return route.fulfill({json:{ok:true}})}
   if(p==='/api/browse')return route.fulfill({json:[{id:'location:one',type:'location',name:'One'}]});
   if(p==='/api/auth/settings'&&req.method()==='GET')return fail?route.fulfill({status:500,json:{message:'测试读取失败'}}):route.fulfill({json:{providers,local_enabled:true}});
   if(p==='/api/auth/settings/identities')return route.fulfill({json:{users:[{login:'external',enabled:true,has_local_password:false},{login:'owner',enabled:true,has_local_password:true}],bindings:[]}});
   if(p==='/api/auth/settings/audit')return route.fulfill({json:{page:1,entries:[]}});
   if(p==='/api/auth/settings/oidc'&&req.method()==='PUT'){
    const data=req.postDataJSON();writes.push(data);providers=providers.map(item=>item.kind==='oidc'?{...item,enabled:data.enabled,config:data.config,secret_configured:true}:item);
    return route.fulfill({json:{saved:true}});
   }
   return route.fulfill({status:404,json:{message:'not found'}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  for(const theme of ['light','dark'])for(const width of [1280,320]){
   await page.evaluate(value=>localStorage.setItem('rollbook.theme',value),theme);await page.setViewportSize({width,height:900});await page.reload();
   if(width===1280){await page.locator('#nav button[data-view=settings]').click();await page.getByRole('button',{name:'打开认证设置'}).click()}
   else await page.goto('http://rollbook.test/settings/auth');
   assert.equal(new URL(page.url()).pathname,'/settings/auth');
   await page.getByRole('heading',{name:'认证设置'}).waitFor();
   await page.getByText('暂无绑定。').waitFor();
   const users=page.locator('.auth-settings table').first();assert.deepEqual(await users.locator('thead th').allTextContents(),['账号','状态','本地口令','操作']);assert.deepEqual(await users.locator('tbody tr').first().locator('td').nth(2).allTextContents(),['无']);
   const scroll=await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth);assert.equal(scroll,0);
   const out=process.env.AUTH_SETTINGS_SCREENSHOTS;if(out){fs.mkdirSync(out,{recursive:true});await page.screenshot({path:path.join(out,`auth-${theme}-${width}.png`),fullPage:true})}
  }
  const oidc=page.locator('.auth-provider').filter({has:page.getByRole('heading',{name:'通用 OIDC'})});
  await oidc.getByLabel('Issuer').fill('https://issuer.example');await oidc.getByLabel('Client ID').fill('client');
  await oidc.getByLabel('启用此身份源').check();await oidc.getByLabel('覆盖凭据').check();await oidc.getByLabel('新凭据').fill(secret);
  await oidc.getByRole('button',{name:'保存通用 OIDC'}).press('Enter');await page.getByRole('status').getByText('已保存').waitFor();
  await oidc.getByText('敏感凭据：已配置').waitFor();
  assert.equal(writes.length,1);assert.equal(writes[0].secret,secret);assert.equal(writes[0].config.redirect_url,'https://rollbook.test/api/auth/oidc/callback');
  await oidc.getByLabel('新凭据').waitFor({state:'detached'});assert.equal((await page.locator('body').innerText()).includes(secret),false);
  providers=providers.map(item=>item.kind==='ldap'?{...item,source:'environment',shadowed:true,enabled:true,secret_configured:true,config:{url:'ldaps://directory.example',instance:'directory'}}:item);
  await page.reload();const ldap=page.locator('.auth-provider').filter({has:page.getByRole('heading',{name:'LDAP / LDAPS'})});await ldap.getByText('已保存页面配置被环境遮蔽').waitFor();assert.equal(await ldap.getByRole('button',{name:'保存LDAP / LDAPS'}).count(),0);
  fail=true;await page.reload();await page.getByRole('heading',{name:'认证设置'}).waitFor();await page.getByRole('alert').getByText('测试读取失败').waitFor();
  fail=false;await page.reload();await page.getByText('暂无绑定。').waitFor();
  await page.locator('.account-avatar').click();await page.getByRole('menuitem',{name:'退出登录'}).click();
  await page.getByRole('link',{name:'使用 OIDC 登录'}).waitFor();
  assert.equal(new URL(page.url()).pathname,'/');
 }finally{await browser.close()}
});
