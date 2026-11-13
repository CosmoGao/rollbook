const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

const asset={
 'location:site':{id:'location:site',type:'location',name:'站点',status:'在用',rev:1,fields:{kind:'站点'}},
 'host:physical':{id:'host:physical',type:'host',name:'实体主机',status:'在用',rev:1,fields:{kind:'物理机',location_id:'location:site'}},
 'host:vm':{id:'host:vm',type:'host',name:'应用虚机',status:'在用',rev:1,fields:{kind:'虚拟机',location_id:'location:site',parent_host_id:'host:physical'}},
 'service:overleaf':{id:'service:overleaf',type:'service',name:'Overleaf',status:'在用',rev:1,fields:{host_id:'host:vm'}},
 'service:proxy':{id:'service:proxy',type:'service',name:'Proxy',status:'在用',rev:1,fields:{host_id:'host:physical'}},
 'endpoint:outside':{id:'endpoint:outside',type:'endpoint',name:'外网入口',status:'在用',rev:1,fields:{service_id:'service:overleaf',url:'https://example.org',via_json:['service:proxy']}}
};
const detail=ids=>ids.map(id=>asset[id]);
const edges=[{from:'endpoint:outside',to:'service:proxy',kind:'访问'},{from:'service:proxy',to:'service:overleaf',kind:'访问'},{from:'service:overleaf',to:'host:vm',kind:'宿主'},{from:'host:vm',to:'host:physical',kind:'父主机'}];
const topology=(center,view)=>{
 const ids=view==='access'?['endpoint:outside','service:proxy','service:overleaf','host:vm','host:physical']:
  center==='host:physical'?['host:physical','host:vm','service:proxy','service:overleaf','endpoint:outside']:['service:overleaf','service:proxy','host:vm','endpoint:outside'];
 return {center,nodes:ids,details:ids.map(id=>({id,name:asset[id].name,type:asset[id].type,status:asset[id].status,kind:asset[id].fields.kind})),edges:view==='access'?edges:edges.slice(1)};
};
test('主机和服务拓扑可绘制，访问链可切换，节点可打开详情',async()=>{
 const browser=await chromium.launch({headless:true});
 try {
  const page=await browser.newPage({locale:'zh-CN'});page.setDefaultTimeout(5000);
  await page.route('http://rollbook.test/**',route=>{
   const u=new URL(route.request().url()),p=decodeURIComponent(u.pathname);
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   const reply=json=>route.fulfill({json});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({allow_insecure_http:false,http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply(Object.values(asset).filter(o=>o.type===u.searchParams.get('type')));
   if(p==='/api/hosts/host:physical/inventory')return reply({hosts:['host:vm'],services:['service:proxy'],ips:[]});
   if(p==='/api/hosts/host:vm/inventory')return reply({hosts:[],services:['service:overleaf'],ips:[]});
   if(p==='/api/services/service:overleaf')return reply({service:asset['service:overleaf'],host:asset['host:vm'],ips:[],ports:[],endpoints:detail(['endpoint:outside']),dependencies:[],versions:[],credentials:[],notes:[],changes:[]});
   if(p==='/api/services/service:proxy')return reply({service:asset['service:proxy'],host:asset['host:physical'],ips:[],ports:[],endpoints:[],dependencies:[],versions:[],credentials:[],notes:[],changes:[]});
   if(p.endsWith('/topology'))return reply(topology(p.split('/')[3],u.searchParams.get('view')));
   if(p.startsWith('/api/assets/'))return reply(asset[p.slice('/api/assets/'.length)]);
   return route.fulfill({status:404,json:{message:'not found'}});
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#nav button[data-view="assets"]').click();await page.getByRole('tab',{name:'主机'}).click();await page.getByRole('group',{name:'主机视图'}).getByRole('button',{name:'树'}).click();
  await page.getByText('实体主机').first().click();
  await page.locator('.topology-canvas .react-flow').waitFor();
  assert.equal(await page.locator('.react-flow__node').count(),5);
  await page.locator('.react-flow__node').filter({hasText:'Overleaf'}).click();
  await page.getByRole('heading',{name:'Overleaf'}).waitFor();
  await page.getByRole('button',{name:'访问链'}).click();
  await page.locator('.react-flow__node').filter({hasText:'外网入口'}).waitFor();
  assert.equal(await page.locator('.react-flow__edge').count(),4);
  await page.locator('.react-flow__node').filter({hasText:'应用虚机'}).click();
  await page.getByRole('heading',{name:'应用虚机'}).waitFor();
 } finally {await browser.close()}
});
