const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

for(const [locale,expectedHint,expectedError] of [
 ['zh-CN','至少 8 个字符','密码过于常见'],
 ['en-US','At least 8 characters','Password is too common'],
]) test(`password guidance and error in ${locale}`,async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  const page=await browser.newPage({locale});
  await page.route('http://rollbook.test/**',route=>{
   const p=new URL(route.request().url()).pathname;
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'index.html'))});
   for(const name of ['app.js','i18n.js','topology.js','dagre.min.js','style.css'])if(p==='/'+name)return route.fulfill({contentType:name.endsWith('.css')?'text/css':'application/javascript',body:fs.readFileSync(path.join(__dirname,name))});
   if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:false,http_risk:false}});
   if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
   if(p==='/api/version')return route.fulfill({json:{version:'test',commit:'test'}});
   if(p==='/api/auth/change-password')return route.fulfill({status:422,json:{code:'validation',message:'Validation failed',fields:[{field:'new_password',reason:expectedError}]}});
   return route.fulfill({json:[]});
  });
  await page.goto('http://rollbook.test/');
  await page.getByRole('button',{name:locale==='zh-CN'?'稍后再说':'Maybe later'}).click();
  await page.locator('#change-password').click();
  const modal=page.locator('.modal');
  await modal.getByText(expectedHint,{exact:false}).waitFor();
  assert.equal(await modal.locator('input[autocomplete="new-password"]').getAttribute('minlength'),'8');
  await modal.locator('input[autocomplete="current-password"]').fill('OldPass1!');
  await modal.locator('input[autocomplete="new-password"]').fill('Password1');
  await modal.locator('button[type="submit"]').click();
  await modal.getByText(expectedError,{exact:false}).waitFor();
  await page.screenshot({path:path.join(__dirname,`password-${locale}.png`)});
 } finally {await browser.close()}
});
