const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const dist=path.join(__dirname,'dist');
const screenshotDir=process.env.ROLLBOOK_SCREENSHOT_DIR;
const mime={'.html':'text/html','.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png'};
const location=(id,name,kind,parent_id)=>({id,type:'location',name,fields:{kind,parent_id}});
const host=(id,name,location_id)=>({id,type:'host',name,fields:{kind:'物理机',location_id}});
const site=location('location:beijing','北京家','站点');
const rack=location('location:rack','主卧机柜','机柜',site.id);
const assets=[site,rack,location('location:xianghe','香河家','站点'),host('host:pve','Proxmox VE',rack.id),{id:'service:immich',type:'service',name:'Immich',fields:{host_id:'host:pve'}},{id:'uplink:beijing',type:'uplink',name:'联通光纤',fields:{location_id:site.id,public_ipv4:'动态公网'}}];
const proposal={id:'change:1',type:'change',name:'提议',fields:{object_type:'host',object_id:'host:pve',action:'update',actor:'agent',reason:'升级',base_rev:3,current_rev:4,base_object:{name:'Proxmox VE',fields:{os_version:'8.3'}},diff_json:JSON.stringify({fields:{os_version:'8.4'}})}};
async function setup(page){
 await page.route('**/*',route=>{
  const request=route.request(),url=new URL(request.url()),pathname=url.pathname;
  const json=value=>route.fulfill({json:value});
  if(pathname.startsWith('/api/')){
   if(pathname==='/api/auth/session')return json({login:'demo'});
   if(pathname==='/api/config')return json({http_risk:false});
   if(pathname==='/api/version')return json({version:'v0.5.1'});
   if(pathname==='/api/browse')return json(assets.filter(item=>item.type===url.searchParams.get('type')));
   if(pathname==='/api/changes')return json(url.searchParams.has('state')?[proposal]:[proposal]);
   if(pathname==='/api/assets/unverified')return json([site,assets[3],assets[4]]);
   if(pathname==='/api/versions/outdated')return json([]);
   if(pathname==='/api/versions/alerts')return json({enabled:false});
   if(pathname==='/api/search')return json([]);
   return json([]);
  }
  const file=path.resolve(dist,'.'+(path.extname(pathname)?pathname:'/index.html'));
  if(!file.startsWith(dist+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:''});
  return route.fulfill({contentType:mime[path.extname(file)]||'application/octet-stream',body:fs.readFileSync(file)});
 });
 await page.goto('http://rollbook.test/');
 await page.getByRole('heading',{name:'需要你处理'}).waitFor();
}
test('总览任务、角标、提议差异和搜索导航',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({viewport:{width:1280,height:720},locale:'zh-CN',colorScheme:'light'});
  await setup(page);
  await page.getByRole('button',{name:'待核实对象 3'}).waitFor();
  assert.equal((await page.locator('.nav-badge').innerText()).trim(),'4');
  assert.match(await page.locator('.overview-site').first().innerText(),/北京家[\s\S]*1 台主机[\s\S]*1 个服务/);
  assert.match(await page.locator('.overview-site').first().innerText(),/入口: 联通光纤 · 动态公网/);
  if(screenshotDir){fs.mkdirSync(screenshotDir,{recursive:true});await page.screenshot({path:path.join(screenshotDir,'v2ri-63-overview-1280x720.png'),animations:'disabled'});}
  await page.locator('.overview-task-row').first().click();
  await page.getByText('系统版本: 8.3 → 8.4').waitFor();
  assert.equal(await page.locator('pre.diff').count(),0);
  await page.locator('.sidebar #nav button[data-view=overview]').click();
  await page.locator('.overview-site').first().getByRole('button',{name:'看拓扑'}).click();
  assert.equal(new URL(page.url()).searchParams.get('site'),site.id);
  await page.getByRole('combobox',{name:'站点'}).locator('option[value="location:beijing"]').waitFor({state:'attached'});
  assert.equal(await page.getByRole('combobox',{name:'站点'}).inputValue(),site.id);
  await page.locator('.sidebar #nav button[data-view=overview]').click();
  await page.getByRole('button',{name:'待核实对象 3'}).click();
  assert.equal(new URL(page.url()).searchParams.get('tab'),'unverified');
  assert.equal(await page.locator('.row .check-row').count(),3);
  await page.locator('.sidebar #nav button[data-view=overview]').click();
  await page.locator('.overview-search input').fill('Immich');
  await page.locator('.overview-search button').click();
  assert.equal(new URL(page.url()).pathname,'/search');
  await page.close();
 }finally{await browser.close()}
});
