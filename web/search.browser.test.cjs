const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('IP 搜索首卡展示站点、宿主、服务及安全入口，页内顶栏同步',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:1280,height:720}});page.setDefaultTimeout(5000);
  const hits=[{id:'ip:immich',type:'ip',name:'172.20.2.22',status:'在用',site_id:'location:beijing',site_name:'北京家',host_id:'host:immich',host_name:'Immich 虚拟机',primary_ip:'172.20.2.22',services:['Immich'],endpoints:['https://photos.example.invalid/']},{id:'endpoint:bad',type:'endpoint',name:'危险入口',status:'在用',endpoints:[]}];
  await page.route('http://rollbook.test/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname),reply=json=>route.fulfill({json});
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({allow_insecure_http:false,http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply([{id:'location:beijing',type:'location',name:'北京家',fields:{kind:'站点'}}]);
   if(p==='/api/search')return reply(hits);
   return reply([]);
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.keyboard.press('Control+k');await page.locator('.search-box input').fill('172.20.2.22');
  await page.locator('.search-answer').first().getByText('北京家').waitFor();
  const first=page.locator('.search-answer').first();assert.match(await first.innerText(),/Immich 虚拟机.*Immich.*photos\.example\.invalid/s);
  assert.equal(await page.locator('.top-search input').inputValue(),'172.20.2.22');
  assert.equal(await page.locator('a[href="https://photos.example.invalid/"]').count(),1);
  assert.equal(await page.locator('a[href^="javascript:"]').count(),0);
  assert.equal(await page.getByText('试试搜索').count(),0);
  if(process.env.ROLLBOOK_SCREENSHOT_DIR){fs.mkdirSync(process.env.ROLLBOOK_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,'v2ri-64-search-ip.png')})}
 }finally{await browser.close()}
});
