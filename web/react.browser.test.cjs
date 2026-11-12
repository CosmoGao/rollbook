const { test } = require('node:test');
const assert = require('node:assert/strict');
const { chromium } = require('playwright');
const fs = require('node:fs');
const path = require('node:path');
const mime = {'.html':'text/html','.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png','.webmanifest':'application/manifest+json'};
const dir=path.join(__dirname,'dist');

test('React 六项菜单、搜索快捷键及移动布局', async t=>{
  const browser=await chromium.launch({headless:true});
  t.after(()=>browser.close());
  for(const viewport of [{width:1280,height:800},{width:390,height:844}]){
    const page=await browser.newPage({viewport});
    await page.addInitScript(() => localStorage.setItem('rollbook.language','zh-CN'));
    const errors=[];page.on('pageerror',error=>errors.push(error.message));
    await page.route('**/*',async route=>{
      const url=new URL(route.request().url());
      if(url.pathname.startsWith('/api/')){
        let body=[];
        if(url.pathname==='/api/auth/session'||url.pathname==='/api/auth/login')body={login:'admin'};
        if(url.pathname==='/api/config')body={http_risk:false};
        if(url.pathname==='/api/version')body={version:'v0.4.2',commit:'235392f'};
        if(url.pathname==='/api/versions/alerts')body={enabled:false};
        return route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
      }
      const file=path.resolve(dir,'.'+(url.pathname==='/'?'/index.html':url.pathname));
      if(!file.startsWith(dir+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:'not found'});
      return route.fulfill({status:200,contentType:mime[path.extname(file)]||'application/octet-stream',body:fs.readFileSync(file)});
    });
    await page.goto('http://rollbook.test/');
    await page.locator('#app').waitFor({state:'visible'});
    await page.locator('.modal-backdrop').waitFor({state:'visible'});
    await page.getByRole('button',{name:'稍后再说'}).click();
    assert.equal(await page.locator('#nav button').count(),6);
    assert.match(await page.locator('#build-version').innerText(),/v0.4.2/);
    if(viewport.width>760){
      await page.keyboard.press('Control+k');
      await page.locator('.search-box input').waitFor({state:'visible'});
      await page.locator('#nav button[data-view="topology"]').click();
      await page.locator('.flow-container').waitFor({state:'visible'});
      await page.locator('#nav button[data-view="agent"]').click();
      await page.getByRole('heading',{name:'Key 管理'}).waitFor();
    } else {
      await page.getByRole('button',{name:'打开菜单'}).click();
      await page.locator('.mobile-drawer button').filter({hasText:'资产'}).click();
      await page.getByRole('heading',{name:'资产维护'}).waitFor();
    }
    assert.deepEqual(errors,[]);
    await page.close();
  }
});

test('资产、拓扑、待处理、Key 与设置在新构建中可操作',async t=>{
  const browser=await chromium.launch({headless:true});t.after(()=>browser.close());
  const page=await browser.newPage({viewport:{width:1280,height:800}});
  await page.addInitScript(() => localStorage.setItem('rollbook.language','zh-CN'));
  const site={id:'location:test',type:'location',name:'测试站点',status:'在用',rev:1,fields:{kind:'站点'}};
  const host={id:'host:test',type:'host',name:'测试主机',status:'在用',rev:1,fields:{kind:'虚拟机',location_id:site.id}};
  const service={id:'service:test',type:'service',name:'测试服务',status:'在用',rev:1,fields:{host_id:host.id}};
  const assets=[site,host,service];const writes=[];
  await page.route('**/*',async route=>{
    const request=route.request(),url=new URL(request.url()),p=decodeURIComponent(url.pathname);
    const reply=body=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(body)});
    if(p.startsWith('/api/')){
      if(p==='/api/config')return reply({http_risk:false});
      if(p==='/api/version')return reply({version:'v0.4.2',commit:'235392f'});
      if(p==='/api/auth/session')return reply({login:'owner'});
      if(p==='/api/changes/proposals')return reply([]);
      if(p==='/api/assets/unverified')return reply([]);
      if(p==='/api/versions/outdated')return reply([]);
      if(p==='/api/changes'||p==='/api/contexts')return reply([]);
      if(p==='/api/browse')return reply(assets.filter(x=>x.type===url.searchParams.get('type')));
      if(p==='/api/versions/alerts')return reply({enabled:false});
      if(p==='/api/keys'&&request.method()==='GET')return reply([]);
      if(p==='/api/keys'&&request.method()==='POST'){writes.push(JSON.parse(request.postData()));return reply({token:'rb_once_secret'})}
      if(p==='/api/backup/files'||p==='/api/backup/imports')return reply([]);
      if(p==='/api/assets'&&request.method()==='POST'){const payload=JSON.parse(request.postData());writes.push(payload);assets.push({...payload.object,rev:1});return reply(payload.object)}
      if(p.startsWith('/api/assets/')&&p.endsWith('/detail'))return reply({object:assets.find(x=>x.id===decodeURIComponent(p.split('/')[3])),notes:[],credentials:[],changes:[],referenced_by:[]});
      if(p.startsWith('/api/assets/'))return reply(assets.find(x=>x.id===decodeURIComponent(p.split('/')[3]))||{});
      if(p==='/api/sites/location:test')return reply({id:site.id,name:site.name,uplinks:[],networks:[],routers:[],hosts:[host.id]});
      if(p==='/api/hosts/host:test/inventory')return reply({hosts:[],services:[service.id],ips:[]});
      if(p==='/api/hosts/host:test/topology'||p==='/api/services/service:test/topology')return reply({center:service.id,details:[{id:host.id,name:host.name,type:'host'},{id:service.id,name:service.name,type:'service'}],edges:[{from:host.id,to:service.id,kind:'hosts'}]});
      if(p==='/api/services/service:test')return reply({service,host,ips:[],ports:[],endpoints:[],dependencies:[],versions:[]});
      return route.fulfill({status:404,contentType:'application/json',body:'{}'});
    }
    const file=path.resolve(dir,'.'+(p==='/'?'/index.html':p));
    if(!file.startsWith(dir+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:'not found'});
    return route.fulfill({status:200,contentType:mime[path.extname(file)]||'application/octet-stream',body:fs.readFileSync(file)});
  });
  await page.goto('http://rollbook.test/');
  await page.locator('.overview-grid').waitFor();
  await page.locator('#nav button[data-view="assets"]').click();
  await page.getByRole('heading',{name:'资产维护'}).waitFor();
  await page.locator('.asset-type-select').selectOption('host');
  await page.getByText('测试主机').first().waitFor();
  await page.locator('#nav button[data-view="topology"]').click();
  await page.locator('.flow-container .react-flow__node').first().waitFor({timeout:5000});
  await page.locator('#nav button[data-view="pendingHub"]').click();
  await page.getByRole('heading',{name:'待处理'}).waitFor();
  await page.locator('#nav button[data-view="agent"]').click();
  await page.getByRole('button',{name:'创建 Key'}).click();
  await page.locator('.modal input').first().fill('测试 Key');
  await page.locator('.modal button[type="submit"]').click();
  await page.getByText('rb_once_secret',{exact:true}).waitFor();
  assert.equal(writes[0].name,'测试 Key');
  await page.getByRole('button',{name:'完成'}).click();
  await page.locator('#nav button[data-view="settings"]').click();
  await page.getByRole('heading',{name:'设置'}).waitFor();
  await page.getByRole('button',{name:'备份与恢复'}).click();
  await page.getByRole('heading',{name:'备份与恢复'}).waitFor();
  await page.close();
});

test('未登录的英文登录页显示 HTTP 风险和构建版本',async t=>{
  const browser=await chromium.launch({headless:true});t.after(()=>browser.close());
  const page=await browser.newPage({viewport:{width:390,height:844},locale:'en-US'});
  await page.route('**/*',route=>{
    const p=new URL(route.request().url()).pathname;
    if(p==='/api/auth/session')return route.fulfill({status:401,contentType:'application/json',body:'{"message":"Authentication required"}'});
    if(p==='/api/config')return route.fulfill({json:{http_risk:true}});
    if(p==='/api/version')return route.fulfill({json:{version:'v0.4.2',commit:'235392f'}});
    if(p.startsWith('/api/'))return route.fulfill({status:404,json:{}});
    const file=path.resolve(dir,'.'+(p==='/'?'/index.html':p));
    if(!file.startsWith(dir+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:''});
    return route.fulfill({status:200,contentType:mime[path.extname(file)]||'application/octet-stream',body:fs.readFileSync(file)});
  });
  await page.goto('http://rollbook.test/');
  await page.getByRole('heading',{name:'Sign in to Rollbook'}).waitFor();
  assert.match(await page.locator('#login .http-mode-notice').innerText(),/HTTP mode/);
  assert.equal(await page.locator('#login-build-version').innerText(),'Rollbook v0.4.2');
});
