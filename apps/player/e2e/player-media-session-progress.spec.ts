import {expect,test} from '@playwright/test';
import {installPlayerExperienceFixture} from './player-experience-fixture';

installPlayerExperienceFixture(false,false,'iPhone',async (page,title)=>{
  await page.route('**/progress/movie',route=>route.fulfill({status:204}));
  await page.evaluate(settled=>{
    const actions:Record<string,Function>={};
    Object.defineProperty(navigator,'mediaSession',{configurable:true,value:{metadata:null,setActionHandler:(name:string,handler:Function)=>actions[name]=handler,setPositionState:()=>{}}});
    Object.assign(window,{systemMediaAction:(name:string,detail:object)=>actions[name](detail)});
    const video=document.querySelector('video')!;video.dataset.start=settled?'20':'0';
    if(settled)video.setAttribute('src','/media/movie#t=20');
  },title.includes('settled direct'));

});
// The platform invokes these registered public MediaSession callbacks directly.
for(const action of ['seekto','seekbackward','seekforward','stop']) test(`unplayed MediaSession ${action} progress save preserves explicit intent during managed restoration`,async({page})=>{
  const writes:URLSearchParams[]=[];
  page.on('request',request=>{if(new URL(request.url()).pathname==='/progress/movie')writes.push(new URLSearchParams(request.postData()||''));});
  // Automatic room clock correction uses the managed setter; a no-op decoder
  // setter produces no seeking event before the system user action arrives.
  await page.evaluate('setPlayerTime(20)');
  await page.evaluate(action=>{
    (window as Window & {systemMediaAction(name:string,detail:object):void}).systemMediaAction(action,{seekTime:0,seekOffset:20});
    document.querySelector('video')!.dispatchEvent(new Event('seeked'));
  },action);
  await expect.poll(()=>writes.length).toBeGreaterThan(0);
  expect(writes.at(-1)!.get('seconds')).toBe(action==='seekforward'?'40':'0');
  expect(writes.at(-1)!.get('watched')).toBe('false');
});


test('settled direct-fragment progress save retains the first native zero seek',async({page})=>{
  const writes:URLSearchParams[]=[];
  page.on('request',request=>{if(new URL(request.url()).pathname==='/progress/movie')writes.push(new URLSearchParams(request.postData()||''));});
  await expect(page.locator('video')).toHaveJSProperty('readyState',4);
  await expect(page.locator('video')).toHaveJSProperty('currentTime',20);
  await expect(page.locator('video')).toHaveJSProperty('seeking',false);
  await page.locator('video').evaluate((video:HTMLVideoElement)=>{
    video.currentTime=0;video.dispatchEvent(new Event('seeking'));video.dispatchEvent(new Event('seeked'));
  });
  await expect.poll(()=>writes.length).toBe(1);
  expect(writes[0].get('seconds')).toBe('0');
});
