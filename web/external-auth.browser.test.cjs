const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

test('external methods, failed SSO return and local fallback in a browser',async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  const page=await browser.newPage({locale:'zh-CN'});
  let localLogins=0,ldapLogins=0;
  await page.route('http://rollbook.test/**',route=>{
   const p=new URL(route.request().url()).pathname;
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false,login_methods:{local:true,ldap:true,oidc:true,feishu:true}}});
   if(p==='/api/version')return route.fulfill({json:{version:'test',commit:'test'}});
   if(p==='/api/auth/session')return route.fulfill({status:401,json:{code:'unauthorized'}});
   if(p==='/api/auth/ldap/login'){ldapLogins++;return route.fulfill({status:401,json:{code:'unauthorized',message:'Unauthorized'}})}
   if(p==='/api/auth/login'){localLogins++;return route.fulfill({json:{login:'owner'}})}
   return route.fulfill({json:[]});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.getByRole('link',{name:'使用 OIDC 登录'}).waitFor();
  await page.getByRole('link',{name:'使用飞书登录'}).waitFor();
  for(const theme of ['light','dark']){
   await page.evaluate(value=>localStorage.setItem('rollbook.theme',value),theme);
   await page.reload();await page.getByRole('link',{name:'使用 OIDC 登录'}).waitFor();
   await page.waitForFunction(value=>document.documentElement.dataset.theme===value,theme);
   for(const width of [1440,1024,768,390,320]){
    await page.setViewportSize({width,height:800});
    const card=await page.locator('#login-form').boundingBox();
    assert.ok(card&&card.x>=-1&&card.x+card.width<=width+1,`login card overflow at ${width}px ${theme}`);
    const button=await page.getByRole('button',{name:'本地账号'}).boundingBox();
    assert.ok(button&&button.height>=44,`small sign-in method target at ${width}px`);
   }
  }
  assert.equal(await page.getByRole('link',{name:'使用 OIDC 登录'}).getAttribute('href'),'/api/auth/oidc/start');
  await page.goto('http://rollbook.test/?auth_error=external_login_failed');
  await page.getByText('外部登录失败，请重试或选择其他方式。').waitFor({timeout:3000});
  await page.getByRole('button',{name:'LDAP'}).click();
  await page.locator('#login-form input[name="login"]').fill('owner');
  await page.locator('#login-form input[name="password"]').fill('bad');
  await page.locator('#login-form button[type="submit"]').click();
  await page.locator('#login-error').getByText('Unauthorized').waitFor();
  await page.getByRole('button',{name:'本地账号'}).click();
  await page.locator('#login-form input[name="login"]').fill('owner');
  await page.locator('#login-form input[name="password"]').fill('password');
  await page.locator('#login-form button[type="submit"]').click();
  await page.waitForFunction(()=>document.querySelector('#user-name')?.textContent==='owner');
  assert.equal(ldapLogins,1);assert.equal(localLogins,1);
 } finally {await browser.close()}
});
