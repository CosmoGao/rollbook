const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

test('全局搜索在浅深与宽窄视口只有一层描边和焦点环，键盘清空及提交可用',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const width of [1280,320])for(const theme of ['light','dark']){
   const page=await browser.newPage({viewport:{width,height:800},locale:'zh-CN'});
   await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);
   let requests=0;
   await page.route('http://rollbook.test/**',route=>{
    const p=new URL(route.request().url()).pathname;
    if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
    if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
    if(p==='/api/config')return route.fulfill({json:{http_risk:false}});
    if(p==='/api/version')return route.fulfill({json:{version:'test'}});
    if(p==='/api/search'){requests++;return route.fulfill({json:[]})}
    return route.fulfill({json:[]});
   });
   await installBundle(page);await page.goto('http://rollbook.test/');
   await page.keyboard.press('Control+k');
   const input=page.getByRole('searchbox',{name:'搜索服务和资产'});
   await input.waitFor();
   await input.fill('a'.repeat(180));
   const clear=page.getByRole('button',{name:'清空搜索'});
   await clear.waitFor();
   const styles=await page.locator('.search-box').evaluate(form=>{
    const input=form.querySelector('input'),submit=form.querySelector('button[type=submit]');
    const outer=getComputedStyle(form),inner=getComputedStyle(input),button=getComputedStyle(submit);
    return {outerBorder:outer.borderTopWidth,outerOutline:outer.outlineWidth,inputBorder:inner.borderTopWidth,inputOutline:inner.outlineWidth,buttonBorder:button.borderTopWidth,buttonOutline:button.outlineWidth,overflow:document.documentElement.scrollWidth-innerWidth};
   });
   assert.equal(styles.outerBorder,'1px');assert.equal(styles.outerOutline,'2px');
   assert.equal(styles.inputBorder,'0px');assert.equal(styles.inputOutline,'0px');
   assert.equal(styles.buttonBorder,'0px');assert.equal(styles.buttonOutline,'0px');
   assert.ok(styles.overflow<=0,JSON.stringify({width,theme,styles}));
   await clear.focus();await page.keyboard.press('Enter');assert.equal(await input.inputValue(),'');
   assert.equal(await input.evaluate(el=>document.activeElement===el),true);
   await input.fill('test');await page.locator('.search-box button[type="submit"]').focus();
   await page.keyboard.press('Enter');await page.locator('.empty').waitFor();assert.ok(requests>0);
   if(process.env.ROLLBOOK_SCREENSHOT_DIR){fs.mkdirSync(process.env.ROLLBOOK_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,`search-control-${width}-${theme}.png`)})}
   await page.close();
  }
 }finally{await browser.close()}
});
