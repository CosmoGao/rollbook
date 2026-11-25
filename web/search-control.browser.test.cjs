const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

async function checkFrame(page,{theme,width,state,focused}){
 const styles=await page.locator('.search-box').evaluate(form=>{
  const outer=getComputedStyle(form),rect=form.getBoundingClientRect();
  const marker=document.createElement('span');marker.style.color='var(--color-focus)';form.append(marker);const focus=getComputedStyle(marker).color;marker.remove();
  const children=[...form.querySelectorAll('input,button')].map(el=>{
   const s=getComputedStyle(el);return {borders:[s.borderTopWidth,s.borderRightWidth,s.borderBottomWidth,s.borderLeftWidth],outline:s.outlineWidth,shadow:s.boxShadow,rect:el.getBoundingClientRect().toJSON()};
  });
  return {borders:[outer.borderTopWidth,outer.borderRightWidth,outer.borderBottomWidth,outer.borderLeftWidth],color:outer.borderTopColor,focus,outline:outer.outlineWidth,shadow:outer.boxShadow,radii:[outer.borderTopLeftRadius,outer.borderTopRightRadius,outer.borderBottomRightRadius,outer.borderBottomLeftRadius],submitRadius:getComputedStyle(form.querySelector('button[type=submit]')).borderTopRightRadius,rect:rect.toJSON(),children,overflow:document.documentElement.scrollWidth-innerWidth};
 });
 assert.deepEqual(styles.borders,['1px','1px','1px','1px'],JSON.stringify({theme,width,state,styles}));
 assert.equal(styles.outline,'0px',JSON.stringify({theme,width,state,styles}));if(focused)assert.ok(styles.shadow.includes('inset'),JSON.stringify({theme,width,state,styles}));else assert.equal(styles.shadow,'none');
 assert.deepEqual(styles.radii,['8px','8px','8px','8px']);assert.equal(styles.submitRadius,'7px');
 if(focused)assert.equal(styles.color,styles.focus,JSON.stringify({theme,width,state,styles}));
 for(const child of styles.children){assert.deepEqual(child.borders,['0px','0px','0px','0px']);assert.equal(child.outline,'0px');assert.equal(child.shadow,'none');assert.ok(child.rect.left>=styles.rect.left && child.rect.right<=styles.rect.right+0.1)}
 assert.ok(styles.rect.left>=0 && styles.rect.right<=width,JSON.stringify({theme,width,state,styles}));
 assert.ok(styles.overflow<=0,JSON.stringify({theme,width,state,styles}));
 if(process.env.ROLLBOOK_SCREENSHOT_DIR){fs.mkdirSync(process.env.ROLLBOOK_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,`search-control-${theme}-${width}-${state}.png`)})}
}

test('搜索框在浅深主题与宽窄视口共用一条贴合 border-box 的框线',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const width of [1280,320])for(const theme of ['light','dark']){
   const page=await browser.newPage({viewport:{width,height:800},locale:'zh-CN'});
   await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);
   let requests=0,fail=false;
   await page.route('http://rollbook.test/**',route=>{
    const p=new URL(route.request().url()).pathname;
    if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
    if(p==='/api/auth/session')return route.fulfill({json:{login:'owner'}});
    if(p==='/api/config')return route.fulfill({json:{http_risk:false}});
    if(p==='/api/version')return route.fulfill({json:{version:'test'}});
    if(p==='/api/search'){requests++;return fail?route.fulfill({status:500,json:{error:'test'}}):route.fulfill({json:[]})}
    return route.fulfill({json:[]});
   });
   await installBundle(page);await page.goto('http://rollbook.test/');
   await page.keyboard.press('Control+k');
   const form=page.locator('.search-box'),input=page.getByRole('searchbox',{name:'搜索服务和资产'});
   await input.waitFor();await input.evaluate(el=>el.blur());
   await checkFrame(page,{theme,width,state:'idle',focused:false});
   await form.hover();await checkFrame(page,{theme,width,state:'hover',focused:false});
   await input.focus();await checkFrame(page,{theme,width,state:'focus',focused:true});
   await input.fill('a'.repeat(180));
   const clear=page.getByRole('button',{name:'清空搜索'});await clear.waitFor();
   await checkFrame(page,{theme,width,state:'long-query',focused:true});
   await clear.focus();await checkFrame(page,{theme,width,state:'clear-focus',focused:true});
   await page.keyboard.press('Enter');assert.equal(await input.inputValue(),'');assert.equal(await input.evaluate(el=>document.activeElement===el),true);
   await input.fill('test');await form.locator('button[type=submit]').focus();
   await checkFrame(page,{theme,width,state:'submit-focus',focused:true});
   await page.keyboard.press('Enter');await page.locator('.empty').waitFor();assert.ok(requests>0);
   await checkFrame(page,{theme,width,state:'empty-result',focused:true});
   fail=true;await input.fill('error-'+theme+'-'+width);await page.getByRole('alert').waitFor();
   await checkFrame(page,{theme,width,state:'error-result',focused:true});
   await page.close();
  }
 }finally{await browser.close()}
});
