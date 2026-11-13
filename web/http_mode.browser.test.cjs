const {test}=require('node:test');
const assert=require('node:assert/strict');
const {spawn,execFileSync}=require('node:child_process');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const net=require('node:net');
const http=require('node:http');
const https=require('node:https');
const {chromium}=require('playwright');

async function freePort(){
 const server=net.createServer();
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 const port=server.address().port;
 await new Promise(resolve=>server.close(resolve));
 return port;
}

async function ready(url){
 for(let i=0;i<100;i++){
  try{const response=await fetch(url);if(response.ok)return}catch{}
  await new Promise(resolve=>setTimeout(resolve,100));
 }
 throw new Error(`服务未启动：${url}`);
}

test('真实浏览器经 TLS 终结反代与直连 HTTP 的提示和 Cookie', {timeout:120000}, async()=>{
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'rollbook-http-mode-'));
 const go=process.env.GO_BINARY||'go';
 const binary=path.join(dir,'rollbook');
 const cert=path.join(dir,'cert.pem'),key=path.join(dir,'key.pem');
 let app,proxy,browser;
 try{
  execFileSync(go,['build','-o',binary,'./cmd/rollbook']);
  execFileSync('openssl',['req','-x509','-newkey','rsa:2048','-nodes','-keyout',key,'-out',cert,'-days','1','-subj','/CN=localhost'],{stdio:'ignore'});
  const appPort=await freePort(),proxyPort=await freePort();
  app=spawn(binary,['serve'],{env:{...process.env,ROLLBOOK_LISTEN_ADDR:`127.0.0.1:${appPort}`,ROLLBOOK_DB_PATH:path.join(dir,'test.db'),ROLLBOOK_AUTH_PEPPER:'0123456789abcdef0123456789abcdef',ROLLBOOK_BOOTSTRAP_USER:'owner',ROLLBOOK_BOOTSTRAP_PASSWORD:'strong password 12345',ROLLBOOK_ALLOW_INSECURE_HTTP:'true',ROLLBOOK_TRUSTED_PROXY_CIDRS:'127.0.0.1/32'},stdio:'ignore'});
  await ready(`http://127.0.0.1:${appPort}/healthz`);
  proxy=https.createServer({key:fs.readFileSync(key),cert:fs.readFileSync(cert)},(request,response)=>{
   const headers={...request.headers,'x-forwarded-proto':'https'};
   delete headers.forwarded;
   const upstream=http.request({hostname:'127.0.0.1',port:appPort,path:request.url,method:request.method,headers},result=>{
    response.writeHead(result.statusCode,result.headers);result.pipe(response);
   });
   upstream.on('error',error=>{response.writeHead(502);response.end(error.message)});
   request.pipe(upstream);
  });
  await new Promise(resolve=>proxy.listen(proxyPort,'127.0.0.1',resolve));
  browser=await chromium.launch({headless:true});
  const artifactDir=process.env.ROLLBOOK_BROWSER_ARTIFACT_DIR;
  if(artifactDir)fs.mkdirSync(artifactDir,{recursive:true});
  for(const scenario of [{name:'https',url:`https://127.0.0.1:${proxyPort}`,secure:true},{name:'http',url:`http://127.0.0.1:${appPort}`,secure:false}]){
   const context=await browser.newContext({ignoreHTTPSErrors:true});
   const page=await context.newPage();
   const pendingResponses=[];
   page.on('response',response=>{const url=new URL(response.url());if(url.pathname==='/api/changes'&&url.searchParams.get('state')==='待确认')pendingResponses.push(response.status())});
   await page.goto(scenario.url);
   await page.locator('#login-form').waitFor();
   assert.equal(await page.locator('#login .http-mode-notice').isVisible(),!scenario.secure);
   await page.locator('input[name=login]').fill('owner');
   await page.locator('input[name=password]').fill('strong password 12345');
   await page.locator('#login-form button[type=submit]').click();
   await page.locator('#logout').waitFor({state:'visible'});
   const cookies=await context.cookies();
   for(const name of ['rollbook_session','rollbook_csrf']){
    const cookie=cookies.find(item=>item.name===name);
    assert.ok(cookie,`${scenario.name}: missing ${name}`);
    assert.equal(cookie.secure,scenario.secure,`${scenario.name}: ${name} Secure`);
   }
   assert.equal(await page.locator('.sidebar-http-notice').isVisible(),!scenario.secure);
   await page.locator('.modal button').last().click();
   await page.waitForFunction(()=>document.querySelector('.overview-task-count strong')?.textContent==='0');
   assert.equal(await page.locator('.overview-task-count').first().locator('strong').innerText(),'0');
   assert.ok(pendingResponses.length>=2,`${scenario.name}: 总览和侧栏应分别请求待确认变更`);
   assert.ok(pendingResponses.every(status=>status===200),`${scenario.name}: 待确认变更请求应成功`);
   if(artifactDir)await page.screenshot({path:path.join(artifactDir,`${scenario.name}.png`),fullPage:true});
   await context.close();
  }
 }finally{
  if(browser)await browser.close();
  if(proxy)await new Promise(resolve=>proxy.close(resolve));
  if(app&&app.exitCode===null){app.kill();await new Promise(resolve=>app.once('exit',resolve))}
  fs.rmSync(dir,{recursive:true,force:true});
 }
});
