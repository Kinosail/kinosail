import assert from 'node:assert/strict';
import {readFileSync} from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
const source=readFileSync(new URL('./static/passkeys.js',import.meta.url),'utf8');
function fixture(search, serverNext='/') {
  const navigations=[];
  const credential={parseRequestOptionsFromJSON:value=>value};
  const context=vm.createContext({
    document:{body:{dataset:{}},querySelector:()=>null},
    window:{isSecureContext:true,PublicKeyCredential:credential},PublicKeyCredential:credential,
    navigator:{credentials:{get:async()=>({id:'verified'})}},
    localStorage:{getItem:()=>null,setItem(){}},URL,URLSearchParams,
    location:{origin:'https://kino.test',search,replace:value=>navigations.push(value)},
    fetch:async path=>path.endsWith('/begin')?new Response(JSON.stringify({publicKey:{}})):new Response(null,{status:204,headers:{'X-Kinosail-Login-Next':serverNext}}),
  });
  vm.runInContext(source+'\nglobalThis.normalize=safeLoginNext;',context);
  return {context,navigations};
}
test('login honors the Server destination even with an offer query',async()=>{
  for(const search of ['?passkey=offer&next=https://untrusted.example','?setup=1&next=/settings','?mfa=required&next=/settings']) {
    const f=fixture(search,'/library');await f.context.ceremony('login');assert.deepEqual(f.navigations,['/library']);
  }
});
test('post-login destinations remain bounded and same-origin',()=>{
  const f=fixture('');
  for(const next of ['https://other.example','//other.example','/\\other.example','/\nother','/bad path','/'+ 'x'.repeat(2048)]) assert.equal(f.context.normalize(next),'/');
  assert.equal(f.context.normalize('/watch/movie?resume=1#player'),'/watch/movie?resume=1#player');
});
