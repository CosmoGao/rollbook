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
async function shot(page,name){if(output){fs.mkdirSync(output,{recursive:true});await page.screenshot({path:path.join(output,name+'.png')})}}
async function open(page,view){if(page.viewportSize().width<760){await page.getByRole('button',{name:'打开菜单'}).click();await page.locator('.mobile-drawer button').filter({hasText:{overview:'总览',assets:'资产',topology:'拓扑',pendingHub:'待处理',agent:'Agent 接入',settings:'设置'}[view]}).click()}else await page.locator(`#nav button[data-view="${view}"]`).click()}

test('界面布局、页签、面包屑、主题和手机抽屉',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const viewport of [{width:1440,height:900},{width:1280,height:800},{width:390,height:844}]){
   const page=await browser.newPage({viewport,locale:'zh-CN',colorScheme:'light'});page.setDefaultTimeout(10000);await setup(page);
   const mobile=viewport.width<760,prefix=mobile?'mobile':String(viewport.width);
   const loginPage=await browser.newPage({viewport,locale:'zh-CN'});await setup(loginPage,{unauth:true});await shot(loginPage,`${prefix}-login-light`);await loginPage.evaluate(()=>localStorage.setItem('rollbook.theme','dark'));await loginPage.reload();await loginPage.locator('#login-form').waitFor();await shot(loginPage,`${prefix}-login-dark`);await loginPage.close();
   await shot(page,`${prefix}-overview-light`);
   for(const type of ['location','host','network','service']){await open(page,'assets');await page.locator('.asset-type-select').selectOption(type);await page.locator('.card').first().waitFor();await shot(page,`${prefix}-assets-${type}-light`)}
   if(!baseline&&!mobile){await open(page,'assets');await page.locator('.asset-type-select').selectOption('host');await page.locator('.card').nth(119).waitFor();const before=await page.locator('.sidebar').boundingBox(),foot=await page.locator('.sidebar-foot').boundingBox();await page.locator('.main').evaluate(element=>element.scrollTop=element.scrollHeight);assert.ok((await page.locator('.main').evaluate(element=>element.scrollTop))>0);assert.deepEqual(await page.locator('.sidebar').boundingBox(),before);assert.deepEqual(await page.locator('.sidebar-foot').boundingBox(),foot)}
   await open(page,'assets');await page.locator('.asset-type-select').selectOption('host');await page.locator('.card').first().click();await page.getByRole('heading',{name:'示例主机 0'}).waitFor();await shot(page,`${prefix}-detail-light`);if(!baseline){assert.equal(await page.locator('.detail-breadcrumb button').count(),2);await page.locator('.detail-breadcrumb button').first().click();await page.getByRole('heading',{name:'资产维护'}).waitFor()}
   for(const view of ['topology','pendingHub','agent','settings']){await open(page,view);await page.locator('#content h1').first().waitFor();if(view==='topology')await page.locator('.react-flow__node').first().waitFor();await shot(page,`${prefix}-${view}-light`)}
   if(!baseline){await page.getByRole('button',{name:'切换主题'}).click();assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');await page.reload();await page.locator('html[data-theme=dark]').waitFor()}
   else await page.evaluate(()=>localStorage.setItem('rollbook.theme','dark')).then(()=>page.reload());
   await page.locator('#content h1').first().waitFor();await shot(page,`${prefix}-settings-dark`);
   for(const view of ['overview','assets','topology','pendingHub','agent']){await open(page,view);await page.locator('#content h1').first().waitFor();if(view==='topology')await page.locator('.react-flow__node').first().waitFor();await shot(page,`${prefix}-${view}-dark`)}
   for(const type of ['location','host','network','service']){await open(page,'assets');await page.locator('.asset-type-select').selectOption(type);await page.locator('.card').first().waitFor();await shot(page,`${prefix}-assets-${type}-dark`)}
   await open(page,'assets');await page.locator('.asset-type-select').selectOption('host');await page.locator('.card').first().click();await page.getByRole('heading',{name:'示例主机 0'}).waitFor();await shot(page,`${prefix}-detail-dark`)
   if(!baseline){await page.getByRole('button',{name:'切换主题'}).click();assert.equal(await page.locator('html').getAttribute('data-theme'),'light');await page.getByRole('button',{name:'切换主题'}).click();assert.equal(await page.locator('html').getAttribute('data-theme'),'system')}
   await page.close();
  }
  if(!baseline){const english=await browser.newPage({locale:'en-US',colorScheme:'dark'});await english.addInitScript(()=>localStorage.setItem('rollbook.language','en'));await setup(english);assert.equal(await english.locator('html').getAttribute('data-theme'),'system');assert.equal(await english.locator('html').evaluate(element=>getComputedStyle(element).backgroundColor),'rgb(9, 9, 11)');await open(english,'assets');await english.locator('.asset-type-select').selectOption('host');await english.locator('.card').first().click();await english.locator('.detail-breadcrumb').waitFor();assert.match(await english.locator('.detail-breadcrumb').innerText(),/Assets[\s\S]*Hosts[\s\S]*示例主机 0/);await english.close()}
 }finally{await browser.close()}
});


test('待处理批量操作先选择后确认',async()=>{
 if(baseline)return;
 const browser=await chromium.launch({headless:true});
 try{const page=await browser.newPage({locale:'zh-CN'});const actions=[];await setup(page,{actions,proposals:[{id:'change:1',name:'示例变更',type:'change',fields:{object_id:'host:000',action:'update',diff_json:'{}'}}]});await open(page,'pendingHub');
 const reject=page.getByRole('button',{name:'批量驳回'});assert.equal(await reject.isDisabled(),true);await page.locator('.check-row input[type=checkbox]').check();assert.equal(await reject.isEnabled(),true);await reject.click();assert.equal(actions.length,0);await page.getByRole('button',{name:'确认驳回'}).click();assert.deepEqual(actions,[{ids:['change:1'],action:'reject'}]);await page.close()}finally{await browser.close()}
});
