const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const dir=path.join(__dirname,'dist');
const mime={'.html':'text/html','.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png'};

test('Agent 接入：配置复制、Key 一次性明文、下载、中英文与响应式主题',async t=>{
 const browser=await chromium.launch({headless:true});t.after(()=>browser.close());
 for(const scenario of [{language:'zh-CN',width:1280},{language:'en-US',width:390}]){
  const page=await browser.newPage({viewport:{width:scenario.width,height:850},locale:scenario.language,permissions:['clipboard-read','clipboard-write'],acceptDownloads:true});
  await page.addInitScript(language=>localStorage.setItem('rollbook.language',language),scenario.language);
  await page.addInitScript(()=>Object.defineProperty(navigator,'clipboard',{value:{writeText:async value=>{window.__copied=value}}}));
  const keys=[];
  await page.route('**/*',route=>{
   const request=route.request(),url=new URL(request.url()),p=url.pathname;
   const reply=data=>route.fulfill({status:200,contentType:'application/json',body:JSON.stringify(data)});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({http_risk:false});
   if(p==='/api/version')return reply({version:'v0.5.0',commit:'abcdef0'});
   if(p==='/api/browse')return reply([{id:'location:test',type:'location',name:'test'}]);
   if(p==='/api/keys'&&request.method()==='GET')return reply(keys);
   if(p==='/api/keys'&&request.method()==='POST'){keys.push({id:'key:1',name:'demo',level:'read'});return reply({token:'rb_once_secret'})}
   if(p==='/api/agent/skill.zip')return route.fulfill({status:200,contentType:'application/zip',headers:{'Content-Disposition':'attachment; filename="rollbook-skill.zip"'},body:Buffer.from('zip')});
   if(p.startsWith('/api/'))return reply([]);
   const file=path.resolve(dir,'.'+((p==='/'||p==='/agent')?'/index.html':p));
   if(!file.startsWith(dir+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:''});
   return route.fulfill({status:200,contentType:mime[path.extname(file)]||'application/octet-stream',body:fs.readFileSync(file)});
  });
  await page.goto('http://rollbook.test/agent');
  const en=scenario.language.startsWith('en');
  await page.getByRole('heading',{name:en?'MCP setup':'MCP 接入配置'}).waitFor();
  const guide=page.locator('.agent-guide').first();
  assert.match(await guide.innerText(),/http:\/\/rollbook.test\/mcp/);
  assert.match(await guide.innerText(),/tools\/list/);
  assert.match(await guide.innerText(),/<ROLLBOOK_KEY>/);
  const check=await guide.locator('.setup-block').last().locator('pre').innerText();
  assert.match(check,/-H 'Authorization: Bearer <ROLLBOOK_KEY>'/);
  assert.doesNotMatch(check,/^\+/m);
  await guide.locator('.setup-block').first().getByRole('button',{name:en?'Copy':'复制'}).click();
  assert.match(await page.evaluate(()=>window.__copied),/rollbook.test\/mcp/);
  assert.match(await page.locator('.agent-guide').last().locator('a').first().getAttribute('href'),/releases\/tag\/v0.5.0/);
  if(process.env.ROLLBOOK_SCREENSHOT_DIR){fs.mkdirSync(process.env.ROLLBOOK_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,`agent-${scenario.width}-${en?'en':'zh'}-light.png`),fullPage:true})}
  const downloadPromise=page.waitForEvent('download');
  await page.getByRole('link',{name:en?'Download Skill zip':'下载 Skill zip'}).click();
  assert.equal((await downloadPromise).suggestedFilename(),'rollbook-skill.zip');
  await page.getByRole('button',{name:en?'Create key':'创建 Key'}).click();
  await page.locator('.modal input').first().fill('demo');
  await page.locator('.modal button[type="submit"]').click();
  await page.getByText('rb_once_secret',{exact:true}).waitFor();
  assert.doesNotMatch(await page.locator('.modal .setup-block').first().innerText(),/rb_once_secret/);
  await page.locator('.modal input[type="checkbox"]').check();
  assert.match(await page.locator('.modal .setup-block').first().innerText(),/rb_once_secret/);
  await page.getByRole('button',{name:en?'Done':'完成'}).click();
  assert.equal(await page.getByText('rb_once_secret').count(),0);
  if(scenario.width<760){await page.evaluate(()=>localStorage.setItem('rollbook.theme','dark'));await page.reload();await page.getByRole('heading',{name:en?'MCP setup':'MCP 接入配置'}).waitFor()}
  else await page.locator('.top-controls button[aria-label]').first().click();
  assert.equal(await page.locator('html').getAttribute('data-theme'),'dark');
  if(process.env.ROLLBOOK_SCREENSHOT_DIR){fs.mkdirSync(process.env.ROLLBOOK_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,`agent-${scenario.width}-${en?'en':'zh'}-dark.png`),fullPage:true})}
  await page.close();
 }
});
