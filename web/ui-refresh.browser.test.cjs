const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const dist=process.env.ROLLBOOK_DIST_DIR||path.join(__dirname,'dist');
const output=process.env.ROLLBOOK_SCREENSHOT_DIR;
const baseline=!!process.env.ROLLBOOK_CAPTURE_BASELINE;
const mime={'.html':'text/html','.js':'application/javascript','.css':'text/css','.png':'image/png','.svg':'image/svg+xml'};
const assets=[{id:'location:site',type:'location',name:'示例站点',status:'在用',rev:1,fields:{kind:'站点'}},{id:'network:lan',type:'network',name:'示例网段',status:'在用',rev:1,fields:{cidr:'192.0.2.0/24'}},{id:'service:demo',type:'service',name:'示例服务',status:'在用',rev:1,fields:{host_id:'host:000',current_version:'1.0'}},...Array.from({length:120},(_,index)=>({id:`host:${String(index).padStart(3,'0')}`,type:'host',name:`示例主机 ${index}`,status:'在用',rev:1,fields:{kind:'虚拟机'}}))];
async function setup(page,options={}){
 await page.route('**/*',route=>{const request=route.request(),url=new URL(request.url()),p=decodeURIComponent(url.pathname);const reply=(json,status=200)=>route.fulfill({status,json});
  if(p.startsWith('/api/')){
   if(p==='/api/auth/session')return options.unauth?reply({message:'unauthorized'},401):reply({login:'demo'});
   if(p==='/api/config')return reply({http_risk:false});
   if(p==='/api/version')return reply({version:'v0.5.0',commit:'abcdef1'});
   if(p==='/api/browse')return reply(assets.filter(asset=>asset.type===url.searchParams.get('type')));
   if(p==='/api/changes/batch'&&request.method()==='POST'){options.actions?.push(JSON.parse(request.postData()));return reply({results:[]})}
   if(p==='/api/changes')return reply(options.proposals||[]);
   if(p==='/api/assets/unverified'||p==='/api/versions/outdated'||p==='/api/keys')return reply([]);
   if(p==='/api/versions/alerts')return reply({enabled:false});
   if(p.endsWith('/detail'))return reply({object:assets.find(asset=>asset.id===p.split('/')[3]),notes:[],credentials:[],changes:[],referenced_by:[]});
   if(p.startsWith('/api/assets/'))return reply(assets.find(asset=>asset.id===p.slice('/api/assets/'.length))||{});
   if(p.startsWith('/api/hosts/'))return reply({host:assets[3],services:[],ips:[],ports:[],endpoints:[],dependencies:[]});
   if(p.startsWith('/api/sites/'))return reply({site:assets[0],uplinks:[],networks:[],routers:[],hosts:[],services:[]});
   if(p.startsWith('/api/networks/'))return reply({network:assets[1],ips:[],hosts:[]});
   if(p.startsWith('/api/services/')&&p.endsWith('/topology'))return reply({nodes:['host:000','service:demo'],details:[{id:'host:000',name:'示例主机 0',type:'host'},{id:'service:demo',name:'示例服务',type:'service'}],edges:[{from:'host:000',to:'service:demo',kind:'runs'}]});
   if(p.startsWith('/api/services/'))return reply({service:assets[2],host:assets[3],ips:[],ports:[],endpoints:[],dependencies:[],versions:[]});
   return reply([]);
  }
  const file=path.resolve(dist,'.'+(path.extname(p)?p:'/index.html'));if(!file.startsWith(dist+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:''});return route.fulfill({contentType:mime[path.extname(file)]||'application/octet-stream',body:fs.readFileSync(file)});
 });
 await page.goto('http://rollbook.test/');if(options.unauth)await page.locator('#login-form').waitFor();else await page.locator('#nav button[data-view="assets"]').waitFor({state:'attached'});
}
async function chooseType(page,type){if(baseline)return page.locator('.asset-type-select').selectOption(type);return page.getByRole('tab',{name:{location:'站点',host:'主机',network:'网段',service:'服务'}[type]}).click()}
async function shot(page,name){if(output){fs.mkdirSync(output,{recursive:true});await page.screenshot({path:path.join(output,name+'.png'),animations:'disabled'})}}
async function open(page,view){if(page.viewportSize().width<760){await page.getByRole('button',{name:'打开菜单'}).click();await page.locator('.mobile-drawer button').filter({hasText:{overview:'总览',assets:'资产',topology:'拓扑',pendingHub:'待处理',agent:'Agent 接入',settings:'设置'}[view]}).click()}else await page.locator(`#nav button[data-view="${view}"]`).click()}

test('界面布局、页签、面包屑、主题和手机抽屉',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const viewport of [{width:1440,height:900},{width:1280,height:800},{width:390,height:844}]){
   const page=await browser.newPage({viewport,locale:'zh-CN',colorScheme:'light'});page.setDefaultTimeout(10000);await setup(page);
   const mobile=viewport.width<760,prefix=mobile?'mobile':String(viewport.width);
   const loginPage=await browser.newPage({viewport,locale:'zh-CN'});await setup(loginPage,{unauth:true});await shot(loginPage,`${prefix}-login-light`);await loginPage.evaluate(()=>localStorage.setItem('rollbook.theme','dark'));await loginPage.reload();await loginPage.locator('#login-form').waitFor();await shot(loginPage,`${prefix}-login-dark`);await loginPage.close();
   await shot(page,`${prefix}-overview-light`);
   for(const type of ['location','host','network','service']){await open(page,'assets');await chooseType(page,type);await page.locator(type==='location'?'.asset-location-card':'.card').first().waitFor();await shot(page,`${prefix}-assets-${type}-light`)}
   if(!baseline){await open(page,'assets');await chooseType(page,'host');await page.getByRole('group',{name:'主机视图'}).getByRole('button',{name:'树'}).click();await page.locator('.tree-node').first().waitFor();assert.equal(await page.getByRole('tab').count(),4);assert.equal(await page.getByRole('tab',{name:'主机'}).getAttribute('aria-selected'),'true');assert.equal(new URL(page.url()).pathname,'/assets');await shot(page,`${prefix}-assets-host-tree-light`);await page.getByRole('group',{name:'主机视图'}).getByRole('button',{name:'列表'}).click();await page.locator('.card').first().waitFor()}
   if(!baseline&&!mobile){await open(page,'assets');await chooseType(page,'host');await page.locator('.card').nth(119).waitFor();const before=await page.locator('.sidebar').boundingBox(),foot=await page.locator('.sidebar-foot').boundingBox();await page.locator('.content').evaluate(element=>element.scrollTop=element.scrollHeight);assert.ok((await page.locator('.content').evaluate(element=>element.scrollTop))>0);assert.deepEqual(await page.locator('.sidebar').boundingBox(),before);assert.deepEqual(await page.locator('.sidebar-foot').boundingBox(),foot);assert.equal(await page.locator('.sidebar-foot').evaluate(foot=>{const right=foot.closest('.sidebar').getBoundingClientRect().right;return [...foot.querySelectorAll('button')].every(button=>{const rect=button.getBoundingClientRect();return Number.parseFloat(getComputedStyle(button).fontSize)>=14&&rect.height<=44&&rect.right<=right&&button.scrollHeight<=button.clientHeight})}),true)}
   await open(page,'assets');await chooseType(page,'host');await page.locator('.card').first().click();await page.getByRole('heading',{name:'示例主机 0'}).waitFor();await shot(page,`${prefix}-detail-light`);if(!baseline){assert.equal(await page.locator('.detail-breadcrumb button').count(),2);await page.locator('.detail-breadcrumb button').nth(1).click();await page.getByRole('heading',{name:'资产维护'}).waitFor();assert.equal(await page.getByRole('tab').count(),4);assert.equal(await page.getByRole('tab',{name:'主机'}).getAttribute('aria-selected'),'true')}
   for(const view of ['topology','pendingHub','agent','settings']){await open(page,view);await page.locator('#content h1').first().waitFor();if(view==='topology')await page.locator('.react-flow__node').first().waitFor();await shot(page,`${prefix}-${view}-light`)}
   if(!baseline){await page.locator('.top-controls .theme-control select').selectOption('dark');assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');await page.reload();await page.locator('html[data-theme=dark]').waitFor()}
   else await page.evaluate(()=>localStorage.setItem('rollbook.theme','dark')).then(()=>page.reload());
   await page.locator('#content h1').first().waitFor();await shot(page,`${prefix}-settings-dark`);
   for(const view of ['overview','assets','topology','pendingHub','agent']){await open(page,view);await page.locator('#content h1').first().waitFor();if(view==='topology')await page.locator('.react-flow__node').first().waitFor();await shot(page,`${prefix}-${view}-dark`)}
   for(const type of ['location','host','network','service']){await open(page,'assets');await chooseType(page,type);await page.locator(type==='location'?'.asset-location-card':'.card').first().waitFor();await shot(page,`${prefix}-assets-${type}-dark`)}
   await open(page,'assets');await chooseType(page,'host');await page.locator('.card').first().click();await page.getByRole('heading',{name:'示例主机 0'}).waitFor();await shot(page,`${prefix}-detail-dark`)
   if(!baseline){await page.locator('.top-controls .theme-control select').selectOption('light');assert.equal(await page.locator('html').getAttribute('data-theme'),'light');await page.locator('.top-controls .theme-control select').selectOption('system');assert.equal(await page.locator('html').getAttribute('data-theme'),'light')}
   await page.close();
  }
  if(!baseline){const english=await browser.newPage({locale:'en-US',colorScheme:'dark'});await english.addInitScript(()=>localStorage.setItem('rollbook.language','en'));await setup(english);assert.equal(await english.locator('html').getAttribute('data-theme'),'dark');assert.equal(await english.locator('html').evaluate(element=>getComputedStyle(element).backgroundColor),'rgb(15, 23, 42)');await open(english,'assets');await english.getByRole('tab',{name:'Devices & hosts'}).click();await english.locator('.card').first().click();await english.locator('.detail-breadcrumb').waitFor();assert.match(await english.locator('.detail-breadcrumb').innerText(),/Assets[\s\S]*Devices & hosts[\s\S]*示例主机 0/);await english.close()}
 }finally{await browser.close()}
});


test('待处理批量操作先选择后确认',async()=>{
 if(baseline)return;
 const browser=await chromium.launch({headless:true});
 try{const page=await browser.newPage({locale:'zh-CN'});const actions=[];await setup(page,{actions,proposals:[{id:'change:1',name:'示例变更',type:'change',fields:{object_id:'host:000',action:'update',diff_json:'{}'}}]});await open(page,'pendingHub');
 const reject=page.getByRole('button',{name:'批量驳回'});assert.equal(await reject.isDisabled(),true);await page.locator('.check-row input[type=checkbox]').check();assert.equal(await reject.isEnabled(),true);await reject.click();assert.equal(actions.length,0);await page.getByRole('button',{name:'确认驳回'}).click();assert.deepEqual(actions,[{ids:['change:1'],action:'reject'}]);await page.close()}finally{await browser.close()}
});

test('固定版登录与应用壳：主题、焦点、滚动及 320px 抽屉',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const width of [1440,390,320]){
   const viewport={width,height:width===1440?900:844};
   const login=await browser.newPage({viewport,locale:'zh-CN',colorScheme:'light'});
   await setup(login,{unauth:true});
   for(const theme of ['light','dark']){
    await login.evaluate(value=>localStorage.setItem('rollbook.theme',value),theme);
    await login.reload();await login.locator('#login-form').waitFor();
    assert.equal(await login.locator('html').getAttribute('data-theme'),theme);
    assert.ok(Number.parseFloat(await login.locator('#login-form input').first().evaluate(el=>getComputedStyle(el).height))>= (width<768?44:36));
    await login.keyboard.press('Tab');
    assert.ok(await login.locator(':focus-visible').count()>0);
    await shot(login,`v1-login-${width}-${theme}`);
   }
   await login.close();
   const page=await browser.newPage({viewport,locale:'zh-CN',colorScheme:'light'});await setup(page);
   for(const theme of ['light','dark']){
    await page.evaluate(value=>localStorage.setItem('rollbook.theme',value),theme);
    await page.reload();await page.locator('#content h1').first().waitFor();
    assert.equal(await page.locator('html').getAttribute('data-theme'),theme);
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    assert.equal(await page.locator('.main').evaluate(el=>el.scrollTop),0);
    await shot(page,`v1-overview-${width}-${theme}`);
   }
   if(width<1200){
    if(width<768){
     const menuSize=await page.getByRole('button',{name:'打开菜单'}).evaluate(el=>{const rect=el.getBoundingClientRect();return {width:rect.width,height:rect.height}});
     assert.ok(menuSize.width>=44&&menuSize.height>=44,`menu target at ${width}: ${JSON.stringify(menuSize)}`);
    }
    await page.getByRole('button',{name:'打开菜单'}).click();
    await page.locator('.mobile-drawer').waitFor();
    if(width<768){
     const closeSize=await page.getByRole('button',{name:'取消'}).evaluate(el=>{const rect=el.getBoundingClientRect();return {width:rect.width,height:rect.height}});
     assert.ok(closeSize.width>=44&&closeSize.height>=44,`drawer close target at ${width}: ${JSON.stringify(closeSize)}`);
    }
    await page.getByRole('button',{name:'取消'}).click();
    await page.locator('.mobile-drawer').waitFor({state:'hidden'});
   }else{
    assert.equal(await page.locator('#nav button[aria-current=page]').count(),1);
    const header=await page.locator('.topbar').boundingBox();
    await page.locator('.content').evaluate(el=>{el.innerHTML+='<div style="height:1600px"></div>';el.scrollTop=el.scrollHeight});
    assert.ok(await page.locator('.content').evaluate(el=>el.scrollTop)>0);
    assert.deepEqual(await page.locator('.topbar').boundingBox(),header);
   }
   await page.close();
  }
 }finally{await browser.close()}
});

test('业务页面同数据浅深矩阵及窄屏布局',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const width of [1440,1024,768,390,320]){
   const viewport={width,height:width>=768?900:844};
   for(const theme of ['light','dark']){
    const page=await browser.newPage({viewport,locale:'zh-CN',colorScheme:theme});
    page.setDefaultTimeout(10000);
    await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);
    await setup(page);
    for(const [route,name] of [['/','overview'],['/assets','assets'],['/assets/services','assets-services'],['/assets/sites','assets-sites'],['/assets/hosts','assets-hosts'],['/assets/networks','assets-networks'],['/topology','topology'],['/pending','pending'],['/pending?tab=unverified','pending-unverified'],['/pending?tab=outdated','pending-outdated'],['/agent','agent'],['/settings','settings'],['/settings/backup','backup'],['/changes','changes'],['/contexts','contexts'],['/search','search']]){
     await page.goto('http://rollbook.test'+route);
     await page.locator('#content h1').first().waitFor();
     assert.equal(await page.locator('html').getAttribute('data-theme'),theme);
     assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`${route} ${width} ${theme} page overflow`);
     assert.equal(await page.locator('#content h1').count(),1,`${route} heading`);
     await shot(page,`v1-business-${width}-${theme}-${name}`);
    }
    await page.goto('http://rollbook.test/assets');
    await page.getByRole('tab',{name:'主机'}).click();
    await page.locator('.catalog-table tbody tr').first().click();
    await page.getByRole('heading',{name:'示例主机 0'}).waitFor();
    assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
    await shot(page,`v1-business-${width}-${theme}-detail`);
    if(width===1440||width===390){
     await page.goto('http://rollbook.test/assets/hosts');
     await page.getByRole('button',{name:'新建'}).click();
     const form=page.locator('.asset-form');
     const firstInputHeight=await form.locator('input:visible').first().evaluate(el=>el.getBoundingClientRect().height);
     assert.equal(firstInputHeight,width<768?44:36,`asset form input height at ${width}`);
     await form.getByRole('textbox',{name:'名称'}).fill('同数据表单示例');
     await form.getByRole('combobox',{name:'类别 *'}).selectOption('物理机');
     await form.getByRole('textbox',{name:'操作原因 *'}).fill('验证字段错误');
     await form.locator('.form-advanced summary').click();
     await form.locator('#asset-field-resources_json').fill('{invalid');
     await form.getByRole('button',{name:'创建'}).click();
     assert.equal(await form.locator('#asset-field-resources_json').getAttribute('aria-invalid'),'true');
     assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true);
     await shot(page,`v1-business-${width}-${theme}-form-error`);
     await form.locator('.asset-form-fields').evaluate(element=>element.scrollTop=element.scrollHeight);
     const lastField=await form.getByRole('textbox',{name:'操作原因 *'}).boundingBox();
     const actions=await form.locator('.modal-actions').boundingBox();
     assert.ok(lastField&&actions&&lastField.y+lastField.height<=actions.y,`form footer obscures last field at ${width}`);
     const failure=route=>route.fulfill({status:500,json:{message:'测试读取失败'}});
     await page.route('**/api/browse?*',failure);
     await page.goto('http://rollbook.test/assets');
     await page.locator('#content [role=alert]').first().waitFor();
     await shot(page,`v1-business-${width}-${theme}-assets-error`);
     await page.unroute('**/api/browse?*',failure);
     const slow=async route=>{await new Promise(resolve=>setTimeout(resolve,1000));await route.fulfill({json:[]})};
     await page.route('**/api/browse?*',slow);
     await page.goto('http://rollbook.test/assets');
     await page.locator('.table-skeleton').waitFor();
     await shot(page,`v1-business-${width}-${theme}-assets-loading`);
     await page.unroute('**/api/browse?*',slow);
    }
    await page.close();
   }
  }
 }finally{await browser.close()}
});
