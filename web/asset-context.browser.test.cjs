const {installBundle}=require('./browser.bundle.cjs');
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

test('双站点位置和主机树、自动 ID 与宿主预填',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN',viewport:{width:1280,height:720}});page.setDefaultTimeout(5000);
  const assets=[
   {id:'location:bj',type:'location',name:'北京家',kind:'站点',site_id:'location:bj',site_name:'北京家',fields:{kind:'站点'}},
   {id:'location:rack',type:'location',name:'主卧机柜',kind:'机柜',site_id:'location:bj',site_name:'北京家',fields:{kind:'机柜',parent_id:'location:bj'}},
   {id:'location:xh',type:'location',name:'香河',kind:'站点',site_id:'location:xh',site_name:'香河',fields:{kind:'站点'}},
   {id:'host:pve',type:'host',name:'Proxmox',kind:'物理机',site_id:'location:bj',site_name:'北京家',primary_ip:'192.0.2.2',fields:{kind:'物理机',location_id:'location:rack'}},
   {id:'host:xh',type:'host',name:'香河 NAS',kind:'NAS',site_id:'location:xh',site_name:'香河',fields:{kind:'NAS',location_id:'location:xh'}}
  ];let created;
  await page.route('http://rollbook.test/**',route=>{
   const req=route.request(),u=new URL(req.url()),p=decodeURIComponent(u.pathname),reply=json=>route.fulfill({json});
   if(p==='/')return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/config')return reply({http_risk:false});
   if(p==='/api/version')return reply({version:'test',commit:'test'});
   if(p==='/api/browse')return reply(assets.filter(item=>item.type===u.searchParams.get('type')));
   if(p==='/api/assets'&&req.method()==='POST'){created=JSON.parse(req.postData()).object;return reply({...created,rev:1})}
   if(p.startsWith('/api/assets/')&&p.endsWith('/detail')){const object=assets.find(item=>item.id===p.split('/')[3])||created;return reply({object,notes:[],credentials:[],changes:[],referenced_by:[]})}
   if(p.startsWith('/api/assets/'))return reply(assets.find(item=>item.id===p.slice('/api/assets/'.length))||created);
   return reply([]);
  });
  await installBundle(page);await page.goto('http://rollbook.test/');
  await page.locator('#nav button[data-view="assets"]').click();
  assert.match(await page.locator('.tree-node').filter({hasText:'主卧机柜'}).first().innerText(),/机柜/);
  await page.getByRole('tab',{name:'主机'}).click();await page.getByRole('group',{name:'主机视图'}).getByRole('button',{name:'树'}).click();
  assert.deepEqual(await page.locator('.host-site h2').allTextContents(),['北京家','香河']);
  assert.match(await page.locator('.host-site').first().innerText(),/Proxmox.*192\.0\.2\.2/s);
  if(process.env.ROLLBOOK_SCREENSHOT_DIR){fs.mkdirSync(process.env.ROLLBOOK_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.ROLLBOOK_SCREENSHOT_DIR,'v2ri-64-host-tree.png')})}
  await page.getByRole('group',{name:'主机视图'}).getByRole('button',{name:'列表'}).click();await page.getByRole('button',{name:'新建'}).click();
  const form=page.locator('.asset-form');await form.getByRole('textbox',{name:'名称'}).fill('新主机');
  assert.equal(await form.getByRole('textbox',{name:'ID（可修改）'}).inputValue(),'host:新主机');
  assert.equal(await form.getByRole('combobox',{name:'位置 *'}).inputValue(),'location:bj');
  await form.getByRole('textbox',{name:'操作原因 *'}).fill('登记');await form.getByRole('button',{name:'创建'}).click();
  assert.equal(created.id,'host:新主机');assert.equal(created.fields.kind,'物理机');assert.equal(created.status,'在用');
  await page.locator('#nav button[data-view="assets"]').click();await page.getByRole('tab',{name:'主机'}).click();await page.getByText('Proxmox').first().click();
  await page.getByRole('button',{name:'加服务'}).click();
  assert.equal(await page.locator('.asset-form').getByRole('combobox',{name:'宿主 *'}).inputValue(),'host:pve');
 }finally{await browser.close()}
});
