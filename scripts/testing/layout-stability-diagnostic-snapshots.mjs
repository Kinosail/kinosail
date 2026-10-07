// Text-free, bounded evidence used only after the existing main-box predicate fires.
const MAX_NODES = 48;
const MAX_DETAIL_BYTES = 32 * 1024;
const MAIN_DELTA_KEYS = ["x", "documentY", "width", "height"];
const NODE_DELTA_KEYS = ["x", "y", "documentY", "width", "height"];
const changed = (a, b, keys) => keys.filter(key => Number.isFinite(a?.[key]) && Number.isFinite(b?.[key]) && Math.abs(a[key] - b[key]) > 1);
const rect = value => ({x:Number.isFinite(value?.x)?value.x:null,y:Number.isFinite(value?.y)?value.y:null,
  documentY:Number.isFinite(value?.documentY)?value.documentY:null,width:Number.isFinite(value?.width)?value.width:null,
  height:Number.isFinite(value?.height)?value.height:null});
const safeNode = node => ({id:Number.isInteger(node?.id)?node.id:null,parentId:Number.isInteger(node?.parentId)?node.parentId:null,
  depth:Number.isInteger(node?.depth)?Math.max(0,Math.min(32,node.depth)):0,
  tag:typeof node?.tag==="string"?node.tag.slice(0,16):"other",...rect(node),viewportIntersecting:Boolean(node?.viewportIntersecting),
  sample:["main","direct-head","direct-tail","depth-head","depth-tail"].includes(node?.sample)?node.sample:"other",
  position:["static","relative","absolute","fixed","sticky"].includes(node?.position)?node.position:"other",
  visibility:["visible","hidden","collapse"].includes(node?.visibility)?node.visibility:"other"});

// Called in the page context only after existing layout sampling has started.
export function captureMainSnapshot(identify, sampleTime) {
  const MAX_MAIN_SNAPSHOT_NODES=48,MAX_DESCENDANT_VISITS=4096;
  const main=document.querySelector("main");
  if(!main)return {time:Number.isFinite(sampleTime)?sampleTime:performance.now(),main:null,nodes:[],truncated:false,
    scrollX,scrollY,lightDOMElementCount:0,traversedDescendantCount:0,directChildCount:0};
  const rect=node=>{const r=node.getBoundingClientRect();return {x:r.x,y:r.y,documentY:r.y+scrollY,width:r.width,height:r.height};};
  const mainRect=rect(main),directCount=main.childElementCount,directHead=[],directTail=[],depthHead=[],depthTail=[];
  for(let i=0;i<Math.min(8,directCount);i++)directHead.push(main.children.item(i));
  for(let i=Math.max(8,directCount-8);i<directCount;i++)directTail.push(main.children.item(i));
  const walker=document.createTreeWalker(main,NodeFilter.SHOW_ELEMENT);let visited=0;
  while(visited<MAX_DESCENDANT_VISITS&&walker.nextNode()){
    const node=walker.currentNode;visited++;
    if(depthHead.length<8)depthHead.push(node);
    depthTail.push(node);if(depthTail.length>23)depthTail.shift();
  }
  const traversalTruncated=visited===MAX_DESCENDANT_VISITS&&Boolean(walker.nextNode()),selected=new Map([[main,"main"]]);
  for(const node of directHead)if(!selected.has(node))selected.set(node,"direct-head");
  for(const node of directTail)if(!selected.has(node))selected.set(node,"direct-tail");
  for(const node of depthHead)if(!selected.has(node))selected.set(node,"depth-head");
  for(const node of depthTail)if(!selected.has(node))selected.set(node,"depth-tail");
  const nodes=[...selected].slice(0,MAX_MAIN_SNAPSHOT_NODES);
  const sampleTruncated=traversalTruncated||directCount>16||visited+1>nodes.length;
  const details=nodes.map(([node,sample])=>{
    const r=node.getBoundingClientRect(),depth=(()=>{let count=0;for(let p=node;p&&p!==main;p=p.parentElement)count++;return Math.min(32,count);})();
    const style=getComputedStyle(node);
    return {id:identify(node),parentId:node===main?null:identify(node.parentElement),depth,tag:node.nodeName.slice(0,16),sample,
      ...rect(node),viewportIntersecting:r.width>0&&r.height>0&&r.right>0&&r.bottom>0&&r.left<innerWidth&&r.top<innerHeight,
      position:["static","relative","absolute","fixed","sticky"].includes(style.position)?style.position:"other",
      visibility:["visible","hidden","collapse"].includes(style.visibility)?style.visibility:"other"};
  });
  return {time:Number.isFinite(sampleTime)?sampleTime:performance.now(),main:mainRect,nodes:details,truncated:sampleTruncated,
    lightDOMElementCount:traversalTruncated?null:visited+1,traversedDescendantCount:visited,directChildCount:directCount,scrollX,scrollY};
}

const boundedSnapshot = snapshot => ({
  time: Number.isFinite(snapshot?.time) ? snapshot.time : null,
  main: snapshot?.main ? rect(snapshot.main) : null,
  scrollX:Number.isFinite(snapshot?.scrollX)?snapshot.scrollX:null,scrollY:Number.isFinite(snapshot?.scrollY)?snapshot.scrollY:null,
  lightDOMElementCount:Number.isInteger(snapshot?.lightDOMElementCount)&&snapshot.lightDOMElementCount>=0?snapshot.lightDOMElementCount:null,
  traversedDescendantCount:Number.isInteger(snapshot?.traversedDescendantCount)?snapshot.traversedDescendantCount:null,
  directChildCount:Number.isInteger(snapshot?.directChildCount)?snapshot.directChildCount:null,
  sampledNodes:Math.min(MAX_NODES,Array.isArray(snapshot?.nodes)?snapshot.nodes.length:0),
  truncated: Boolean(snapshot?.truncated || (snapshot?.nodes?.length ?? 0) > MAX_NODES),
  nodes: (Array.isArray(snapshot?.nodes) ? snapshot.nodes : []).slice(0, MAX_NODES).map(safeNode),
});

export function summarizeMainChange(initialInput, finalInput, frames = [], paints = []) {
  const initial = boundedSnapshot(initialInput), final = boundedSnapshot(finalInput);
  const firstContentfulPaintTime = paints.find(item => item?.name === "first-contentful-paint" && Number.isFinite(item.time))?.time ?? null;
  let lastUnchangedFrameTime=initial.time,firstChangedFrameTime=null;
  for(const frame of (Array.isArray(frames)?frames:[])){
    if(!Number.isFinite(frame?.time)||initial.time!==null&&frame.time<initial.time)continue;
    const main=frame?.boxes?.find(box=>box.node==="main");
    if(main&&changed(initial.main,main,MAIN_DELTA_KEYS).length){firstChangedFrameTime=frame.time;break;}
    lastUnchangedFrameTime=frame.time;
  }
  const changeInterval=firstChangedFrameTime===null?null:{lastUnchangedFrameTime,firstChangedFrameTime};
  let paintRelation="unknown";
  if(changeInterval&&firstContentfulPaintTime!==null){
    if(changeInterval.firstChangedFrameTime<=firstContentfulPaintTime)paintRelation="before-first-contentful-paint";
    else if(changeInterval.lastUnchangedFrameTime>=firstContentfulPaintTime)paintRelation="after-first-contentful-paint";
    else paintRelation="straddles-first-contentful-paint";
  }
  const before = new Map(initial.nodes.filter(node => Number.isInteger(node.id)).map(node => [node.id, node]));
  const after = new Map(final.nodes.filter(node => Number.isInteger(node.id)).map(node => [node.id, node]));
  const ids = [...new Set([...before.keys(), ...after.keys()])];
  const changedDescendants = []; let changedCount=0;
  for (const id of ids) {
    const a = before.get(id), b = after.get(id);
    const geometry = a && b ? changed(a, b, NODE_DELTA_KEYS) : [];
    const presence = !a ? "added" : !b ? "removed" : geometry.length ? "moved" : "present";
    if (presence === "present") continue;
    changedCount++;
    if(changedDescendants.length<MAX_NODES)changedDescendants.push({id, tag: (b ?? a).tag, sample:(b??a).sample, presence, changedFields: geometry,
      visibility: a?.viewportIntersecting || b?.viewportIntersecting ? "in-viewport" : "offscreen"});
  }
  const result = {initial, final, changeInterval, firstContentfulPaintTime,paintRelation,changedDescendants,
    changedDescendantsTruncated: changedCount > MAX_NODES};
  while (JSON.stringify(result).length > MAX_DETAIL_BYTES && (initial.nodes.length || final.nodes.length)) {
    let removed;
    for(const sample of ["depth-head","direct-head","depth-tail","direct-tail"]){
      for(const node of [...initial.nodes].reverse())if(node.sample===sample){removed=node.id;break;}
      if(removed!==undefined)break;
    }
    if(removed!==undefined){initial.nodes=initial.nodes.filter(node=>node.id!==removed);final.nodes=final.nodes.filter(node=>node.id!==removed);}
    else if(initial.nodes.length)initial.nodes.pop();else final.nodes.pop();
    initial.sampledNodes=initial.nodes.length;final.sampledNodes=final.nodes.length;initial.truncated=true;final.truncated=true;
    const keep=new Set([...initial.nodes,...final.nodes].map(node=>node.id));
    result.changedDescendants=result.changedDescendants.filter(node=>keep.has(node.id));
    result.changedDescendantsTruncated=true;
  }
  return result;
}
