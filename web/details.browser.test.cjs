const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('四类详情安全显示笔记、凭据与引用；变更筛选和共享上下文可用',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN'});page.setDefaultTimeout(5000);
  const ids=['host:box','network:lan','service:app','endpoint:web'];
  const types=['host','network','service','endpoint'];
  const assets=Object.fromEntries(ids.map((id,i)=>[id,{id,type:types[i],name:id,rev:1,status:'在用',fields:{kind:i===0?'物理机':undefined,host_id:i===2?'host:box':undefined,url:i===3?'https://example.invalid':undefined}}]));
  const note={id:'note:one',type:'note',name:'测试笔记',fields:{kind:'运维笔记',body_md:'**安全** <script>window.__pwned=true</script> [危险](javascript:alert(1)) [文档](https://example.invalid)',author:'agent'},created_at:'2026-11-12T10:00:00Z'};
  const credential={id:'credential:one',type:'credential',name:'引用',fields:{vault:'vault',item_ref:'item',purpose:'访问'}};
  const change={id:'change:one',type:'change',name:'变更',created_at:'2026-11-12T10:00:00Z',fields:{action:'update',actor:'agent',actor_key_name:'robot',object_id:'host:box',object_type:'host',reason:'更新'}};
  const context={id:'context:one',type:'context',created_at:'2026-11-12T10:00:00Z',source:'agent',fields:{key:'handoff',scope:'项目',owner:'agent',body_md:'**交接** <img src=x onerror=window.__pwned=true>'}};
  let query;
  await page.route('http://rollbook.test/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   const reply=json=>route.fulfill({json});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({allow_insecure_http:false,http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply(Object.values(assets).filter(x=>x.type===u.searchParams.get('type')));
   if(p==='/api/changes/proposals')return reply([]);if(p==='/api/assets/unverified')return reply([]);if(p==='/api/versions/outdated')return reply([]);if(p==='/api/changes'){query=u.searchParams;return reply([change])}
   if(p==='/api/contexts')return reply([context]);
   if(p.startsWith('/api/assets/')&&p.endsWith('/detail')){const id=p.slice('/api/assets/'.length,-'/detail'.length);return reply({object:assets[id],notes:[note],credentials:[credential],changes:[change],referenced_by:[assets['service:app']]})}
   if(p.startsWith('/api/assets/'))return reply(assets[p.slice('/api/assets/'.length)]||credential);
   if(p==='/api/services/service:app')return reply({service:assets['service:app'],host:assets['host:box'],ips:[],ports:[],endpoints:[],dependencies:[],versions:[]});
   if(p.startsWith('/api/hosts/')||p.startsWith('/api/networks/'))return reply({ips:[],hosts:[],services:[]});
   if(p==='/api/services/service:app/topology')return reply({nodes:[],edges:[]});
   return route.fulfill({status:404,json:{message:'unexpected '+p}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.getByRole('button',{name:'稍后再说'}).click();
  for(const id of ids){
   await page.locator('#nav button[data-view="assets"]').click();await page.locator('.asset-type-select').selectOption(id.split(':')[0]);await page.locator('.card').filter({hasText:id}).click();
   await page.getByRole('heading',{name:'笔记'}).waitFor();
   assert.equal(await page.locator('.detail-entry strong').first().count(),1);
   assert.equal(await page.getByRole('heading',{name:'凭据引用'}).count(),1);
   assert.equal(await page.getByRole('heading',{name:'反向引用'}).count(),1);
   assert.equal(await page.locator('.markdown strong').count(),1);
   assert.equal(await page.locator('.markdown script').count(),0);
   assert.equal(await page.locator('a[href^="javascript:"]').count(),0);
   assert.equal(await page.evaluate(()=>window.__pwned),undefined);
  }
  await page.locator('#nav [data-view=overview]').click();await page.locator('.section').filter({has:page.getByRole('heading',{name:'最近变更'})}).getByRole('button',{name:'查看全部'}).click();
  await page.locator('.filter-bar input').first().fill('robot');
  await page.locator('.filter-bar select').selectOption('host');
  await page.locator('.filter-bar input[type=date]').first().fill('2026-11-01');
  await page.locator('.filter-bar button').click();
  await page.getByText('update · host:box').waitFor();
  assert.equal(query.get('key'),'robot');assert.equal(query.get('object_type'),'host');assert.equal(query.get('since'),new Date('2026-11-01T00:00:00').toISOString());
  await page.locator('#nav [data-view=overview]').click();await page.locator('.section').filter({has:page.getByRole('heading',{name:'共享上下文'})}).getByRole('button',{name:'查看全部'}).click();
  await page.getByText('交接').waitFor();
  assert.equal(await page.locator('.markdown img').count(),0);
  assert.equal(await page.evaluate(()=>window.__pwned),undefined);
 }finally{await browser.close()}
});
