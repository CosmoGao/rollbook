const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

test('空库首次引导同视口浅深、键盘关闭和回焦',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const width of [1440,390,320]){
   const viewport={width,height:width===1440?900:844};
   for(const theme of ['light','dark']){
    const page=await browser.newPage({viewport,locale:'zh-CN'});
    await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);
    await page.route('http://rollbook.test/**',route=>{
     const p=new URL(route.request().url()).pathname;
     if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
     if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
     if(p==='/api/config')return route.fulfill({json:{http_risk:false}});
     if(p==='/api/version')return route.fulfill({json:{version:'test'}});
     if(p==='/api/browse')return route.fulfill({json:[]});
     if(p==='/api/changes'||p==='/api/assets/unverified'||p==='/api/versions/outdated')return route.fulfill({json:[]});
     return route.fulfill({status:404,json:{message:p}});
    });
    await installBundle(page);await page.goto('http://rollbook.test/');
    const dialog=page.getByRole('dialog',{name:'欢迎使用清册'});
    await dialog.waitFor();
    assert.equal(await page.locator('html').getAttribute('data-theme'),theme);
    assert.equal(await dialog.getByRole('button',{name:'导入虚构示例数据'}).count(),1);
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    assert.equal(await dialog.evaluate(el=>el.contains(document.activeElement)),true);
    const dir=process.env.ROLLBOOK_SCREENSHOT_DIR;
    if(dir){fs.mkdirSync(dir,{recursive:true});await page.screenshot({path:path.join(dir,`v1-first-run-${width}-${theme}.png`),animations:'disabled'})}
    await page.keyboard.press('Tab');
    assert.equal(await dialog.evaluate(el=>el.contains(document.activeElement)),true);
    await page.keyboard.press('Escape');await dialog.waitFor({state:'detached'});
    await page.waitForTimeout(150);
    const focused=await page.evaluate(()=>({id:document.activeElement?.id,current:document.activeElement?.getAttribute('aria-current'),menu:document.activeElement?.classList.contains('mobile-menu')}));
    assert.equal(focused.current==='page'||focused.id==='content'||focused.menu,true,JSON.stringify(focused));
    await page.close();
   }
  }
 }finally{await browser.close()}
});

test('离线读取与权限不足显示可读的错误状态',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const mode of ['offline','forbidden']){
   const page=await browser.newPage({viewport:{width:390,height:844},locale:'zh-CN'});
   await page.route('http://rollbook.test/**',route=>{
    const p=new URL(route.request().url()).pathname;
    if(p==='/assets')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
    if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
    if(p==='/api/config')return route.fulfill({json:{http_risk:false}});
    if(p==='/api/version')return route.fulfill({json:{version:'test'}});
    if(p==='/api/browse')return mode==='offline'?route.abort('internetdisconnected'):route.fulfill({status:403,json:{code:'forbidden',message:'权限不足'}});
    return route.fulfill({json:[]});
   });
   await installBundle(page);await page.goto('http://rollbook.test/assets');
   const alert=page.locator('#content [role="alert"]').first();await alert.waitFor();
   assert.ok((await alert.innerText()).trim().length>0);
   if(mode==='forbidden')assert.match(await alert.innerText(),/权限不足/);
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
   await page.close();
  }
 }finally{await browser.close()}
});
