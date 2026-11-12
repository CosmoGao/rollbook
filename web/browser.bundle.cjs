const fs=require('node:fs');
const path=require('node:path');
const dist=path.join(__dirname,'dist');
const types={'.js':'application/javascript','.css':'text/css','.svg':'image/svg+xml','.png':'image/png','.webmanifest':'application/manifest+json'};
async function installBundle(page){
 await page.route(/^https?:\/\/[^/]+\/assets\//,route=>{
  const pathname=new URL(route.request().url()).pathname;
  const file=path.resolve(dist,'.'+pathname);
  if(!file.startsWith(dist+path.sep)||!fs.existsSync(file))return route.fulfill({status:404,body:''});
  return route.fulfill({status:200,contentType:types[path.extname(file)]||'application/octet-stream',body:fs.readFileSync(file)});
 });
}
module.exports={installBundle};
