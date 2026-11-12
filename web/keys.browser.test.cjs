const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');

const asset=name=>fs.readFileSync(path.join(__dirname,name));

test('Key 列表显示元数据，编辑后更新，吊销项默认折叠',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN'});
  let current={id:'key-1',name:'原名称',level:'read',issuer_name:'owner',note:'旧备注',created_at:'2026-11-01T00:00:00Z',expires_at:'',last_used_at:''};
  const calls=[];
  await page.route('http://rollbook.test/**',route=>{
   const request=route.request(),pathname=new URL(request.url()).pathname;
   const reply=(data,status=200)=>route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
   if(pathname==='/')return route.fulfill({contentType:'text/html',body:asset('index.html')});
   if(['app.js','i18n.js','style.css'].includes(pathname.slice(1)))return route.fulfill({contentType:pathname.endsWith('.css')?'text/css':'application/javascript',body:asset(pathname.slice(1))});
   if(pathname==='/api/auth/session')return reply({login:'owner'});
   if(pathname==='/api/config')return reply({allow_insecure_http:false});
   if(pathname==='/api/version')return reply({version:'test',commit:'test'});
   if(pathname==='/api/browse')return reply([]);
   if(pathname==='/api/keys'&&request.method()==='GET')return reply([current]);
   if(pathname.endsWith('/metadata')&&request.method()==='PATCH'){const value=JSON.parse(request.postData());calls.push(value);current={...current,...value};return reply({ok:true})}
   if(pathname.endsWith('/revoke')&&request.method()==='POST'){current.revoked_at='2026-11-12T00:00:00Z';return reply({ok:true})}
   return reply({message:'unexpected '+pathname},404);
  });
  await page.goto('http://rollbook.test/');
  await page.getByRole('button',{name:'稍后再说'}).click();
  await page.locator('#nav button[data-view="keys"]').click();
  await page.locator('#content .row').first().waitFor();
  assert.match(await page.locator('#content').innerText(),/只读/);
  assert.match(await page.locator('#content').innerText(),/旧备注/);
  assert.match(await page.locator('#content').innerText(),/创建时间/);
  assert.match(await page.locator('#content').innerText(),/到期时间/);
  assert.match(await page.locator('#content').innerText(),/最后使用时间/);
  await page.locator('#content button').filter({hasText:'编辑'}).click();
  await page.locator('.modal input').first().fill('新名称');
  await page.locator('.modal textarea').fill('新备注');
  await page.locator('.modal input[type="datetime-local"]').fill('2030-01-01T12:00');
  await page.locator('.modal button[type="submit"]').click();
  await page.locator('.modal').waitFor({state:'detached'});
  assert.equal(calls.length,1);
  assert.equal(calls[0].name,'新名称');
  assert.equal(calls[0].note,'新备注');
  assert.match(calls[0].expires_at,/2030-01-01/);
  assert.match(await page.locator('#content').innerText(),/新名称/);
  await page.locator('#content button').filter({hasText:'吊销'}).click();
  await page.locator('.modal button').filter({hasText:'确认吊销'}).click();
  await page.locator('.modal').waitFor({state:'detached'});
  assert.equal(await page.locator('#content .row').count(),1);
  assert.equal(await page.locator('#content details').evaluate(node=>node.open),false);
  assert.equal(await page.locator('#content details').isVisible(),true);
  assert.equal(await page.locator('#content details .row').isVisible(),false);
  await page.locator('#content details summary').click();
  assert.equal(await page.locator('#content details .row').isVisible(),true);
  assert.match(await page.locator('#content details').innerText(),/已吊销/);
 }finally{await browser.close()}
});
