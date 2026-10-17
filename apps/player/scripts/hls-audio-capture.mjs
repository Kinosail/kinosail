import {createRequire} from 'node:module';
import {join} from 'node:path';
import {writeFile} from 'node:fs/promises';
const require = createRequire(join(process.cwd(), 'apps/player/e2e/package.json'));
const {chromium} = require('@playwright/test');
const origin = process.env.QA_RESUME_ORIGIN, token = process.env.QA_RESUME_TOKEN;
const id = process.env.QA_RESUME_ITEM, output = process.env.QA_AUDIO_OUTPUT;
const point = Number(process.env.QA_AUDIO_CAPTURE_POINT);
const span = Number(process.env.QA_AUDIO_CAPTURE_SPAN || '3');
const trim = process.env.KINOSAIL_DIAG_LEGACY !== '1';
const rate = Number(process.env.QA_AUDIO_SAMPLE_RATE || '48000');
if (![8000, 32000, 44100, 48000, 96000].includes(rate)) throw new Error('capture_rate_bound');
if (!Number.isFinite(span) || span < 1 || span > 10) throw new Error('capture_span_bound');
if (!Number.isFinite(point) || point < 1 || point > 120) throw new Error('capture_point_bound');
const channel = process.env.PLAYWRIGHT_CHANNEL || (!process.env.CI && process.platform === 'darwin' ? 'chrome' : undefined);
const browser = await chromium.launch({channel});
const context = await browser.newContext({bypassCSP: true,
  userAgent: 'Mozilla/5.0 Chrome/130.0.0.0 Safari/537.36',
  extraHTTPHeaders: {Authorization: `Bearer ${token}`}});
const result = {boundary: 'Actual populated watch page; diagnostic CSP exception permits only the recording module; no security acceptance',
  point, span, rate, browserVersion: browser.version(), responses: []};
try {
  const progress = await context.request.put(`${origin}/api/v1/items/${id}/progress`, {data: {seconds: 12.5, watched: false}});
  if (progress.status() !== 200) throw new Error('progress_status');
  const page = await context.newPage();
  page.on('response', response => {
    const path = new URL(response.url()).pathname;
    if (path.startsWith('/hls/')) result.responses.push({
      status: response.status(), segment: path.match(/\/(segment-\d{5}\.m4s)$/)?.[1],
      kind: path.endsWith('.m3u8') ? 'playlist' : path.endsWith('.m4s') ? 'fragment' : 'initialization'});
  });
  await page.goto(`${origin}/watch/${id}?compatible=1`);
  await page.waitForFunction(() => document.querySelector('video')?.readyState >= 2);
  const capture = await page.evaluate(async ({point, span, rate, trim}) => {
    const video = document.querySelector('video'), events = [], checkpoints = [];
    const rawTime = Object.getOwnPropertyDescriptor(HTMLMediaElement.prototype, 'currentTime').get;
    let ac;
    const checkpoint = (stage, renderFrame) => checkpoints.push({stage, renderFrame, media:video.currentTime,
      rawMedia:rawTime.call(video), audio:ac?.currentTime, wall:performance.now(),
      readyState:video.readyState, seeking:video.seeking, paused:video.paused});
    if (video.dataset.compatibilityMode !== 'audio-transcode') throw new Error('capture_mode');
    checkpoint('ready');
    await new Promise((resolve, reject) => {
      let callback, previous;
      const timer = setTimeout(() => {
        video.cancelVideoFrameCallback(callback);
        reject(new Error('initial_presentation_timeout'));
      }, 15000);
      const frame = (_now, metadata) => {
        if (previous !== undefined && metadata.mediaTime > previous && !video.paused && !video.seeking) {
          clearTimeout(timer); resolve();
        } else { previous = metadata.mediaTime; callback = video.requestVideoFrameCallback(frame); }
      };
      callback = video.requestVideoFrameCallback(frame);
      video.play().catch(error => { clearTimeout(timer); video.cancelVideoFrameCallback(callback); reject(error); });
    });
    checkpoint('initial-presenting');
    video.pause();
    checkpoint('paused');
    for (const name of ['seeking', 'seeked', 'waiting', 'stalled', 'ended', 'ratechange', 'playing', 'pause', 'canplay'])
      video.addEventListener(name, event => {if(events.length < 128) events.push({type:event.type, media:video.currentTime, wall:performance.now()});});
    await new Promise((resolve,reject) => {
      const timer = setTimeout(() => reject(new Error('capture_seek_timeout')),15000);
      const seeked=()=>{
        if(video.paused && !video.seeking && Math.abs(video.currentTime-point)<=.001){
          video.removeEventListener('seeked',seeked);clearTimeout(timer);resolve();
        }
      };
      video.addEventListener('seeked',seeked);
      video.currentTime = point;
    });
    checkpoint('seek-complete');
    ac = new AudioContext({sampleRate:rate});
    const source = ac.createMediaElementSource(video);
    await ac.suspend();
    const module = URL.createObjectURL(new Blob([`
      class Capture extends AudioWorkletProcessor {
        constructor(options) {super();this.limit=options.processorOptions.limit;this.trim=options.processorOptions.trim;this.pcm=[];this.blocks=[];this.leading=0;this.active=true;
          this.port.onmessage=()=>{this.active=false;this.port.postMessage({pcm:this.pcm,blocks:this.blocks,leadingSilentFrames:this.leading});};}
        process(inputs) {const data=inputs[0]?.[0];
          if(this.active && data && this.pcm.length+data.length<=this.limit && this.blocks.length<65536) {
            if(this.blocks.length===0)this.port.postMessage({first:true,frame:currentFrame});
            this.blocks.push({frame:currentFrame,length:data.length});
            if(this.trim && this.pcm.length===0 && data.every(value=>value===0))this.leading+=data.length;
            else {
              if(this.pcm.length===0)this.port.postMessage({onset:true,frame:currentFrame});
              for(let i=0;i<data.length;i++)this.pcm.push(data[i]);
            }
          }
          return true;
        }
      }
      registerProcessor('qa-long-audio',Capture);`],{type:'text/javascript'}));
    await ac.audioWorklet.addModule(module);URL.revokeObjectURL(module);
    const worklet = new AudioWorkletNode(ac,'qa-long-audio',{channelCount:1,channelCountMode:'explicit',
      processorOptions:{limit:Math.ceil((span+1)*rate),trim}});
    const done = new Promise(resolve => worklet.port.onmessage = event => {
      if(event.data.first)checkpoint('first-worklet-message',event.data.frame);
      else if(event.data.onset)checkpoint('first-audio-message',event.data.frame);else resolve(event.data);
    });
    source.connect(worklet);worklet.connect(ac.destination);
    video.muted = false;video.volume = 1;
    checkpoint('play-request');await video.play();checkpoint('play-resolved');
    await ac.resume();checkpoint('audio-resumed');
    await new Promise((resolve,reject) => {
      const timer=setTimeout(()=>reject(new Error('capture_play_timeout')),15000);
      const tick=()=>{if(video.currentTime>=Math.min(point+span,video.duration)||video.ended){clearTimeout(timer);resolve();}else setTimeout(tick,20);};tick();
    });
    video.pause();worklet.port.postMessage('stop');const recorded=await done;
    worklet.disconnect();source.disconnect();await ac.close();
    return {...recorded,events,checkpoints,sampleRate:ac.sampleRate,currentTime:video.currentTime,duration:video.duration,ended:video.ended};
  },{point,span,rate,trim});
  const {pcm,...facts}=capture;
  await writeFile(output+'.f32',Buffer.from(new Float32Array(pcm).buffer));
  result.capture={...facts,sampleCount:pcm.length};
  result.passed=pcm.length>rate && result.responses.every(row=>row.status<400);
} catch(error) {result.passed=false;result.failureClass=error instanceof Error && /^[a-z_]+$/.test(error.message) ? error.message : 'capture_failure';}
finally {await context.close();await browser.close();}
await writeFile(output+'.json',JSON.stringify(result,null,2)+'\n');
console.log(JSON.stringify({passed:result.passed,failureClass:result.failureClass,sampleCount:result.capture?.sampleCount}));
process.exitCode=result.passed?0:1;
