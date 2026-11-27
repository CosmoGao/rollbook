const {test}=require('node:test');
const assert=require('node:assert/strict');
const {spawn,execFileSync}=require('node:child_process');
const {mkdtempSync,rmSync}=require('node:fs');
const {tmpdir}=require('node:os');
const path=require('node:path');
const {chromium}=require('playwright');

test('假飞书上游与真实浏览器：授权绑定、冲突及邮箱登录',async()=>{
 const dir=mkdtempSync(path.join(tmpdir(),'rollbook-feishu-flow-'));
 const binary=path.join(dir,'fixture');
 execFileSync('go',['build','-o',binary,'./web/test-fixtures/feishu-server.go']);
 const server=spawn(binary,[],{stdio:['pipe','pipe','inherit']});
 let browser;
 try{
  const origin=await new Promise((resolve,reject)=>{let output='';const timeout=setTimeout(()=>reject(new Error('fixture timeout')),10000);server.stdout.on('data',chunk=>{output+=chunk;if(output.includes('\n')){clearTimeout(timeout);resolve(output.trim().split('\n')[0])}});server.once('exit',code=>{clearTimeout(timeout);reject(new Error('fixture exited: '+code))})});
  browser=await chromium.launch({headless:true});
  async function scenario(code,expect,bind=false){
   const context=await browser.newContext({ignoreHTTPSErrors:true,locale:'zh-CN'});
   const page=await context.newPage();page.setDefaultTimeout(5000);
   await context.route(/\/api\/auth\/feishu\/(?:bind\/)?start$/,async route=>{
    const response=await route.fetch({maxRedirects:0});
    const state=new URL(response.headers().location).searchParams.get('state');
    await route.fulfill({response,status:302,headers:{...response.headers(),location:`${origin}/api/auth/feishu/callback?state=${encodeURIComponent(state)}&code=${encodeURIComponent(code)}`}});
   });
   await page.goto(origin+'/');
   if(bind){
    await page.locator('#login-form input[name="login"]').fill('owner');
    await page.locator('#login-form input[name="password"]').fill('initial password 123');
    const loginResponse=page.waitForResponse(response=>new URL(response.url()).pathname==='/api/auth/login',{timeout:15000});
    await page.locator('#login-form button[type="submit"]').click();
    assert.equal((await loginResponse).status(),200,await page.locator('#login-error').innerText());
    await page.locator('.account-avatar').waitFor({timeout:15000});
    await page.goto(origin+'/settings/personal');
    await page.getByRole('link',{name:'绑定飞书'}).click();
    if(expect==='ok'){
     await page.getByText('飞书绑定成功').waitFor();
     assert.equal(await page.getByRole('link',{name:'绑定飞书'}).count(),0);
     const session=await page.evaluate(()=>fetch('/api/auth/session').then(r=>r.json()));assert.equal(session.login,'owner');await page.evaluate(()=>fetch('/fixture/reset-owner',{method:'POST'}));
    }else await page.getByRole('alert').getByText(/飞书身份已被其他账号绑定/).waitFor();
   }else{
    await page.getByRole('link',{name:'使用飞书登录'}).click();
    if(expect==='owner'){await page.locator('.account-avatar[title="owner"]').waitFor()}
    else if(expect==='unmatched'){await page.getByRole('alert').getByText(/邮箱未匹配/).waitFor()}
    else if(expect==='conflict'){await page.getByRole('alert').getByText(/归属冲突/).waitFor()}
    else if(expect==='disabled'){await page.getByRole('alert').getByText(/绑定已禁用/).waitFor()}
   }
   await context.close();
  }
  await scenario('bind_other|','conflict',true);
  await scenario('new-bind|','ok',true);
  await scenario('auto|owner@example.com','owner');
  await scenario('auto|owner@example.com','owner');
  await scenario('unknown|unknown@example.com','unmatched');
  await scenario('conflict|owner@example.com','conflict');
  await scenario('disabled|owner@example.com','disabled');
 }finally{if(browser)await browser.close();server.stdin.end('stop\n');if(server.exitCode===null)await new Promise(resolve=>server.once('exit',resolve));rmSync(dir,{recursive:true,force:true})}
});
