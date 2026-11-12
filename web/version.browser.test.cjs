const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

const webFile=name=>fs.readFileSync(path.join(__dirname,name));

async function openPage(page,{version,commit,loggedIn=false,failVersion=false}){
 await page.route('http://rollbook.test/**',route=>{
  const url=new URL(route.request().url()),p=url.pathname;
  if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
  if(p==='/api/version')return failVersion?route.fulfill({status:503,json:{}}):route.fulfill({json:{version,commit}});
  if(p==='/api/config')return route.fulfill({json:{http_risk:false}});
  if(p==='/api/auth/session')return loggedIn?route.fulfill({json:{login:'owner'}}):route.fulfill({status:401,json:{code:'unauthorized'}});
  if(p==='/api/browse')return route.fulfill({json:[{id:'location:site',type:'location',name:'Site',fields:{kind:'站点'}}]});
  return route.fulfill({json:[]});
 });
 await installBundle(page);await page.goto('http://rollbook.test/');
}

test('版本在中英文登录页和工作台一致，且窄屏可见',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const [locale,version,commit,expected,width] of [
   ['zh-CN','v0.4.2','1234567','Rollbook v0.4.2',1280],
   ['en-US','main','abcdef0','Rollbook main abcdef0',390],
  ]){
   const context=await browser.newContext({locale,viewport:{width,height:844}});
   const page=await context.newPage();
   await openPage(page,{version,commit});
   await page.locator('#login-build-version').getByText(expected).waitFor();
   assert.equal(await page.locator('#login-build-version').isVisible(),true);
   assert.equal(await page.locator('#login-build-version').evaluate(el=>{const r=el.getBoundingClientRect();return r.bottom<=innerHeight&&r.left>=0&&r.right<=innerWidth}),true);
   assert.equal(await page.locator('#login-build-version').textContent(),expected);
   if(process.env.ROLLBOOK_SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,`login-${locale}.png`),fullPage:true});
   await page.unrouteAll();
   await openPage(page,{version,commit,loggedIn:true});
   const label=page.locator(width<760?'#mobile-build-version':'#build-version');
   await label.getByText(expected).waitFor();
   assert.equal(await label.isVisible(),true);
   assert.equal(await label.textContent(),expected);
   if(process.env.ROLLBOOK_SCREENSHOT_DIR)await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,`app-${locale}.png`),fullPage:true});
   await context.close();
  }
 }finally{await browser.close()}
});

test('未知提交与请求失败均不显示空括号或额外信息',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const [version,commit,failVersion,expected] of [
   ['main','',false,'Rollbook main'],
   ['main','unknown',false,'Rollbook main'],
   ['v0.4.2','abcdef0',false,'Rollbook v0.4.2'],
   ['../../build/path','abcdef0',false,'Rollbook'],
   ['main','abcdef0',true,'Rollbook'],
  ]){
   const page=await browser.newPage();
   await openPage(page,{version,commit,failVersion});
   await page.locator('#login-build-version').getByText(expected,{exact:true}).waitFor();
   assert.equal(await page.locator('#login-build-version').textContent(),expected);
   await page.close();
  }
 }finally{await browser.close()}
});
