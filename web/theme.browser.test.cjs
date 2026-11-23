const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

test('主题在登录、导航和退出后显式选择且一次生效',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const width of [320,375,1280])for(const system of ['light','dark']){
   const page=await browser.newPage({locale:'zh-CN',viewport:{width,height:800},colorScheme:system,reducedMotion:'reduce'});
   page.setDefaultTimeout(5000);
   let signedIn=false;
   await page.route('http://rollbook.test/**',route=>{
    const pathname=new URL(route.request().url()).pathname;
    if(!pathname.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
    if(pathname==='/api/auth/login'){signedIn=true;return route.fulfill({json:{login:'tester'}})}
    if(pathname==='/api/auth/logout'){signedIn=false;return route.fulfill({json:{}})}
    if(pathname==='/api/auth/session')return route.fulfill(signedIn?{json:{login:'tester'}}:{status:401,json:{}});
    if(pathname==='/api/config')return route.fulfill({json:{login_methods:{local:true}}});
    if(pathname==='/api/browse')return route.fulfill({json:[{id:'site',type:'location',name:'Site'}]});
    return route.fulfill({json:[]});
   });
   await page.addInitScript(()=>{if(!localStorage.getItem('rollbook.theme'))localStorage.setItem('rollbook.theme','light')});
   await installBundle(page);
   await page.goto('http://rollbook.test/');
   const login=page.locator('.login-theme select');await login.waitFor({state:'visible'});
   assert.equal(await page.locator('html').getAttribute('data-theme'),'light');
   for(const choice of ['dark','light','dark','system']){
    await login.selectOption(choice);
    const effective=choice==='system'?system:choice;
    assert.equal(await page.locator('html').getAttribute('data-theme'),effective);
    assert.equal(await page.evaluate(()=>localStorage.getItem('rollbook.theme')),choice);
    assert.equal(await login.inputValue(),choice);
    assert.match(await page.locator('.login-theme svg').getAttribute('class'),new RegExp('lucide-'+(choice==='system'?'monitor':choice==='dark'?'moon':'sun')));
    await page.waitForFunction(expected=>getComputedStyle(document.querySelector('.login-wrap')).backgroundColor===expected,effective==='dark'?'rgb(15, 23, 42)':'rgb(248, 250, 252)');
   }
   await page.emulateMedia({colorScheme:system==='dark'?'light':'dark'});
   await page.waitForFunction(value=>document.documentElement.dataset.theme===value,system==='dark'?'light':'dark');
   await login.selectOption('dark');await page.reload();await login.waitFor({state:'visible'});
   assert.equal(await login.inputValue(),'dark');assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');
   await login.focus();await page.keyboard.press('Home');await page.keyboard.press('Enter');
   assert.equal(await login.inputValue(),'light');assert.equal(await page.locator('html').getAttribute('data-theme'),'light');
   await login.focus();await page.keyboard.press('Space');await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');
   assert.equal(await login.inputValue(),'dark');assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');
   await page.locator('#login-form input[name=login]').fill('tester');
   await page.locator('#login-form input[name=password]').fill('password');
   await page.locator('#login-form button[type=submit]').click();
   const app=page.locator('.top-controls .theme-control select');await app.waitFor({state:'visible'});
   assert.equal(await app.inputValue(),'dark');
   await app.selectOption('light');assert.equal(await page.locator('html').getAttribute('data-theme'),'light');
   await page.goto('http://rollbook.test/settings/personal');await app.waitFor({state:'visible'});
   assert.equal(await app.inputValue(),'light');
   await page.locator('.account-avatar').click();await page.getByRole('menuitem',{name:'退出登录'}).click();
   await login.waitFor({state:'visible'});assert.equal(await login.inputValue(),'light');
   assert.equal(await page.locator('html').getAttribute('data-theme'),'light');
   assert.ok(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1));
   await page.close();
  }
 }finally{await browser.close()}
});
