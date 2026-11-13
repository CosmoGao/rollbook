const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('服务首屏展示访问链、宿主层级、影响面与聚焦契约',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({viewport:{width:1280,height:720},locale:'zh-CN'});page.setDefaultTimeout(5000);
  const assets={
   'location:site':{id:'location:site',type:'location',name:'北京家',rev:1,fields:{kind:'站点'}},
   'location:rack':{id:'location:rack',type:'location',name:'主卧机柜',rev:1,fields:{kind:'机柜',parent_id:'location:site'}},
   'host:proxmox':{id:'host:proxmox',type:'host',name:'Proxmox VE',rev:1,fields:{location_id:'location:rack',kind:'物理机'}},
   'host:vm':{id:'host:vm',type:'host',name:'Immich 主机',rev:1,fields:{location_id:'location:rack',parent_host_id:'host:proxmox',kind:'虚拟机'}},
   'service:app':{id:'service:app',type:'service',name:'Immich',rev:1,fields:{host_id:'host:vm'}},
   'service:cloudflare':{id:'service:cloudflare',type:'service',name:'Cloudflare Tunnel',rev:1,fields:{host_id:'host:proxmox'}},
   'service:zoraxy':{id:'service:zoraxy',type:'service',name:'Zoraxy',rev:1,fields:{host_id:'host:proxmox'}},
  };
  const endpoint={id:'endpoint:web',type:'endpoint',name:'照片入口',fields:{service_id:'service:app',url:'https://photos.invalid',via_json:['service:cloudflare','service:zoraxy']}};
  const external={id:'service:external',type:'service',name:'外部消费者',fields:{host_id:'host:elsewhere'}};
  const hostImpact={root:assets['host:proxmox'],hosts:[{object:assets['host:vm'],path:['host:proxmox','host:vm'],source:'parent_host_id'}],services:[{object:assets['service:app'],path:['host:proxmox','host:vm','service:app'],source:'host_id'},{object:external,path:['host:proxmox','host:vm','service:app','service:external'],source:'dependency'}],endpoints:[{object:endpoint,path:['host:proxmox','host:vm','service:app','endpoint:web'],source:'service_id'}]};
  const impact={root:assets['service:app'],hosts:[],services:[{object:external,path:['service:app','service:external'],source:'dependency'}],endpoints:[{object:endpoint,path:['service:app','endpoint:web'],source:'service_id'}]};
  await page.route('http://rollbook.test/**',route=>{
   const url=new URL(route.request().url()),p=decodeURIComponent(url.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   const reply=json=>route.fulfill({json});
   if(p==='/api/auth/session')return reply({login:'owner'});if(p==='/api/config')return reply({http_risk:false});if(p==='/api/version')return reply({version:'test'});
   if(p==='/api/browse')return reply(Object.values(assets).filter(a=>a.type===url.searchParams.get('type')));
   if(p==='/api/changes'||p==='/api/assets/unverified'||p==='/api/versions/outdated')return reply([]);
   if(p==='/api/assets/host:proxmox/detail')return reply({object:assets['host:proxmox'],notes:[],credentials:[],changes:[],referenced_by:[]});
   if(p==='/api/assets/service:app/detail')return reply({object:assets['service:app'],notes:[],credentials:[],changes:[],referenced_by:[]});
   if(p.startsWith('/api/assets/'))return reply(assets[p.slice('/api/assets/'.length)]);
   if(p==='/api/hosts/host:proxmox/inventory')return reply({hosts:[],services:[],ips:[]});
   if(p==='/api/hosts/host:proxmox/impact')return reply(hostImpact);
   if(p==='/api/hosts/host:proxmox/offline')return reply({preview:{host:assets['host:proxmox'],services:[],ips:[],endpoints:[],impact:hostImpact},operation_id:''});
   if(p==='/api/services/service:app')return reply({service:assets['service:app'],host:assets['host:vm'],ips:[],ports:[],endpoints:[endpoint],dependencies:[],versions:[]});
   if(p==='/api/services/service:app/impact')return reply(impact);
   if(p==='/api/hosts/host:proxmox/topology')return reply({center:'host:proxmox',nodes:['host:proxmox'],edges:[],details:[{id:'host:proxmox',name:'Proxmox VE',type:'host'}]});
   if(p==='/api/services/service:app/topology')return reply({center:'service:app',nodes:['service:app'],edges:[],details:[{id:'service:app',name:'Immich',type:'service'}]});
   return route.fulfill({status:404,json:{message:'unexpected '+p}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#nav button[data-view="assets"]').click();await page.getByRole('tab',{name:'服务'}).click();await page.locator('.card').filter({hasText:'Immich'}).click();
  await page.getByRole('heading',{name:'影响面'}).waitFor();
  await page.getByRole('heading',{name:'本服务停机'}).waitFor();await page.getByRole('heading',{name:/上层物理机停机.*Proxmox VE/}).waitFor();assert.match(await page.locator('.impact-scenarios').innerText(),/Immich 主机/);assert.match(await page.locator('.impact-scenarios').innerText(),/照片入口/);assert.match(await page.locator('.impact-scenario').first().innerText(),/外部消费者/);assert.match(await page.locator('.impact-scenario').nth(1).innerText(),/服务中断：1/);assert.match(await page.locator('.impact-scenario').nth(1).innerText(),/外部依赖需评估：外部消费者/);
  assert.ok((await page.getByText('Cloudflare Tunnel → Zoraxy').count())>=1);
  assert.equal(await page.locator('.detail-breadcrumb button').filter({hasText:'Proxmox VE'}).count(),1);
  assert.equal(await page.locator('.audit-details').first().evaluate(el=>el.open),false);
  const focus=page.getByRole('button',{name:'在拓扑中聚焦'});assert.ok((await focus.boundingBox()).y<720);
  fs.mkdirSync(path.join(__dirname,'screenshots'),{recursive:true});await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-62-service-detail.png')});
  await focus.click();assert.match(page.url(),/\/topology\?focus=service%3Aapp&view=impact/);
  await page.locator('#nav button[data-view=assets]').click();await page.getByRole('tab',{name:'主机'}).click();await page.locator('.card').filter({hasText:'Proxmox VE'}).click();await page.getByRole('heading',{name:'影响面'}).waitFor();assert.match(await page.locator('.impact-hosts').innerText(),/Immich 主机/);assert.match(await page.locator('.impact-hosts').innerText(),/Immich/);await page.screenshot({path:path.join(__dirname,'screenshots','v2ri-62-host-impact.png')});await page.getByRole('button',{name:'停用主机'}).click();assert.match(await page.locator('.modal').innerText(),/Immich/);
 }finally{await browser.close()}
});
