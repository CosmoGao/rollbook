const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('站点卡片保留导航和独立展开，宽窄屏与两种主题下可读',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const long='很长的站点名称'.repeat(12);
  const assets=[
   {id:'location:beijing',type:'location',name:'北京',fields:{kind:'站点'}},
   {id:'location:room',type:'location',name:long,fields:{kind:'房间',parent_id:'location:beijing'}},
   {id:'location:xianghe',type:'location',name:'香河',fields:{kind:'站点'}}
  ];
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:1440,height:900}});page.setDefaultTimeout(5000);
  await page.route('http://rollbook.test/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   const reply=json=>route.fulfill({json});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({allow_insecure_http:false,http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply(assets.filter(a=>a.type===u.searchParams.get('type')));
   if(p.endsWith('/detail'))return reply({notes:[],credentials:[],changes:[],referenced_by:[]});
   if(p.startsWith('/api/sites/'))return reply({uplinks:[],networks:[],routers:[],hosts:[]});
   if(p.startsWith('/api/assets/'))return reply(assets.find(a=>a.id===p.slice('/api/assets/'.length)));
   return route.fulfill({status:404,json:{message:'not found'}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#nav button[data-view="assets"]').click();
  const cards=page.locator('.asset-location-grid > .asset-location-node');
  await cards.first().waitFor();assert.equal(await cards.count(),2);
  const first=cards.first(),second=cards.nth(1);
  const left=await first.boundingBox(),right=await second.boundingBox();
  assert.ok(left&&right&&right.x-left.x<320&&left.width<=280);
  assert.equal(await first.getByRole('button',{name:'展开北京'}).count(),0);
  await first.getByRole('button',{name:'收起北京'}).click();
  assert.equal(await first.getByRole('button',{name:long}).isVisible(),false);
  assert.equal(await cards.count(),2);
  await first.getByRole('button',{name:'展开北京'}).focus();await page.keyboard.press('Enter');
  assert.equal(await first.getByRole('button',{name:long}).isVisible(),true);
  const longCard=first.locator('.asset-location-card').nth(1),box=await longCard.boundingBox();
  assert.ok(box&&box.width<=280);
  assert.equal(await longCard.evaluate(el=>el.scrollWidth<=el.clientWidth),true);
  for(const theme of ['light','dark']){
   await page.evaluate(value=>document.documentElement.setAttribute('data-theme',value),theme);
   assert.equal(await first.locator('.asset-location-open').first().evaluate(el=>getComputedStyle(el).textAlign),'left');
  }
  await page.setViewportSize({width:390,height:844});
  const narrowFirst=await first.boundingBox(),narrowSecond=await second.boundingBox();
  assert.ok(narrowFirst&&narrowSecond&&narrowSecond.y>narrowFirst.y);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
  await second.getByRole('button',{name:'香河'}).click();
  await page.getByRole('button',{name:'编辑'}).waitFor();
  for(let i=0;i<24;i++)assets.push({id:'location:extra'+i,type:'location',name:'新增站点 '+i,fields:{kind:'站点'}});
  await page.setViewportSize({width:1440,height:900});
  await page.locator('#nav button[data-view="assets"]').click();
  await page.waitForFunction(()=>document.querySelectorAll('.asset-location-grid > .asset-location-node').length===26);
  for(const width of [1024,768,390,320]){
   await page.setViewportSize({width,height:844});
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`横向溢出：${width}px`);
   if(width>=768)assert.equal(await cards.last().evaluate(el=>el.getBoundingClientRect().width<=280),true);
  }
 }finally{await browser.close()}
});
