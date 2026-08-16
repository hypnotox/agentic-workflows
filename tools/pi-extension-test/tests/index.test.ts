import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { Value } from "typebox/value";
import { createExtensionRecorder, createRecordingEventBus } from "pi-tools/testing";
import { registerSubagentTools, type ExtensionDependencies } from "../../../.pi/extensions/awf-subagents/index.ts";
import * as routing from "../../../.pi/extensions/awf-subagents/model-routing.ts";

const root = "/repo";
const global = "/agent/awf-subagents.json";
const local = "/repo/.pi/awf-subagents.local.json";
const agent = (name: string) => `/repo/.pi/agents/${name}.md`;
const all = (prefix = "p") => Object.fromEntries(routing.PREFERENCE_FIELDS.map((field) => [field, `${prefix}/${field}`]));

function harness(files: Record<string, string> = {}) {
 const eventBus=createRecordingEventBus(); const recorder:any=createExtensionRecorder({eventBus,exec:async (_:string,args:string[])=>args.includes("HEAD")?{code:0,stdout:"a\n",stderr:"",killed:false}:{code:0,stdout:"",stderr:"",killed:false}} as any);
 recorder.modelRegistry.configuredAuth=true;
 const add=(ref:string, _auth=true, present=true) => { const [provider,id]=ref.split("/"); recorder.modelRegistry.add({provider,id,name:id,api:"openai-completions"},present); };
 add("parent/model"); add("p/model");
 const deps:ExtensionDependencies={extensionFile:"/repo/.pi/extensions/awf-subagents/index.ts",agentDir:"/agent",configDirName:".pi",readFile:async(path:string)=> { if(path.endsWith(".md")) return "---\nname: x\n---\nbody"; if(path in files)return files[path]; throw Object.assign(new Error("missing"),{code:"ENOENT"});},writeFile:async()=>{},mkdir:async()=>{},rename:async()=>{},unlink:async()=>{},realpath:async p=>p,lstat:async()=>({isFile:()=>true,isSymbolicLink:()=>false})};
 const ctx:any=recorder.makeContext({cwd:root,model:recorder.modelRegistry.registry.find("parent","model"),sessionManager:{}} as any);
 void recorder.install((pi:any)=>registerSubagentTools(pi,deps)); let batch:any;
 return {pi:recorder.api,deps,ctx,get eventHandlers(){return new Proxy({}, {get:(_,name:string)=>(value:any)=>eventBus.emit(name,value)}) as any;},get hooks(){return Object.fromEntries([...recorder.handlers.entries()].map(([name,handlers]:any)=>[name,handlers[0]]));},request:()=>{const e:any=recorder.emissions.at(-1);return e&&{name:e[0],v:e[1]};},batch:()=>batch,setBatch:(x:any)=>batch=x,get notices(){return recorder.ui.calls.filter((c:any)=>c.name==="notify").map((c:any)=>c.args);},get commands(){return Object.fromEntries(recorder.commands.map((c:any)=>[c.name,c.command]));},add,models:{delete:(ref:string)=>{const [provider,id]=ref.split("/");recorder.modelRegistry.remove(provider,id);}},capability:(x:any)=>eventBus.emit("pi-tools:subagent-profiles:capability",x)} as any;
}
function register(h:ReturnType<typeof harness>) { h.capability({protocolVersion:2,register:(value:any)=>{h.setBatch(value);return {state:"registered"};}}); return h.batch().profiles as any[]; }
function registry(refs:string[]=["p/model"]) { const entries=new Map(refs.map(ref=>{const [provider,id]=ref.split("/");return [ref,{provider,id,auth:true}]})); return {find:(p:string,i:string)=>entries.get(`${p}/${i}`),hasConfiguredAuth:(m:any)=>m.auth!==false,getAvailable:()=>[...entries.values()]}; }

test("required Pi suites install the shared recorder rather than bypassing it", async () => {
 for (const suite of ["index.test.ts", "profile-adapter.test.ts", "using-effort.test.ts"]) {
  const source = await readFile(new URL(`./${suite}`, import.meta.url), "utf8");
  assert.match(source, /from "pi-tools\/testing"/);
  assert.match(source, /createExtensionRecorder/);
  assert.match(source, /\.install\(/);
 }
});

test("protocol-v2 registration is factory-time, atomic, correlated, and terminal notices are once-only", async()=>{
 const h=harness(); assert.equal(h.request().name,"pi-tools:subagent-profiles:request"); assert.equal(h.request().v.protocolVersion,2);
 h.capability({protocolVersion:2,correlationId:"other",register:()=>assert.fail("foreign")}); assert.equal(h.batch(),undefined);
 const profiles=register(h); assert.equal(h.batch().registrationId,"awf:subagent-profiles:v2"); assert.equal(h.batch().suppressDefault,true); assert.equal(profiles.length,6); assert.deepEqual(profiles.map(p=>p.concurrency),[10,10,10,10,10,1]); assert.equal(profiles[5].exclusiveParentBatch,true);
 await h.hooks.session_start({},h.ctx); await new Promise<void>((resolve) => setTimeout(resolve,0)); assert.equal(h.notices.length,0);
 const rejected=harness(); rejected.eventHandlers["pi-tools:subagent-profiles:registration-result"]({protocolVersion:2,registrationId:"awf:subagent-profiles:v2",state:"rejected",reason:"no"}); await rejected.hooks.session_start({},rejected.ctx); await new Promise<void>((resolve) => setTimeout(resolve,0)); assert.equal(rejected.notices.length,1); assert.match(rejected.notices[0][0],/no/);
 const missing=harness(); await missing.hooks.session_start({},missing.ctx); await new Promise<void>((resolve) => setTimeout(resolve,0)); assert.equal(missing.notices.length,1); assert.match(missing.notices[0][0],/missing, late, or incompatible/);
});

test("profiles retain exact schemas, rendered contracts, routing selection, metadata, and commit policy", async()=>{
 const values=all(); const h=harness({[global]:JSON.stringify(values)}); for(const ref of Object.values(values))h.add(ref); const ps=register(h); await h.hooks.session_start({},h.ctx);
 for(const p of ps) assert.equal(Value.Check(p.parameters, p.id==="awf-explore"?{task:"x",breadth:"targeted",detail:"paths"}:p.id==="awf-implement"?{task:"x",allowCommits:false}:{task:"x"}),true);
 const contexts:any[]=[{args:{task:"x"},parent:{model:h.ctx.model}},{args:{task:"x",breadth:"broad",detail:"analysis"},parent:{model:h.ctx.model}},{args:{task:"x"},parent:{model:h.ctx.model}},{args:{task:"x"},parent:{model:h.ctx.model}},{args:{task:"x"},parent:{model:h.ctx.model}},{args:{task:"x",allowCommits:false},parent:{model:h.ctx.model}}];
 for(let i=0;i<ps.length;i++){ const p=ps[i]; const c=contexts[i]; const selected=await p.selectModel(c); const preferenceIndex=i===0?1:i===1?2:i<5?3:4; assert.equal(`${selected.provider}/${selected.id}`, Object.values(values)[preferenceIndex]); const state=await p.beforeRun?.(c); const prepared=await p.prepare(c); assert.match(prepared.systemPrompt,/body/); assert.equal(prepared.cwd,root); if(p.afterRun) { const out=await p.afterRun({state:"completed"},state); assert.ok(out.profileData); } }
 assert.deepEqual(await ps[0].selectModel({args:{task:"x",model:"p/model"},parent:{model:h.ctx.model}}),{provider:"p",id:"model",thinkingLevels:["off","minimal","low","medium","high","xhigh","max"]});
 const before=await ps[5].beforeRun(contexts[5]); const failure=await ps[5].afterRun({state:"completed"},before); assert.equal(failure.failure,undefined);
});

test("routing preferences strictly parse, merge, validate, resolve, preview, and bound cards", async()=>{
 assert.deepEqual(routing.parseExactModelReference("p/m"),{provider:"p",id:"m"}); for(const x of [undefined,"auto","a/","/a","p/a b"])assert.deepEqual(routing.parseExactModelReference(x),{reason:"malformed"}); assert.deepEqual(routing.parseExactModelReference(`p/${"x".repeat(256)}`),{reason:"overlong"});
 const g=routing.emptyPreferenceSource("global","/g"), p=routing.emptyPreferenceSource("project","/p"); g.values={default:"p/model",grounding:"p/model"}; p.values={exploration:"p/model"}; const state=routing.effectivePreferenceState(g,p,[]); assert.equal(state.effective.exploration?.scope,"project"); assert.match(routing.routingPreview(p,g,state).join("\n"),/exploration: p\/model/); assert.deepEqual(routing.resolveChildModel(registry(),{provider:"parent",id:"model"},"explore",undefined,state).source,"project-role");
 const bad=routing.parsePreferenceSource("global","/g",'{"unknown":"secret"}'); assert.deepEqual(bad.invalid,[{kind:"source",scope:"global",reason:"unknown-key"}]); assert.match(routing.invalidText(bad.invalid[0] as any),/unknown-key/); const blocked=routing.effectivePreferenceState(bad,p,[]); assert.throws(()=>routing.resolveChildModel(registry(),undefined,"explore",undefined,blocked),/implicit routing is blocked/); assert.throws(()=>routing.resolveChildModel(registry(),undefined,"explore",undefined,routing.effectivePreferenceState(routing.emptyPreferenceSource("global","/g"),routing.emptyPreferenceSource("project","/p"),[])),/without an active parent/);
 const refs=all(); const cardState:any={...routing.effectivePreferenceState(routing.parsePreferenceSource("global","/g",JSON.stringify(refs)),p,[]),effective:Object.fromEntries(routing.PREFERENCE_FIELDS.map(k=>[k,{reference:`p/${"x".repeat(1000)}`,scope:"global"}]))}; assert.match(routing.buildRoutingCard(cardState),/state: unavailable/);
});

test("preference loading reloads global and local sources and invalid explicit overrides remain separate", async()=>{
 const h=harness({[global]:JSON.stringify({default:"p/model"}),[local]:JSON.stringify({review:"p/model"})}); const state=await routing.loadPreferenceState(h.deps,h.ctx.modelRegistry); assert.equal(state.effective.review?.scope,"project"); assert.equal(routing.resolveChildModel(h.ctx.modelRegistry,h.ctx.model,"review","p/model",state).source,"requested"); h.models.delete("p/model"); assert.deepEqual(routing.registryFailures(h.ctx.modelRegistry,[state.global])[0],{kind:"field",scope:"global",field:"default",reason:"unregistered"});
});
