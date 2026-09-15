import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';

const source = readFileSync(new URL('./static/public-login.js', import.meta.url), 'utf8');
const flush = () => new Promise(resolve => setImmediate(resolve));
function fixture(next = '/') {
  const listeners = new Map(), requests = [], timers = new Map(), navigation = [];
  const elements = Object.fromEntries(['start','cancel','pending','code','status','code-label'].map(name => [name, {
    disabled: name === 'start', hidden: name === 'pending', textContent: '', focused: false,
    addEventListener(event, fn) { listeners.set(`${name}:${event}`,fn); }, focus() { this.focused = true; },
  }]));
  let now = 1000, timerID = 0;
  vm.runInNewContext(source, {
    document: { body: { dataset: { loginNext: next } }, querySelector(selector) {
      if (selector.startsWith('meta')) return {content:'csrf-token'};
      return elements[selector.slice('[data-public-'.length,-1)] || null;
    } },
    window: { addEventListener(event, fn) { listeners.set(event,fn); } },
    location: { origin:'https://family.duckdns.org',replace(value){navigation.push(value);} },
    URL, AbortSignal, Date: {now:()=>now},
    setTimeout(fn) { const id=++timerID; timers.set(id,fn); return id; },
    clearTimeout(id) {timers.delete(id);},
    fetch(url, options) {return new Promise((resolve,reject)=>requests.push({url,options,resolve,reject}));},
  });
  return { elements,requests,navigation,timers,
    click(name){return listeners.get(`${name}:click`)();},
    event(name,value={}){listeners.get(name)(value);},
    advance(milliseconds){now+=milliseconds;},
    tick(){const [id,fn]=timers.entries().next().value;timers.delete(id);fn();},
  };
}
function respond(request,status,body='') {request.resolve({status,ok:status>=200&&status<300,text:async()=>typeof body==='string'?body:JSON.stringify(body)});}
async function start(f) {
  f.click('start'); respond(f.requests.at(-1),201,{code:'123456',expiresIn:300}); await flush();
}
test('code request has no secret in the URL or body and waits for approval before navigating',async()=>{
  const f=fixture('/watch/movie'); await start(f);
  assert.equal(f.requests[0].url,'/auth/quick-connect');
  assert.equal(f.requests[0].options.method,'POST');
  assert.equal(f.requests[0].options.credentials,'same-origin');
  assert.equal(f.requests[0].options.headers['X-Kinosail-CSRF'],'csrf-token');
  assert.equal(f.requests[0].options.body,undefined);
  assert.equal(f.elements.code.textContent,'123456');
  assert.equal(f.elements.pending.hidden,false);
  assert.equal(f.elements['code-label'].focused,true);
  assert.equal(f.navigation.length,0);
  f.tick(); respond(f.requests.at(-1),202); await flush();
  assert.equal(f.navigation.length,0); assert.equal(f.timers.size,1);
  f.tick(); respond(f.requests.at(-1),204); await flush();
  assert.deepEqual(f.navigation,['https://family.duckdns.org/watch/movie']);
});
for(const body of [null,{},[],{code:123456,expiresIn:300},{code:'<script>',expiresIn:300},{code:'123456',expiresIn:0},{code:'123456',expiresIn:Infinity},{code:'123456',expiresIn:3601},{code:'123456',expiresIn:300,secret:'never-render'},'x'.repeat(1025)]) {
  test(`malformed response fails without displaying a code: ${JSON.stringify(body)?.slice(0,75)}`,async()=>{
    const f=fixture();f.click('start');respond(f.requests[0],201,body);await flush();
    assert.equal(f.elements.pending.hidden,true);assert.equal(f.elements.code.textContent,'');assert.equal(f.elements.start.disabled,false);assert.equal(f.timers.size,0);
  });
}
test('expiry clears the code and stops polling',async()=>{
  const f=fixture();await start(f);f.advance(301000);f.tick();
  assert.match(f.elements.status.textContent,/expired/);assert.equal(f.elements.code.textContent,'');assert.equal(f.requests.length,1);
});
test('cancel withdraws the request and ignores a late poll',async()=>{
  const f=fixture();await start(f);f.tick();const poll=f.requests.at(-1);
  f.click('cancel');respond(f.requests.at(-1),204);await flush();
  respond(poll,204);await flush();
  assert.equal(f.requests.at(-1).url,'/auth/quick-connect/cancel');assert.equal(f.navigation.length,0);assert.equal(f.timers.size,0);assert.match(f.elements.status.textContent,/canceled/);assert.equal(f.elements.start.focused,true);
});
test('failed cancellation tells the user not to approve the outstanding code',async()=>{
  const f=fixture();await start(f);f.click('cancel');f.requests.at(-1).reject(new Error('offline'));await flush();
  assert.match(f.elements.status.textContent,/Do not approve/);assert.equal(f.elements.pending.hidden,true);
});
for(const status of [404,409,429,500]) {
  test(`poll failure ${status} clears the code and permits recovery`,async()=>{
    const f=fixture();await start(f);f.tick();respond(f.requests.at(-1),status);await flush();
    assert.equal(f.elements.code.textContent,'');assert.equal(f.elements.start.disabled,false);assert.equal(f.timers.size,0);assert.equal(f.navigation.length,0);
  });
}
test('cross-origin return destinations stay on the Server',async()=>{
  const f=fixture('https://attacker.example');await start(f);f.tick();respond(f.requests.at(-1),204);await flush();assert.deepEqual(f.navigation,['/']);
});
test('leaving stops polling and returning from browser history offers a fresh code',async()=>{
  const f=fixture();await start(f);f.event('pagehide');assert.equal(f.timers.size,0);
  f.event('pageshow',{persisted:true});assert.equal(f.elements.pending.hidden,true);assert.equal(f.elements.start.disabled,false);
});
