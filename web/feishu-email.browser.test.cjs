const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const {chromium}=require('playwright');
const {installBundle}=require('./browser.bundle.cjs');

test('个人设置显示飞书绑定与邮箱，并展示中英文错误',async()=>{
 const browser=await chromium.launch({headless:true});
 try{
  const page=await browser.newPage({locale:'zh-CN'});let email='';let saved=0;let signedIn=true;
  await page.route('https://rollbook.example:5443/**',route=>{
   const p=new URL(route.request().url()).pathname;
   if(!p.startsWith('/api/'))return route.fulfill({contentType:'text/html',body:fs.readFileSync(path.join(__dirname,'dist','index.html'))});
   if(p==='/api/config')return route.fulfill({json:{login_methods:{local:true,ldap:false,oidc:false,feishu:true},http_risk:false}});
   if(p==='/api/version')return route.fulfill({json:{version:'test'}});
   if(p==='/api/auth/session')return route.fulfill(signedIn?{json:{login:'owner',auth_provider:'local',can_change_password:true}}:{status:401,json:{code:'unauthorized'}});
   if(p==='/api/auth/profile'&&route.request().method()==='PUT'){
    const value=JSON.parse(route.request().postData()).email;
    if(value==='bad@example.com')return route.fulfill({status:409,json:{code:'email_taken',message:'邮箱已被使用',fields:[{field:'email',reason:'邮箱已被使用'}]}});
    email=value;saved++;return route.fulfill({json:{saved:true}});
   }
   if(p==='/api/auth/profile')return route.fulfill({json:{email,feishu_enabled:true,feishu_bound:false,local_session:true}});
   return route.fulfill({json:[]});
  });
  await installBundle(page);await page.goto('https://rollbook.example:5443/settings/personal');
  await page.getByRole('heading',{name:'邮箱与飞书'}).waitFor();
  assert.equal(await page.getByRole('link',{name:'绑定飞书'}).getAttribute('href'),'/api/auth/feishu/bind/start');
  await page.getByRole('textbox',{name:'邮箱'}).fill('owner@example.com');
  await page.getByRole('button',{name:'保存邮箱'}).click();
  await page.getByText('邮箱已保存').waitFor();assert.equal(saved,1);
  await page.getByRole('textbox',{name:'邮箱'}).fill('bad@example.com');
  await page.getByRole('button',{name:'保存邮箱'}).click();
  await page.getByRole('alert').getByText(/邮箱已被使用/).waitFor();
  await page.getByLabel('Language / 语言').last().selectOption('en');
  await page.getByRole('link',{name:'Bind Feishu'}).waitFor();
  signedIn=false;await page.goto('https://rollbook.example:5443/?auth_error=unmatched');
  await page.getByRole('alert').getByText('No matching email. Sign in locally, then bind Feishu.').waitFor();
 }finally{await browser.close()}
});
