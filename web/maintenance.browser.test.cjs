const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

const html=fs.readFileSync(path.join(__dirname,'index.html'),'utf8');
const js=fs.readFileSync(path.join(__dirname,'app.js'),'utf8');
const css=fs.readFileSync(path.join(__dirname,'style.css'),'utf8');

test('从空库用网页录入、维护并两次确认删除',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage();page.setDefaultTimeout(5000);
  const objects=new Map();const writes=[];let offlineID='',deleteAttempts=0;
  await page.route('http://rollbook.test/**',async route=>{
   const req=route.request(),u=new URL(req.url()),p=decodeURIComponent(u.pathname),m=req.method();
   const body=()=>JSON.parse(req.postData()||'{}');
   const reply=(data,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
   if(p==='/')return route.fulfill({contentType:'text/html',body:html});
   if(p==='/app.js')return route.fulfill({contentType:'application/javascript',body:js});
   if(p==='/style.css')return route.fulfill({contentType:'text/css',body:css});
   if(p==='/api/config')return reply({allow_insecure_http:false});
   if(p==='/api/auth/session')return reply({login:'owner'});
   if(p==='/api/browse')return reply([...objects.values()].filter(o=>o.type===u.searchParams.get('type')));
   if(p==='/api/assets'&&m==='POST'){
    const data=body();writes.push({action:'create',...data});const o={...data.object,rev:1,status:data.object.status||'在用'};objects.set(o.id,o);return reply(o);
   }
   if(p.startsWith('/api/assets/')){
    const parts=p.split('/'),id=decodeURIComponent(parts[3]),o=objects.get(id);
    if(m==='GET')return reply(o||{},o?200:404);
    if(m==='PATCH'){const data=body();writes.push({action:'update',...data});Object.assign(o,data.patch,{fields:{...o.fields,...data.patch.fields},rev:o.rev+1});return reply(o)}
    if(parts[4]==='verify'){const data=body();writes.push({action:'verify',...data});o.rev++;o.verified_at=new Date().toISOString();return reply(o)}
    if(m==='DELETE'){if(deleteAttempts++===0)return reply({fields:[{field:'id',reason:'对象仍被引用',references:['service:app']}]},422);writes.push({action:'delete',...body()});objects.delete(id);return reply({ok:true})}
   }
   if(p==='/api/notes'&&m==='POST'){const data=body();writes.push({action:'note',...data});objects.get(data.id).rev++;objects.set('note:one',{id:'note:one',type:'note',name:'运维笔记',rev:1,status:'在用',fields:{target_id:data.id,kind:data.kind,body_md:data.body_md}});return reply({id:'note:one'})}
   if(p==='/api/sites/location:site')return reply({id:'location:site',name:'站点',uplinks:[],networks:[],routers:[],hosts:[]});
   if(p==='/api/hosts/host:box/inventory')return reply({hosts:[],ips:[...objects.values()].filter(o=>o.type==='ip').map(o=>o.id),services:[...objects.values()].filter(o=>o.type==='service').map(o=>o.id)});
   if(p==='/api/networks/network:net/inventory')return reply({hosts:[],ips:[],services:[]});
   if(p==='/api/services/service:app')return reply({service:objects.get('service:app'),host:objects.get('host:box'),ips:[],ports:[],endpoints:[],dependencies:[],versions:[],credentials:[],notes:[],changes:[]});
   if(p==='/api/services/service:app/topology')return reply({nodes:[],edges:[]});
   if(p==='/api/hosts/host:box/offline'&&m==='GET')return reply({preview:{host:objects.get('host:box'),services:[objects.get('service:app')].filter(Boolean),ips:[objects.get('ip:one')].filter(Boolean),endpoints:[]},operation_id:offlineID});
   if(p==='/api/hosts/host:box/offline'&&m==='POST'){const data=body();writes.push({action:'offline',...data});offlineID='offline:one';objects.get('host:box').status='停用';objects.get('host:box').rev++;return reply({operation_id:offlineID})}
   if(p==='/api/hosts/host:box/restore'&&m==='POST'){writes.push({action:'restore',...body()});objects.get('host:box').status='在用';objects.get('host:box').rev++;return reply({restored:['host:box'],skipped:[]})}
   return reply({message:'Unexpected request '+m+' '+p},404);
  });
  await page.goto('http://rollbook.test/');
  await page.getByRole('button',{name:/资产维护/}).click();
  async function create(type,name,fields){
   await page.getByRole('button',{name:/资产维护/}).click();
   await page.locator('#content select').first().selectOption(type);
   await page.getByRole('button',{name:'新建'}).click();
   const form=page.locator('.asset-form');
   await form.locator('label').filter({hasText:'名称'}).locator('input').fill(name);
   await form.locator('label').filter({hasText:'ID（可修改）'}).locator('input').fill(type+':'+({location:'site',network:'net',host:'box',ip:'one',service:'app'}[type]));
   for(const [field,value] of Object.entries(fields)){const input=form.locator('label').filter({hasText:field}).first().locator('input, select');if(await input.evaluate(el=>el.tagName)==='SELECT')await input.selectOption(value);else await input.fill(value)}
   await form.locator('label').filter({hasText:'操作原因'}).locator('input').fill('录入测试');
   await form.getByRole('button',{name:'创建'}).click();
   await page.locator('.modal').waitFor({state:'detached'});
  }
  await create('location','站点',{kind:'站点',source:'自动发现'});
  await create('network','网段',{cidr:'192.0.2.0/24',location_id:'location:site'});
  await create('host','主机',{kind:'物理机',location_id:'location:site'});
  await create('ip','192.0.2.5',{address:'192.0.2.5',host_id:'host:box',network_id:'network:net'});
  await create('service','应用',{host_id:'host:box'});
  assert.equal(writes.filter(x=>x.action==='create').length,5);
  assert.equal(writes[0].object.source,'自动发现');
  await page.getByRole('button',{name:/资产维护/}).click();await page.locator('#content select').first().selectOption('host');await page.locator('.card').filter({hasText:'host:box'}).click();
  await page.getByRole('button',{name:'编辑'}).click();const form=page.locator('.asset-form');await form.locator('label').filter({hasText:'名称'}).locator('input').fill('主机改名');await form.locator('label').filter({hasText:'操作原因'}).locator('input').fill('改名');await form.getByRole('button',{name:'保存修改'}).click();await page.locator('.modal').waitFor({state:'detached'});
  page.once('dialog',d=>d.accept('现场确认'));await page.getByRole('button',{name:'标记核实'}).click();await page.waitForTimeout(50);
  await page.getByRole('button',{name:'加笔记'}).click();await page.locator('.modal textarea').fill('检查完成');await page.locator('.modal input').fill('巡检');await page.getByRole('button',{name:'保存笔记'}).click();await page.locator('.modal').waitFor({state:'detached'});
  await page.getByRole('button',{name:/资产维护/}).click();await page.locator('#content select').first().selectOption('note');await page.locator('.card').filter({hasText:'note:one'}).click();await page.getByRole('button',{name:'删除'}).waitFor();assert.equal(await page.getByRole('button',{name:'编辑'}).count(),0);
  await page.getByRole('button',{name:/资产维护/}).click();await page.locator('#content select').first().selectOption('host');await page.locator('.card').filter({hasText:'host:box'}).click();
  await page.getByRole('button',{name:'停用主机'}).click();assert.match(await page.locator('.modal').innerText(),/应用/);assert.match(await page.locator('.modal').innerText(),/192.0.2.5/);await page.locator('.modal .check-row input').nth(0).check();await page.locator('.modal .check-row input').nth(1).check();await page.locator('.modal input[placeholder="停用原因"]').fill('换机');await page.getByRole('button',{name:'确认停用'}).click();await page.locator('.modal').waitFor({state:'detached'});
  await page.getByRole('button',{name:'恢复主机'}).click();await page.locator('.modal input[placeholder="恢复原因"]').fill('设备恢复');await page.getByRole('button',{name:'恢复',exact:true}).click();await page.locator('.modal').waitFor({state:'detached'});
  await page.getByRole('button',{name:/资产维护/}).click();await page.locator('#content select').first().selectOption('ip');await page.locator('.card').click();await page.getByRole('button',{name:'删除'}).click();await page.locator('.modal input[placeholder="删除原因"]').fill('撤销测试数据');let dialogs=0;page.on('dialog',d=>{dialogs++;d.accept()});await page.getByRole('button',{name:'继续删除'}).click();await page.locator('.modal .error').getByText(/service:app/).waitFor();await page.getByRole('button',{name:'继续删除'}).click();await page.locator('.modal').waitFor({state:'detached'});assert.equal(dialogs,2);
  assert.deepEqual(writes.map(x=>x.action),['create','create','create','create','create','update','verify','note','offline','restore','delete']);
 }finally{await browser.close()}
});
