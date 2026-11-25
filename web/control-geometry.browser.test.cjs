const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

const routes=['/','/assets','/assets/services','/assets/sites','/assets/hosts','/assets/networks','/search','/pending','/pending?tab=unverified','/pending?tab=outdated','/changes','/contexts','/topology','/agent','/settings','/settings/personal','/settings/auth','/settings/backup'];
const evidence=path.join(__dirname,'../docs/control-geometry-evidence');
const before=process.env.ROLLBOOK_BASELINE==='1';
const captureEvidence=process.env.ROLLBOOK_GEOMETRY_EVIDENCE==='1';

async function setup(page,authenticated=true){
 await installBundle(page);
 await page.route('http://rollbook.test/**',route=>{
  const url=new URL(route.request().url()),p=url.pathname;
  if(p.startsWith('/assets/')&&fs.existsSync(path.join(__dirname,'dist',p)))return route.fallback();
  if(!p.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
  if(p==='/api/auth/session')return authenticated?route.fulfill({json:{login:'owner'}}):route.fulfill({status:401,json:{}});
  if(p==='/api/config')return route.fulfill({json:{http_risk:false}});
  if(p==='/api/version')return route.fulfill({json:{version:'test'}});
  if(p==='/api/versions/alerts')return route.fulfill({json:{enabled:false,webhook_url:'',format:'feishu'}});
  if(p==='/api/auth/settings')return route.fulfill({json:{providers:['ldap','oidc','feishu'].map(kind=>({kind,enabled:false,source:'page',shadowed:false,config:{},secret_configured:false})),local_enabled:true}});
  if(p==='/api/auth/settings/identities')return route.fulfill({json:{users:[{login:'owner',enabled:true,has_local_password:true}],bindings:[]}});
  if(p==='/api/auth/settings/audit')return route.fulfill({json:{page:1,entries:[]}});
  if(p==='/api/keys')return route.fulfill({json:[{id:'key-fixture',name:'测试 Key',level:'read',issuer_name:'owner',note:'',created_at:'2026-01-01T00:00:00Z'}]});
  if(p==='/api/browse'&&url.searchParams.get('type')==='location')return route.fulfill({json:[{id:'test-site',type:'location',name:'测试站点'}]});
  return route.fulfill({json:[]});
 });
}

async function measure(locator){return locator.evaluate(el=>{
 const s=getComputedStyle(el),r=el.getBoundingClientRect();
 return {selector:(()=>{let x=el;const a=[];while(x&&x!==document.body&&a.length<5){let n=x.tagName.toLowerCase();if(x.id)n+='#'+x.id;else if(x.classList.length)n+='.'+[...x.classList].slice(0,2).join('.');a.unshift(n);x=x.parentElement}return a.join(' > ')})(),type:el.tagName==='INPUT'?(el.type||'text'):el.tagName.toLowerCase(),border:s.borderTopWidth,borderColor:s.borderTopColor,outline:s.outlineWidth,outlineStyle:s.outlineStyle,offset:s.outlineOffset,shadow:s.boxShadow,radius:s.borderTopLeftRadius,rect:r.toJSON(),invalid:el.getAttribute('aria-invalid'),disabled:el.disabled,readOnly:el.readOnly,focus:el.matches(':focus-visible')};
 });}

test('全站文本控件在真实页面中共用自身或组合外框的焦点几何',async()=>{
 const browser=await chromium.launch({headless:true});
 const report=[];
 try{
  for(const theme of ['light','dark'])for(const width of [1280,390]){
   for(const route of routes.filter(value=>!process.env.ROLLBOOK_ROUTE||value===process.env.ROLLBOOK_ROUTE)){
    const page=await browser.newPage({viewport:{width,height:820},locale:'zh-CN'});
    await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);
    await setup(page);await page.goto('http://rollbook.test'+route);
    await page.locator('#app').waitFor({state:'visible',timeout:5000}).catch(async error=>{throw new Error(`${route} ${theme} ${width}: ${error.message}; ${await page.locator('body').innerText()}`)});
    await page.waitForTimeout(100);
    const controls=page.locator('input:not([type=checkbox]):not([type=radio]),select,textarea');
    const entries=[];
    for(let i=0;i<await controls.count();i++){
     const control=controls.nth(i);if(!await control.isVisible())continue;
     const idle=await measure(control);
     if(idle.rect.width<1||idle.rect.height<1)continue;
     await control.focus();const focused=await measure(control);
     const owner=await control.evaluate(el=>!!el.closest('form.search-box,form.top-search,form.mobile-search,label.theme-control'));
     const entry={idle,focused,owner};entries.push(entry);
     const target=route==='/pending'&&idle.selector.includes('pending-toolbar')||(!before&&theme==='light'&&width===1280&&!idle.selector.includes('header.topbar')&&entries.filter(item=>!item.idle.selector.includes('header.topbar')).length===1);
     if(captureEvidence&&target){fs.mkdirSync(evidence,{recursive:true});const name=(before?'before':'after')+'-'+(route.replace(/[^a-z]+/gi,'-').replace(/^-|-$/g,'')||'overview')+'-'+theme+'-'+width+'.png';await (owner?control.locator('..'):control).screenshot({path:path.join(evidence,name)});}
     if(captureEvidence&&!before&&route==='/'&&idle.selector.includes('header.topbar')&&owner){const kind=idle.selector.includes('top-search')?'top-search':idle.selector.includes('mobile-search')?'mobile-search':'theme-control';fs.mkdirSync(evidence,{recursive:true});await control.locator('..').screenshot({path:path.join(evidence,`after-header-${kind}-${theme}-${width}.png`)});}
     if(!before){
      if(owner){assert.equal(focused.outline,'0px',JSON.stringify({route,theme,width,entry}));assert.equal(focused.border,'0px',JSON.stringify({route,theme,width,entry}));assert.equal(focused.shadow,'none',JSON.stringify({route,theme,width,entry}));}
      else {assert.equal(focused.offset,'0px',JSON.stringify({route,theme,width,entry}));assert.equal(focused.outline,'0px',JSON.stringify({route,theme,width,entry}));assert.ok(focused.shadow.includes('inset')||focused.disabled,JSON.stringify({route,theme,width,entry}));}
     }
    }
    report.push({route,theme,width,controls:entries});
    await page.close();
   }
  }
 }finally{await browser.close()}
 if(captureEvidence){fs.mkdirSync(evidence,{recursive:true});fs.writeFileSync(path.join(evidence,before?'baseline.json':'verified.json'),JSON.stringify(report));}
});

test('登录、资产弹窗、Key 弹窗、个人密码与认证各组的控件焦点',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const theme of ['light','dark'])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:820},locale:'zh-CN'});
   await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);
   await setup(page,false);await page.goto('http://rollbook.test/');await page.locator('#login-form').waitFor({state:'visible'});
   for(const selector of ['#login-form input[name=login]','#login-form input[name=password]','#login-language','.login-theme select']){
    const control=page.locator(selector);await control.focus();const item=await measure(control);
    if(selector==='.login-theme select')assert.equal(item.shadow,'none');
    else {assert.equal(item.offset,'0px');assert.equal(item.outline,'0px');assert.ok(item.shadow.includes('inset'));}
   }
   await page.close();
   const views=[
    {url:'/assets',button:'新建',container:'.asset-form'},
    {url:'/agent',button:'创建 Key',container:'.modal'},
    {url:'/settings/personal',button:'改密码',container:'.modal'},
   ];
   for(const view of views){
    const p=await browser.newPage({viewport:{width,height:820},locale:'zh-CN'});
    await p.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);await setup(p);
    await p.goto('http://rollbook.test'+view.url);await p.getByRole('button',{name:view.button,exact:true}).first().click();
    const container=p.locator(view.container).first();await container.waitFor({state:'visible'});
    const controls=container.locator('input:not([type=checkbox]):not([type=radio]),select,textarea');
    assert.ok(await controls.count()>0,view.url);
    for(let i=0;i<await controls.count();i++){
     const control=controls.nth(i);if(!await control.isVisible())continue;await control.focus();const item=await measure(control);
     assert.equal(item.outline,'0px',JSON.stringify({view,theme,width,item}));assert.equal(item.offset,'0px');assert.ok(item.shadow.includes('inset'),JSON.stringify({view,theme,width,item}));
     if(captureEvidence&&i===0&&theme==='light'&&width===1280){fs.mkdirSync(evidence,{recursive:true});await control.screenshot({path:path.join(evidence,`after-${view.url.slice(1).replaceAll('/','-')}-dialog.png`)});}
    }
    await p.close();
   }
   const auth=await browser.newPage({viewport:{width,height:820},locale:'zh-CN'});
   await auth.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);await setup(auth);
   await auth.goto('http://rollbook.test/settings/auth');await auth.getByRole('heading',{name:'认证设置'}).waitFor();
   for(const section of ['LDAP / LDAPS','通用 OIDC','飞书']){
    await auth.getByRole('button',{name:section,exact:true}).click();
    const controls=auth.locator('.auth-panel input:not([type=checkbox]):not([type=radio]),.auth-panel select,.auth-panel textarea');
    assert.ok(await controls.count()>0,section);
    for(let i=0;i<await controls.count();i++){
     const control=controls.nth(i);if(!await control.isVisible()||await control.isDisabled())continue;
     await control.scrollIntoViewIfNeeded();await control.focus();const item=await measure(control);if(!item.focus)continue;assert.equal(item.outline,'0px',JSON.stringify({section,theme,width,item}));assert.equal(item.offset,'0px');assert.ok(item.shadow.includes('inset'),JSON.stringify({section,theme,width,item}));
     if(captureEvidence&&i===0&&theme==='light'&&width===1280){fs.mkdirSync(evidence,{recursive:true});await control.screenshot({path:path.join(evidence,`after-auth-${section==='LDAP / LDAPS'?'ldap':section==='通用 OIDC'?'oidc':'feishu'}.png`)});}
    }
   }
   await auth.close();
  }
 }finally{await browser.close()}
});

test('筛选记录的状态与强制颜色模式保持同一 border-box',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const theme of ['light','dark'])for(const width of [1280,390]){
   const page=await browser.newPage({viewport:{width,height:820},locale:'zh-CN'});
   await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);await setup(page);
   await page.goto('http://rollbook.test/pending');const input=page.locator('.pending-toolbar .filter-fields input[type=search]');await input.waitFor();
   const idle=await measure(input);await input.hover();const hover=await measure(input);
   await page.keyboard.press('Tab');await input.focus();const focus=await measure(input);
   await input.fill('测试筛选');const typing=await measure(input);
   assert.equal(hover.border,idle.border);assert.equal(focus.border,idle.border);assert.equal(typing.border,idle.border);
   assert.equal(focus.rect.width,idle.rect.width);assert.equal(focus.rect.height,idle.rect.height);
   assert.equal(focus.outline,'0px');assert.equal(focus.offset,'0px');assert.ok(focus.shadow.includes('inset'));
   await input.evaluate(el=>el.setAttribute('aria-invalid','true'));const invalid=await measure(input);
   const danger=await page.evaluate(()=>{const el=document.createElement('div');el.style.color='var(--color-danger-text)';document.body.append(el);const color=getComputedStyle(el).color;el.remove();return color});
   assert.equal(invalid.borderColor,danger);
   assert.ok(invalid.shadow.includes('inset'));
   await input.evaluate(el=>el.removeAttribute('aria-invalid'));
   await page.emulateMedia({forcedColors:'active'});await input.focus();const forced=await measure(input);
   assert.equal(forced.outlineStyle,'solid');assert.equal(forced.outline,'2px');assert.equal(forced.offset,'-2px');
   await page.close();
  }
 }finally{await browser.close()}
});
