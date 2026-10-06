import {expect, test} from '@playwright/test';
import {playerSource} from './static-sources';
// Controlled decoder clocks supplement the real-process public saved-state journey.
for (const clock of [0, 0.5]) test(`unplayed settled native HLS clock ${clock} only saves an explicit zero chapter seek`, async ({page}) => {
  const writes: URLSearchParams[] = [];
  await page.route('**/progress/movie', route => { writes.push(new URLSearchParams(route.request().postData() || '')); return route.fulfill({status:204}); });
  await page.route('https://127.0.0.1:38127/', route => route.fulfill({contentType:'text/html',body:`<body><div class="media-stage"><video data-progress="/progress/movie" data-start="180.05" data-duration="7200" data-hls="/hls/movie/p/a-a0-s0-none-t0-b0/index.m3u8"></video><div data-player-status><span data-player-message>Loading video…</span><progress data-buffered></progress></div></div><button data-seek="0">First chapter</button></body>`}));
  await page.goto('https://127.0.0.1:38127/');
  await page.evaluate(() => {
    Object.defineProperty(navigator,'vendor',{configurable:true,value:'Apple Computer, Inc.'});
    const video=document.querySelector('video')!;
    let time=0,ready=0,source='';
    Object.defineProperty(HTMLMediaElement.prototype,'currentTime',{configurable:true,get:()=>time,set:(value:number)=>{time=value;if(ready)queueMicrotask(()=>{video.dispatchEvent(new Event('seeking'));video.dispatchEvent(new Event('seeked'));});}});
    Object.defineProperties(video,{
      duration:{configurable:true,value:7200},readyState:{get:()=>ready},networkState:{value:2},paused:{value:true},error:{value:null},buffered:{get:()=>({length:1,start:()=>0,end:()=>5})},seekable:{value:{length:0}},
      canPlayType:{value:(type:string)=>type==='application/vnd.apple.mpegurl'?'probably':''},src:{get:()=>source,set:(value:string)=>{source=value;time=0;}},currentSrc:{get:()=>source},load:{value:()=>{}},pause:{value:()=>video.dispatchEvent(new Event('pause'))},
    });
    Object.assign(window,{settleDecoder:(value:number)=>{time=value;ready=4;}});
  });
  await page.addScriptTag({content:playerSource});
  await page.locator('video').evaluate((video,time)=>{
    (window as Window & {settleDecoder(value:number):void}).settleDecoder(time);
    for(const name of ['loadedmetadata','canplay','pause','kinosail:page-exit'])video.dispatchEvent(new Event(name));
  },clock);
  await expect(page.locator('video')).toHaveJSProperty('paused',true);
  await page.waitForTimeout(100);
  expect(writes).toEqual([]);
  await page.getByRole('button',{name:'First chapter'}).click();
  await page.locator('video').evaluate(video=>{
    (window as Window & {settleDecoder(value:number):void}).settleDecoder(0);
    for(const name of ['loadedmetadata','canplay','seeking','seeked'])video.dispatchEvent(new Event(name));
  });
  await expect.poll(()=>writes.length).toBeGreaterThan(0);
  expect(writes.at(-1)!.get('seconds')).toBe('0');
  expect(writes.at(-1)!.get('watched')).toBe('false');
});
