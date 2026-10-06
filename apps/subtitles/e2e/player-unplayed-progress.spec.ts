import {expect,test} from '@playwright/test';
import {playerSource} from './static-sources';
// A controlled source replacement isolates decoder restoration from user seeks.
test('unplayed source replacement restores its position without creating progress',async({page})=>{
  const writes:URLSearchParams[]=[];
  await page.route('**/progress/movie',route=>{writes.push(new URLSearchParams(route.request().postData()||''));return route.fulfill({status:204});});
  await page.route('https://127.0.0.1:38128/',route=>route.fulfill({contentType:'text/html',body:`<video data-playback-policy="compatible" data-playback-override="true" data-start="3" data-direct="/media/movie" data-hls="/movie.m3u8" data-progress="/progress/movie"></video><div data-quality-control hidden><select aria-label="Stream quality" data-quality></select><span data-quality-state></span></div><button data-seek="0">First chapter</button>`}));
  await page.goto('https://127.0.0.1:38128/');
  await page.evaluate(()=>{
    const video=document.querySelector('video')!;let position=3,src='';
    Object.defineProperties(video,{
      currentTime:{get:()=>position,set:(value:number)=>{position=value;queueMicrotask(()=>{video.dispatchEvent(new Event('seeking'));video.dispatchEvent(new Event('seeked'));});}},
      readyState:{value:4},duration:{value:100},paused:{value:true},
      src:{get:()=>src,set:(value:string)=>{src=value;position=0;}},currentSrc:{get:()=>src},load:{value:()=>{}},
    });
    class FakeHls{
      static isSupported=()=>true;static Events={MANIFEST_PARSED:'manifest',LEVEL_SWITCHED:'switched',ERROR:'error'};static ErrorTypes={NETWORK_ERROR:'network',MEDIA_ERROR:'media'};
      handlers=new Map();levels=[];autoLevelEnabled=true;
      constructor(){Object.assign(window,{instance:this});}on(event:string,handler:Function){this.handlers.set(event,handler);}loadSource(){}attachMedia(){}destroy(){}
      manifest(){this.handlers.get('manifest')('manifest',{levels:[]});}
    }
    Object.assign(window,{Hls:FakeHls});
  });
  await page.addScriptTag({content:playerSource});
  await page.waitForFunction(()=>Boolean((window as Window & {instance?:unknown}).instance));
  await page.evaluate(()=>(window as Window & {instance:{manifest():void}}).instance.manifest());
  await page.getByLabel('Stream quality').selectOption('original');
  await page.locator('video').dispatchEvent('loadedmetadata');
  await expect(page.locator('video')).toHaveJSProperty('currentTime',3);
  await page.waitForTimeout(100);
  expect(writes).toEqual([]);
  await page.getByRole('button',{name:'First chapter'}).click();
  await expect.poll(()=>writes.length).toBe(1);
  expect(writes[0].get('seconds')).toBe('0');
  expect(writes[0].get('watched')).toBe('false');
});
