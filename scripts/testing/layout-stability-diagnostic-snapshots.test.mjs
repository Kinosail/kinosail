import {test} from "node:test";
import assert from "node:assert/strict";
import {captureMainSnapshot,summarizeMainChange} from "./layout-stability-diagnostic-snapshots.mjs";

const box = (height, documentY = 0) => ({x:0,y:documentY,documentY,width:390,height});
const node = (id, tag, rect, viewportIntersecting) => ({id,parentId:1,tag,...rect,viewportIntersecting});

test("main growth is dated against paint and identifies visible versus offscreen descendants", () => {
  const initial={time:404,main:box(500),lightDOMElementCount:2,truncated:false,nodes:[node(2,"DIV",box(700,900),false)]};
  const final={time:3756,main:box(605),lightDOMElementCount:3,truncated:false,nodes:[node(2,"DIV",box(805,900),false),node(3,"IMG",box(40,300),true)]};
  const frames=[{time:404,boxes:[{node:"main",...box(500)}]},{time:559,boxes:[{node:"main",...box(605)}]}];
  const result=summarizeMainChange(initial,final,frames,[{name:"first-contentful-paint",time:424}]);
  assert.deepEqual(result.changeInterval,{lastUnchangedFrameTime:404,firstChangedFrameTime:559});
  assert.equal(result.paintRelation,"straddles-first-contentful-paint");
  assert.deepEqual(result.changedDescendants.map(n=>[n.id,n.presence,n.visibility]),[[2,"moved","offscreen"],[3,"added","in-viewport"]]);
});

test("main growth before first paint is distinguished and snapshots stay bounded", () => {
  const nodes=Array.from({length:100},(_,i)=>node(i+2,"DIV",box(i),false));
  const initial={time:100,main:box(500),lightDOMElementCount:101,truncated:true,nodes};
  const finalNodes=nodes.map(n=>({...n,height:n.height+2}));
  const final={time:700,main:box(600),lightDOMElementCount:101,truncated:true,nodes:finalNodes};
  const frames=[{time:100,boxes:[{node:"main",...box(500)}]},{time:200,boxes:[{node:"main",...box(600)}]}];
  const result=summarizeMainChange(initial,final,frames,[{name:"first-contentful-paint",time:424}]);
  assert.deepEqual(result.changeInterval,{lastUnchangedFrameTime:100,firstChangedFrameTime:200});
  assert.equal(result.paintRelation,"before-first-contentful-paint");
  assert.equal(result.initial.nodes.length,48);
  assert.equal(result.final.nodes.length,48);
  assert.equal(result.changedDescendants.length,48);
  assert.equal(result.initial.lightDOMElementCount,101);
  assert.ok(Buffer.byteLength(JSON.stringify(result))<=32*1024);
});

test("main growth is post-paint only when its last unchanged sample is after paint",()=>{
  const initial={time:500,main:box(500),nodes:[],truncated:false};
  const final={time:700,main:box(600),nodes:[],truncated:false};
  const frames=[{time:500,boxes:[{node:"main",...box(500)}]},{time:600,boxes:[{node:"main",...box(600)}]}];
  const result=summarizeMainChange(initial,final,frames,[{name:"first-contentful-paint",time:424}]);
  assert.deepEqual(result.changeInterval,{lastUnchangedFrameTime:500,firstChangedFrameTime:600});
  assert.equal(result.paintRelation,"after-first-contentful-paint");
});

test("tail-sampled late child movement is reported and incomplete samples stay explicit",()=>{
  const nodes=Array.from({length:48},(_,i)=>({...node(i+2,"DIV",box(i*20),false),sample:i<8?"direct-head":i>=40?"depth-tail":"depth-head"}));
  nodes[47]={...node(9000,"SECTION",box(3000),false),sample:"direct-tail"};
  const finalNodes=nodes.map(n=>n.id===9000?{...n,height:n.height+104.89}:n);
  const initial={time:404,main:box(500),lightDOMElementCount:null,truncated:true,nodes};
  const final={time:3756,main:box(605),lightDOMElementCount:null,truncated:true,nodes:finalNodes};
  const frames=[{time:404,boxes:[{node:"main",...box(500)}]},{time:559,boxes:[{node:"main",...box(605)}]}];
  const result=summarizeMainChange(initial,final,frames,[{name:"first-contentful-paint",time:424}]);
  assert.ok(result.changedDescendants.some(n=>n.id===9000&&n.sample==="direct-tail"&&n.presence==="moved"&&n.visibility==="offscreen"));
  assert.equal(result.initial.truncated,true);
  assert.equal(result.initial.nodes.length,48);
  assert.ok(Buffer.byteLength(JSON.stringify(result))<=32*1024);
});

test("byte cap preserves a selected direct-tail node before truncating other sample rows",()=>{
  const max=Number.MAX_VALUE;
  const make=(id,sample,x,tag="SECTION")=>({id,parentId:1,depth:32,tag,x,y:x,documentY:x,width:max,height:x,
    viewportIntersecting:false,sample,position:"absolute",visibility:"visible"});
  const initialNodes=Array.from({length:48},(_,i)=>make(i+2,i<8?"direct-head":i>=40?"depth-tail":"depth-head",max));
  initialNodes[47]=make(9000,"direct-tail",max);
  const finalNodes=initialNodes.map(n=>({...n,x:-max,y:-max,documentY:-max,height:-max}));
  const result=summarizeMainChange({time:1,main:box(max),nodes:initialNodes,truncated:true,lightDOMElementCount:null},
    {time:10,main:box(-max),nodes:finalNodes,truncated:true,lightDOMElementCount:null},
    [{time:1,boxes:[{node:"main",...box(max)}]},{time:2,boxes:[{node:"main",...box(-max)}]}],
    [{name:"first-contentful-paint",time:5}]);
  assert.ok(Buffer.byteLength(JSON.stringify(result))<=32*1024);
  assert.ok(result.initial.nodes.some(n=>n.id===9000&&n.sample==="direct-tail"));
  assert.ok(result.changedDescendants.some(n=>n.id===9000&&n.sample==="direct-tail"));
});

test("DOM capture selects a late direct child and reports its offscreen growth",()=>{
  const original={document:globalThis.document,NodeFilter:globalThis.NodeFilter,innerWidth:globalThis.innerWidth,innerHeight:globalThis.innerHeight,scrollX:globalThis.scrollX,scrollY:globalThis.scrollY,getComputedStyle:globalThis.getComputedStyle};
  const makeElement=(id,y,height,parent=null)=>({nodeName:id===0?"MAIN":"SECTION",parentElement:parent,children:[],childElementCount:0,
    getBoundingClientRect:()=>({x:0,y,width:390,height,right:390,bottom:y+height,left:0,top:y})});
  let mainHeight=500;const main=makeElement(0,0,500),children=Array.from({length:100},(_,i)=>makeElement(i+1,1000+i*20,10,main));
  main.getBoundingClientRect=()=>({x:0,y:0,width:390,height:mainHeight,right:390,bottom:mainHeight,left:0,top:0});
  main.children=children;main.children.item=i=>children[i];main.childElementCount=children.length;
  globalThis.document={querySelector:()=>main,createTreeWalker:()=>{let index=-1;return {nextNode(){index++;return children[index]||null},get currentNode(){return children[index]}}}};
  globalThis.NodeFilter={SHOW_ELEMENT:1};globalThis.innerWidth=390;globalThis.innerHeight=844;globalThis.scrollX=0;globalThis.scrollY=0;globalThis.getComputedStyle=()=>({position:"static",visibility:"visible"});
  try {
    const ids=new WeakMap();let next=0;const identify=node=>{if(!ids.has(node))ids.set(node,++next);return ids.get(node)};
    const pageCapture=Function(`return (${captureMainSnapshot.toString()})`)();
    const initial=pageCapture(identify,404);mainHeight=605;children[99].getBoundingClientRect=()=>({x:0,y:2980,width:390,height:114.89,right:390,bottom:3094.89,left:0,top:2980});
    const final=pageCapture(identify,559);
    const frames=[{time:404,boxes:[{node:"main",...box(500)}]},{time:559,boxes:[{node:"main",...box(605)}]}];
    const result=summarizeMainChange(initial,final,frames,[{name:"first-contentful-paint",time:424}]);
    const late=initial.nodes.find(n=>n.id===identify(children[99]));
    assert.equal(late.sample,"direct-tail");assert.equal(late.viewportIntersecting,false);
    assert.ok(result.changedDescendants.some(n=>n.id===identify(children[99])&&n.presence==="moved"&&n.visibility==="offscreen"));
    assert.equal(initial.truncated,true);assert.equal(initial.lightDOMElementCount,101);
  } finally {
    for(const [key,value] of Object.entries(original)){if(value===undefined)delete globalThis[key];else globalThis[key]=value;}
  }
});
