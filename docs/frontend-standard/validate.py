#!/usr/bin/env python3
"""仅本地文件与 sRGB 组合校验；不启动浏览器，不作整站认证。"""
import json,re,sys
from pathlib import Path
P=Path(__file__).resolve().parent
D=json.loads((P/'tokens.json').read_text()); CSS=(P/'tokens.css').read_text(); SPEC=(P/'FRONTEND-SPEC.md').read_text()
checks=[]; rows=[]
def check(label,ok):
 checks.append({'check':label,'pass':bool(ok)})
def luminance(h):
 c=[int(h[i:i+2],16)/255 for i in (1,3,5)]
 c=[x/12.92 if x<=0.04045 else ((x+0.055)/1.055)**2.4 for x in c]
 return sum(x*w for x,w in zip(c,(.2126,.7152,.0722)))
def contrast(a,b):
 a,b=sorted((luminance(a),luminance(b)))
 return (b+.05)/(a+.05)
def pair(theme,fg,bg,minimum,kind):
 c=D['themes'][theme]; value=contrast(c[fg],c[bg]); ok=value+1e-10>=minimum
 rows.append({'theme':theme,'foreground':fg,'background':bg,'fg':c[fg],'bg':c[bg],'ratio':round(value,4),'required':minimum,'purpose':kind,'pass':ok})
check('主题 token 键一一对应',set(D['themes']['light'])==set(D['themes']['dark']))
blocks=re.findall(r'([^{}]+)\{([^{}]*)\}',CSS)
for theme,colors in D['themes'].items():
 block=next(body for selector,body in blocks if f'[data-theme="{theme}"]' in selector)
 found=dict(re.findall(r'--color-([\w-]+):\s*([^;]+);',block))
 check(f'{theme}: CSS 颜色与 JSON 完全一致', found==colors)
 for k,v in colors.items():
  check(f'{theme}.{k} 为 HEX/HEX8',bool(re.fullmatch(r'#[0-9A-F]{6}(?:[0-9A-F]{2})?',v)))
  if theme=='light':
   check(f'{k} 主规范颜色表一致', f'| `{k}` | `{v}` | `{D["themes"]["dark"][k]}` |' in SPEC)
 neutrals=['bg','surface','surface-hover','surface-active']
 for bg in neutrals:
  for fg in ['text','text-muted','link','link-hover','link-active']:
   pair(theme,fg,bg,4.5,'正文/次要文字/链接')
  for status in ['success','warning','danger','info']:
   pair(theme,status+'-text',bg,4.5,'状态说明/错误文字')
  for fg in ['control-border','control-border-hover','control-border-active','focus','danger-text']:
   pair(theme,fg,bg,3,'必要轮廓/焦点/错误轮廓')
 pair(theme,'selected-fg','selected-bg',4.5,'当前项文字/图标')
 for bg in ['primary','primary-hover','primary-active']:
  pair(theme,'primary-fg',bg,4.5,'主按钮标签')
 for bg in ['danger','danger-hover','danger-active']:
  pair(theme,'danger-fg',bg,4.5,'危险按钮标签')
 for status in ['success','warning','danger','info']:
  pair(theme,status+'-fg',status,4.5,'实心状态标签')
  pair(theme,status+'-on-subtle',status+'-subtle',4.5,'浅底状态标签')
 for fg in [f'chart-{i}' for i in range(1,7)]:
  for bg in ['bg','surface']:
   pair(theme,fg,bg,3,'图表必要图形；不代表类别间对比')
for k,v in D['shared'].items():
 check(f'共享 {k} CSS/JSON一致',f'--{k}: {v};' in CSS)
 check(f'共享 {k} 文档/JSON一致',f'| `{k}` | `{v}` |' in SPEC)
for k,v in {'control-h':'36px','control-h-compact':'32px','control-h-touch':'44px','table-row-h':'40px','radius-control':'8px','radius-card':'12px','font-size-body':'14px','line-height-body':'22px','font-size-mobile':'16px'}.items():
 check(f'基线尺寸 {k}',D['shared'][k]==v)
report={'standardVersion':D['meta']['version'],'method':'WCAG 2.x sRGB 相对亮度，线性化阈值0.04045；(L亮+0.05)/(L暗+0.05)；不透明组合；未四舍五入值用于判定','scope':'计划中合法 token 组合，不检查 DOM、真实背景叠加、焦点几何、屏幕阅读器或整站合规','contrastTests':len(rows),'contrastPass':sum(r['pass'] for r in rows),'structuralChecks':len(checks),'structuralPass':sum(c['pass'] for c in checks),'results':rows,'checks':checks,'exclusions':['真正禁用控件 disabled-*','仅装饰 border','遮罩 overlay 与其下方任意图片','图表任意两类颜色之间','产品后续自定义品牌资源/覆盖']}
(P/'validation-results.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
md=f'''# 颜色与文件一致性校验结果

版本1.0 · 2026-11-23（Asia/Shanghai，UTC+8）

复现：`python3 validate.py`（Python 3 标准库，无第三方依赖）。

- 颜色组合：{report['contrastPass']}/{len(rows)} 通过。
- 结构与一致性断言：{report['structuralPass']}/{len(checks)} 通过。
- 最小文字组合：{min(r['ratio'] for r in rows if r['required']==4.5):.4f}:1；最小非文本组合：{min(r['ratio'] for r in rows if r['required']==3):.4f}:1。
- 方法：{report['method']}。
- 范围：{report['scope']}。不宣称整站无障碍认证。

## 必要轮廓修订与主要组合

| 主题 | 组合 | 计算值 | 门槛 | 结果 |
|---|---|---|---|---|
'''
for t in ['light','dark']:
 c=D['themes'][t]
 for fg,bg in [('border','surface'),('control-border','surface'),('control-border','bg'),('text','bg'),('text-muted','surface-active'),('primary-fg','primary'),('selected-fg','selected-bg'),('focus','surface')]:
  val=contrast(c[fg],c[bg]); req=3 if fg in ['border','control-border','focus'] else 4.5
  md+=f'| {t} | {fg} / {bg} | {val:.4f}:1 | {req}:1 | '+('装饰线，不充当必要轮廓' if fg=='border' else ('通过' if val>=req else '失败'))+' |\n'
md+='''
## 修订说明

装饰 border 沿用色值，在白色/暗表面上不足3:1；保留其分割用途，新增 control-border 用于必要控件边界。深色实心按钮采用该轮廓与周围中性背景区分；焦点环以2px背景间隙隔离。完整状态值已显式定义，placeholder不叠加透明度。状态正文用独立 *-text，避免把暗色填色当文字。浅色active表面由初次候选#E2E8F0修正为#EDF2F7：默认链接由4.1926:1提高到4.5889:1，成功说明由4.3917:1提高到4.8068:1。

## 校验边界

禁用项为规范例外，不作为通过组合；装饰分割线、遮罩叠加、品牌覆盖和图表色两两对比不被泛化认证。实际组件仍需检查可见轮廓、字号、层叠、键盘和读屏。全部合法组合实数和每项文件断言见 validation-results.json。
'''
failed=[r for r in rows if not r['pass']]; bad=[c for c in checks if not c['pass']]
if failed or bad: md+='\n## 失败项\n\n```json\n'+json.dumps(failed+bad,ensure_ascii=False,indent=2)+'\n```\n'
(P/'VALIDATION.md').write_text(md)
print(json.dumps({k:report[k] for k in ['contrastTests','contrastPass','structuralChecks','structuralPass']},ensure_ascii=False))
if failed or bad:
 print(json.dumps(failed+bad,ensure_ascii=False,indent=2));sys.exit(1)
