import test from 'node:test';
import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import {stripTypeScriptTypes} from 'node:module';
import {runInNewContext} from 'node:vm';
import * as boundary from '../fixture-response.mjs';
let setupBoundary = {};
try {setupBoundary = await import('../fixture-setup.mjs');} catch (error) {if (error.code !== 'ERR_MODULE_NOT_FOUND') throw error;}
const origin='http://127.0.0.1:39061';
const secret='ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';
const enrollment={token:'Token-123_abc',expiresIn:2592000,mfaEnrollmentRequired:true,totp:{secret,
 uri:`otpauth://totp/Kinosail:Owner?secret=${secret}&issuer=Kinosail&digits=6&period=30`,
 recoveryCodes:Array.from({length:10},(_,i)=>`${i.toString(16).toUpperCase().padStart(4,'0')}-ABCD-0123-4567`)}};
function peer({data=enrollment,raw=JSON.stringify(data),status=201,url=origin+'/api/v1/setup',type='application/json',declared}={}) {
 const effects={initial:0,downstream:0,totp:0,cookies:0,navigate:0,save:0};let callback;
 const code=stripTypeScriptTypes(readFileSync(new URL('./owner.setup.e2e.ts',import.meta.url),'utf8')).replace(/^import .*;\n/gm,'');
 const setupResponse=new Response(raw,{status,headers:{'Content-Type':type,...(declared===undefined?{}:{'Content-Length':declared})}});
 Object.defineProperty(setupResponse,'url',{value:url});
 const fetch=async(target,options)=>{
  if(new URL(target).pathname==='/api/v1/setup'){effects.initial++;return setupResponse;}
  effects.downstream++;assert.equal(options.headers.Authorization,'Bearer '+enrollment.token);
  return {status:200,url:target.toString()};
 };
 runInNewContext(code,{test:{setup:(title,options,fn)=>{callback=fn;}},...boundary,...setupBoundary,fetch,URL,AbortSignal,
  totp:value=>{effects.totp++;assert.equal(value,secret);return '123456';},expect:value=>({toBe:expected=>assert.equal(value,expected),toHaveURL:async expected=>assert.equal(expected,'/settings')})});
 const app={baseUrl:origin,open:async()=>{effects.navigate++;}};
 return {effects,run:()=>callback({app,browser:{setCookies:async()=>{effects.cookies++;}},session:{save:async()=>{effects.save++;}}})};
}
test('valid owned enrollment completes original setup, MFA, onboarding and session sequence',async()=>{
 const p=peer();await p.run();assert.deepEqual(p.effects,{initial:1,downstream:2,totp:1,cookies:1,navigate:1,save:1});
});
const change=fn=>{const d=structuredClone(enrollment);fn(d);return d;};
for(const [name,options] of [
 ['foreign origin',{url:'http://127.0.0.1:39062/api/v1/setup'}],['wrong path',{url:origin+'/api/v1/other'}],
 ['non-success',{status:200}],['malformed',{raw:'{'}],['trailing',{raw:'{}{}'}],['UTF8',{raw:Uint8Array.from([0xc3,0x28])}],
 ['duplicate key',{raw:JSON.stringify(enrollment).replace('"token":','"token":"Other","token":')}],
 ['escaped duplicate',{raw:JSON.stringify(enrollment).replace('"token":','"\\u0074oken":"Other","token":')}],
 ['oversized actual',{raw:' '.repeat(524289)}],['oversized declared',{declared:'524289'}],['bad declared',{declared:'-1'}],
 ['wrong type',{type:'text/html'}],['missing',{data:{}}],['unknown',{data:{...enrollment,extra:true}}],
 ['nonfinite',{raw:JSON.stringify(enrollment).replace('2592000','1e400')}],['depth',{raw:'['.repeat(10)+'0'+']'.repeat(10)}],
 ['missing token',{data:change(d=>delete d.token)}],['blank token',{data:change(d=>d.token=' ')}],
 ['nonASCII token',{data:change(d=>d.token='é')}],['oversized token',{data:change(d=>d.token='x'.repeat(257))}],
 ['wrong expiry',{data:change(d=>d.expiresIn=0)}],['wrong enrollment flag',{data:change(d=>d.mfaEnrollmentRequired=false)}],
 ['string enrollment flag',{data:change(d=>d.mfaEnrollmentRequired='true')}],['missing enrollment',{data:change(d=>delete d.totp)}],
 ['unknown enrollment',{data:change(d=>d.totp.extra=true)}],['invalid secret',{data:change(d=>d.totp.secret='not-base32')}],
 ['URI secret disagreement',{data:change(d=>d.totp.uri=d.totp.uri.replace(secret,'A'.repeat(32)))}],
 ['missing recovery',{data:change(d=>delete d.totp.recoveryCodes)}],['recovery cardinality',{data:change(d=>d.totp.recoveryCodes.pop())}],
 ['recovery duplicate',{data:change(d=>d.totp.recoveryCodes[1]=d.totp.recoveryCodes[0])}],
 ['recovery malformed',{data:change(d=>d.totp.recoveryCodes[0]='raw')}]
]) test('registered setup rejects '+name+' before downstream actions',async()=>{
 const p=peer(options);await assert.rejects(p.run());assert.equal(p.effects.initial,1);
 assert.deepEqual({...p.effects,initial:0},{initial:0,downstream:0,totp:0,cookies:0,navigate:0,save:0});
});
