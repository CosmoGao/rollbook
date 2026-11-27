const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

test('飞书公开端口差异：浅深主题与宽窄屏提示',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  for(const theme of ['light','dark'])for(const width of [1280,390]){
   const page=await browser.newPage({locale:'zh-CN',viewport:{width,height:900}});
   let signedIn=true;
   await page.addInitScript(value=>localStorage.setItem('rollbook.theme',value),theme);
   await page.route('https://rollbook.example:5443/**',route=>{
    const p=new URL(route.request().url()).pathname;
    if(!p.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
    if(p==='/api/config')return route.fulfill({json:{allow_insecure_http:true,http_risk:false,login_methods:{local:true,ldap:false,oidc:false,feishu:false}}});
    if(p==='/api/version')return route.fulfill({json:{version:'test'}});
    if(p==='/api/auth/session')return route.fulfill(signedIn?{json:{login:'owner',can_change_password:true}}:{status:401,json:{code:'unauthorized'}});
    if(p==='/api/auth/settings')return route.fulfill({json:{providers:[{kind:'feishu',enabled:false,source:'page',shadowed:false,config:{},secret_configured:false}],local_enabled:true,public_origin:'https://rollbook.example'}});
    if(p==='/api/auth/settings/feishu'&&route.request().method()==='PUT')return route.fulfill({status:422,json:{code:'validation',message:'校验失败',fields:[{field:'client_id',reason:'App ID 必填'}]}});
    return route.fulfill({status:404,json:{code:'not_found'}});
   });
   await installBundle(page);
   await page.goto('https://rollbook.example:5443/settings?section=feishu');
   await page.getByRole('heading',{name:'飞书'}).waitFor();
   await page.getByRole('alert').getByText(/浏览器公开地址与服务端识别的地址不一致/).waitFor();
   assert.equal(await page.getByLabel('回调 URL').inputValue(),'https://rollbook.example:5443/api/auth/feishu/callback');
   assert.equal(await page.getByLabel('回调 URL').getAttribute('readonly'),'');
   await page.getByRole('button',{name:'保存身份源'}).click();
   await page.getByRole('alert').getByText(/App ID: App ID 必填/).waitFor();
   assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth-document.documentElement.clientWidth),0);
   const out=process.env.AUTH_ORIGIN_SCREENSHOTS;
   if(out){fs.mkdirSync(out,{recursive:true});await page.screenshot({path:path.join(out,`feishu-origin-${theme}-${width}.png`),fullPage:true})}
   signedIn=false;
   await page.goto('https://rollbook.example:5443/');
   await page.locator('#login-form').waitFor();
   assert.equal(await page.getByRole('link',{name:'使用飞书登录'}).count(),0);
   await page.close();
  }
 }finally{await browser.close()}
});
