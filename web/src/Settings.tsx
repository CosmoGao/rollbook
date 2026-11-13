import React from 'react';
import { createRoot, Root } from 'react-dom/client';
import { useTranslation } from 'react-i18next';
import { Button } from './components/ui/button';

let root:Root|undefined;
export function unmountSettings(){if(root){root.unmount();root=undefined}}
export function mountSettings(target:Element,onBackup:()=>void,onPassword:()=>void,onLogoutAll:()=>void,onLanguage:(value:string)=>void){
  unmountSettings();root=createRoot(target);
  root.render(<Settings onBackup={onBackup} onPassword={onPassword} onLogoutAll={onLogoutAll} onLanguage={onLanguage}/>);
}
function Settings({onBackup,onPassword,onLogoutAll,onLanguage}:{onBackup:()=>void;onPassword:()=>void;onLogoutAll:()=>void;onLanguage:(value:string)=>void}){
  const {t,i18n}=useTranslation();
  return <>
    <div className="hero"><h1>{t('settings')}</h1><p className="muted">{t('账号、备份与恢复')}</p></div>
    <section className="section"><h2>{t('账号与数据')}</h2><div className="controls"><Button variant="outline" onClick={onBackup}>{t('备份与恢复')}</Button><Button variant="outline" onClick={onPassword}>{t('改密码')}</Button><Button variant="destructive" onClick={onLogoutAll}>{t('退出所有设备')}</Button></div></section>
    <section className="section"><h2>{t('语言')}</h2><select className="field" aria-label={t('语言')} value={i18n.language} onChange={event=>onLanguage(event.target.value)}><option value="zh-CN">中文</option><option value="en">English</option></select></section>
  </>;
}
