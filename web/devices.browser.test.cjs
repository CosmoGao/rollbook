const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('设备录入、检索、详情与拓扑覆盖',async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:1365,height:900}});page.setDefaultTimeout(8000);
  const sites=[{id:'location:home',type:'location',name:'家',kind:'站点',site_id:'location:home',site_name:'家',fields:{kind:'站点'}},{id:'location:room',type:'location',name:'客厅',kind:'房间',site_id:'location:home',site_name:'家',fields:{kind:'房间',parent_id:'location:home'}},{id:'location:bedroom',type:'location',name:'卧室',kind:'房间',site_id:'location:home',site_name:'家',fields:{kind:'房间',parent_id:'location:home'}},{id:'location:cloud',type:'location',name:'云区域',kind:'站点',site_id:'location:cloud',site_name:'云区域',fields:{kind:'站点'}}];
  const hosts=[
   ['tv','客厅电视','智能电视','Example 55','location:room'],['ps5','游戏机','游戏机','Sony PlayStation 5','location:room'],['switch','Switch','游戏机','Nintendo Switch','location:room'],['hub','智能网关','智能家居中枢与网关','Example Hub','location:room'],['sensor','门磁','传感器与控制器','Example Sensor','location:room'],['printer','打印机','打印与扫描设备','Example Printer','location:room'],['nas','NAS','NAS','Example NAS','location:home'],['router','路由器','网络设备','Example Router','location:home'],['vm','独立云 VM','虚拟机','Example VM','location:cloud'],['player','播放器','机顶盒与播放器','Example Player','location:room'],['speaker','音箱','音箱与音频设备','Example Speaker','location:bedroom'],['camera','摄像头','摄像头与门铃','Example Camera','location:bedroom'],['vacuum','扫地机','家电','Example Vacuum','location:home'],['other','其他设备','其他设备','Example Other','location:home'],['server','服务器','物理机','Example Server','location:home'],['guest','部署 VM','虚拟机','Example VM','location:home','host:server'],['container','容器实例','容器','Example Container','location:home','host:guest']
  ].map(([id,name,kind,model,location_id,parent_host_id])=>({id:'host:'+id,type:'host',name,kind,site_id:location_id==='location:cloud'?'location:cloud':'location:home',site_name:location_id==='location:cloud'?'云区域':'家',fields:{kind,model,location_id,...(parent_host_id?{parent_host_id}:{})},aliases:id==='ps5'?['PS5']:[],tags:id==='sensor'?['Zigbee']:[],rev:1,status:'在用'}));
  const services=[...([['print','打印服务','printer'],['dns','DNS','router'],['media','媒体服务','nas'],['app','容器服务','container']].map(([id,name,host])=>({id:'service:'+id,type:'service',name,fields:{host_id:'host:'+host},rev:1,status:'在用'})))];
  const all=()=>[...sites,...hosts,...services];let created;
  await page.route('http://rollbook.test/**',route=>{
   const req=route.request(),u=new URL(req.url()),p=decodeURIComponent(u.pathname),reply=json=>route.fulfill({json});
   if(p==='/'||p==='/topology')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/auth/session')return reply({login:'owner'});if(p==='/api/config')return reply({http_risk:false});if(p==='/api/version')return reply({version:'test'});
   if(p==='/api/browse')return reply(all().filter(item=>item.type===u.searchParams.get('type')));
   if(p==='/api/search'){const q=(u.searchParams.get('q')||'').toLowerCase();return reply(all().filter(item=>[item.id,item.name,item.kind,item.fields?.model,...item.aliases||[],...item.tags||[]].some(value=>String(value||'').toLowerCase().includes(q))))}
   if(p==='/api/assets'&&req.method()==='POST'){created=JSON.parse(req.postData()).object;created.rev=1;created.kind=created.fields.kind;created.site_id='location:home';created.site_name='家';hosts.push(created);return reply(created)}
   if(p.startsWith('/api/assets/')&&p.endsWith('/detail'))return reply({object:all().find(item=>item.id===p.split('/')[3]),notes:[],credentials:[],changes:[],referenced_by:[]});
   if(p.startsWith('/api/assets/'))return reply(all().find(item=>item.id===p.slice('/api/assets/'.length))||{});
   if(p.startsWith('/api/hosts/')&&p.endsWith('/inventory'))return reply({ips:[],services:services.filter(item=>item.fields.host_id===p.split('/')[3]).map(item=>item.id)});
   if(p.startsWith('/api/hosts/')&&p.endsWith('/impact'))return reply({root:all().find(item=>item.id===p.split('/')[3]),hosts:[],services:[],endpoints:[]});
   if(p.startsWith('/api/hosts/')&&p.endsWith('/topology'))return reply({nodes:[],details:[],edges:[]});
   if(p==='/api/topology/layers')return reply({nodes:[...sites.map(item=>({id:item.id,name:item.name,type:'location',kind:item.kind,parent_id:item.fields.parent_id})),...hosts.map(item=>({id:item.id,name:item.name,type:'host',kind:item.kind,location_id:item.fields.location_id,parent_id:item.fields.parent_host_id})),...services.map(item=>({id:item.id,name:item.name,type:'service',host_id:item.fields.host_id}))],edges:[{from:'host:sensor',to:'host:hub',kind:'依赖'}]});
   return reply([]);
  });
  await installBundle(page);await page.goto('http://rollbook.test/');await page.locator('#nav button[data-view="assets"]').click();await page.getByRole('tab',{name:'设备与主机'}).click();
  for(const value of ['智能电视','游戏机','机顶盒与播放器','音箱与音频设备','摄像头与门铃','打印与扫描设备','智能家居中枢与网关','家电','传感器与控制器','其他设备']){await page.getByLabel('设备类别').selectOption(value);await page.locator('tbody tr').filter({hasText:value}).first().waitFor();assert.equal(await page.locator('tbody tr').count(),value==='游戏机'?2:1)}
  await page.getByLabel('设备类别').selectOption('');await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-68-devices.png'),fullPage:true});
  await page.getByRole('button',{name:'新建'}).click();const form=page.locator('.asset-form');assert.equal(await form.getByRole('combobox',{name:'类别 *'}).inputValue(),'');
  await form.getByRole('textbox',{name:'名称'}).fill('Xbox');await form.getByRole('combobox',{name:'类别 *'}).selectOption('游戏机');await form.getByRole('textbox',{name:'型号'}).fill('Microsoft Xbox Series X');await form.getByRole('textbox',{name:'别名'}).fill('Series X');await form.getByRole('textbox',{name:'别名'}).press('Enter');await form.getByRole('textbox',{name:'操作原因 *'}).fill('登记');await form.getByRole('button',{name:'创建'}).click();
  assert.equal(created.fields.kind,'游戏机');assert.deepEqual(created.aliases,['Series X']);await page.getByRole('heading',{name:'Xbox'}).waitFor();await page.getByText('未登记 IP').first().waitFor();await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-68-game-detail.png')});
  await page.locator('#nav button[data-view="assets"]').click();await page.getByRole('tab',{name:'设备与主机'}).click();await page.getByLabel('搜索资产').fill('PlayStation');await page.locator('tbody tr').filter({hasText:'游戏机'}).waitFor();assert.equal(await page.locator('tbody tr').count(),1);
  await page.locator('#nav button[data-view="topology"]').click();await page.locator('.topology-site-card').first().waitFor();await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-68-topology-default.png')});await page.getByLabel('站点',{exact:true}).selectOption('location:home');await page.locator('.topology-host-card > button').filter({hasText:'客厅电视'}).waitFor();await page.locator('.topology-host-card > button').filter({hasText:'打印机'}).click();await page.locator('.topology-host-card').filter({hasText:'打印机'}).getByText('打印服务').waitFor();await page.locator('.topology-host-card').filter({hasText:'打印机'}).evaluate(element=>element.scrollIntoView({block:'center'}));await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-68-topology-expanded.png')});await page.getByLabel('聚焦对象').selectOption('host:sensor');await page.locator('.topology-flow .react-flow__edge-text').filter({hasText:'依赖'}).waitFor();
 }finally{await browser.close()}
});
