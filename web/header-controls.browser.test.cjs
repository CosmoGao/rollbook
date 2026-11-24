const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

const screenshotDir=process.env.ROLLBOOK_SCREENSHOT_DIR;
const baseline=process.env.ROLLBOOK_BASELINE==='1';

async function capture(page,theme,width,selector,state){
 const safe=selector.replace(/[^a-z-]/g,'');
 if(screenshotDir){fs.mkdirSync(screenshotDir,{recursive:true});await page.screenshot({path:path.join(screenshotDir,`${theme}-${width}-${safe}-${state}.png`)});}
 const result=await page.locator(selector).evaluate(outer=>{
  const style=getComputedStyle(outer),rect=outer.getBoundingClientRect();
  const child=[...outer.querySelectorAll('input,select,button')].map(el=>{const s=getComputedStyle(el);return {border:s.borderTopWidth,outline:s.outlineStyle==='none'?'0px':s.outlineWidth,offset:s.outlineOffset,rect:el.getBoundingClientRect().toJSON()}});
  const marker=document.createElement('span');marker.style.color='var(--color-focus)';outer.append(marker);const focus=getComputedStyle(marker).color;marker.remove();
  return {border:style.borderTopWidth,color:style.borderTopColor,focus,radius:style.borderTopLeftRadius,outline:style.outlineWidth,children:child,rect:rect.toJSON(),overflow:document.documentElement.scrollWidth-innerWidth};
 });
 if(!baseline){
  assert.equal(result.border,'1px',JSON.stringify({theme,width,selector,state,result}));
  assert.equal(result.radius,'8px');
  assert.equal(result.outline,'0px');
  assert.ok(result.rect.left>=0 && result.rect.right<=width,JSON.stringify(result));
  assert.ok(result.overflow<=0,JSON.stringify(result));
  for(const child of result.children){assert.equal(child.border,'0px',JSON.stringify({theme,width,selector,state,result}));assert.equal(child.outline,'0px',JSON.stringify({theme,width,selector,state,result}));assert.ok(child.rect.left>=result.rect.left-0.1 && child.rect.right<=result.rect.right+0.1,JSON.stringify(result));}
  if(state==='focus'||state==='input'||state==='open'||state==='submit-focus')assert.equal(result.color,result.focus,JSON.stringify({theme,width,selector,state,result}));
 }
 return result;
}

test('HEADER 搜索与主题控件在浅深、宽窄视口保持单一外框和完整操作',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const theme of ['light','dark'])for(const width of [1280,390,320]){
   const page=await browser.newPage({viewport:{width,height:800},locale:'zh-CN'});
   await page.addInitScript(value=>{if(!localStorage.getItem('rollbook.theme'))localStorage.setItem('rollbook.theme',value)},theme);
   await page.route('http://rollbook.test/**',route=>{
    const p=new URL(route.request().url()).pathname;
    if(p==='/'||p==='/search')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
    if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
    if(p==='/api/config')return route.fulfill({json:{http_risk:false}});
    if(p==='/api/version')return route.fulfill({json:{version:'test'}});
    return route.fulfill({json:[]});
   });
   await installBundle(page);await page.goto('http://rollbook.test/');
   await page.getByRole('button',{name:'稍后再说'}).click();
   const search=width===1280?'form.top-search':'form.mobile-search';
   const form=page.locator(search),input=form.locator('input');
   await form.waitFor({state:'visible'});
   assert.equal(await page.locator(width===1280?'form.mobile-search':'form.top-search').isVisible(),false);
   await capture(page,theme,width,search,'idle');
   await form.hover();await capture(page,theme,width,search,'hover');
   await input.focus();
   assert.equal(await input.evaluate(el=>el===document.activeElement),true);
   await capture(page,theme,width,search,'focus');
   await input.fill('很长的搜索内容'.repeat(30));
   await capture(page,theme,width,search,'input');
   if(width!==1280){
    await form.locator('button[type="submit"]').focus();
    await capture(page,theme,width,search,'submit-focus');
   }
   await input.fill('server');await input.press('Enter');
   await page.waitForURL('**/search');
   await page.keyboard.press('Control+k');
   await page.waitForFunction(()=>document.activeElement===document.querySelector('.search-box input'));
   assert.equal(await page.locator('.search-box input').evaluate(el=>el===document.activeElement),true);
   await page.keyboard.press('Meta+k');
   await page.waitForFunction(()=>document.activeElement===document.querySelector('.search-box input'));
   const header=page.locator('header.topbar label.theme-control');
   await header.waitFor({state:'visible'});
   await capture(page,theme,width,'header.topbar label.theme-control','idle');
   await header.hover();await capture(page,theme,width,'header.topbar label.theme-control','hover');
   const select=header.locator('select');await select.focus();
   await capture(page,theme,width,'header.topbar label.theme-control','focus');
   await select.press('Space');await capture(page,theme,width,'header.topbar label.theme-control','open');
   await select.press('Escape');
   await select.selectOption(theme==='light'?'dark':'light');
   assert.equal(await page.evaluate(()=>localStorage.getItem('rollbook.theme')),theme==='light'?'dark':'light');
   assert.equal(await page.locator('html').getAttribute('data-theme'),theme==='light'?'dark':'light');
   await page.reload();
   await page.waitForFunction(expected=>document.documentElement.dataset.theme===expected,theme==='light'?'dark':'light');
   assert.equal(await page.locator('html').getAttribute('data-theme'),theme==='light'?'dark':'light');
   await page.emulateMedia({forcedColors:'active'});
   const forcedSearch=page.locator(search);
   await forcedSearch.locator('input').focus();
   const forcedFocus=await forcedSearch.evaluate(el=>({outline:getComputedStyle(el).outlineStyle,offset:getComputedStyle(el).outlineOffset}));
   assert.equal(forcedFocus.outline,'solid');assert.equal(forcedFocus.offset,'0px');
   await page.locator('header.topbar label.theme-control select').focus();
   const forcedTheme=await page.locator('header.topbar label.theme-control').evaluate(el=>({outline:getComputedStyle(el).outlineStyle,offset:getComputedStyle(el).outlineOffset}));
   assert.equal(forcedTheme.outline,'solid');assert.equal(forcedTheme.offset,'0px');
   await page.close();
  }
 }finally{await browser.close()}
});
