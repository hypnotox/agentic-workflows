import assert from "node:assert/strict";
import test from "node:test";
import { Value } from "typebox/value";
import extension, { registerSubagentTools, type ExtensionDependencies } from "../../../.pi/extensions/awf-subagents/index.ts";
import { PREFERENCE_FIELDS, RECOMMENDED_PRESET } from "../../../.pi/extensions/awf-subagents/model-routing.ts";

const ROOT="/repo", GLOBAL="/agent/awf-subagents.json", LOCAL="/repo/.pi/awf-subagents.local.json";
const WORKTREE="/repo/.awf/worktrees/w", COMMON="/repo/.git", ADMIN="/repo/.git/worktrees/w", BACKLINK=`${ADMIN}/gitdir`;
const err=(message:string,code="EIO")=>Object.assign(new Error(message),{code});
type Answer=unknown|((h:any)=>unknown);

function harness(options:{files?:Record<string,string|Error>;answers?:Answer[];activeTools?:any;sessionManager?:any;exec?:(args:string[],cwd:string,index:number)=>any;realpath?:(p:string)=>Promise<string>;lstat?:(p:string)=>Promise<any>}={}) {
 const files=options.files??{}, answers=[...(options.answers??[])], events=new Map<string,any>(), hooks=new Map<string,any>(), commands=new Map<string,any>();
 const notices:any[]=[], prompts:any[]=[], writes:any[]=[], execs:any[]=[], models=new Map<string,any>(), available=new Set<string>(); let request:any, batch:any;
 const add=(ref:string,auth=true,present=true)=>{const slash=ref.indexOf("/"), m={provider:ref.slice(0,slash),id:ref.slice(slash+1),name:ref.slice(slash+1),auth};models.set(ref,m);if(present)available.add(ref);}; add("parent/model"); add("p/model");
 const pop=()=>{const a=answers.shift();return typeof a==="function"?(a as any)(h):a;};
 const pi:any={events:{on:(n:string,f:any)=>events.set(n,f),emit:(n:string,v:any)=>{request={name:n,value:v};}},on:(n:string,f:any)=>hooks.set(n,f),registerCommand:(n:string,c:any)=>commands.set(n,c),getActiveTools:()=>options.activeTools??[],exec:async (_c:string,args:string[],o:any)=>{execs.push({args,cwd:o.cwd});return options.exec?.(args,o.cwd,execs.length-1)??{code:1,stdout:""};}};
 const deps:ExtensionDependencies={extensionFile:"/repo/.pi/extensions/awf-subagents/index.ts",agentDir:"/agent",configDirName:".pi",readFile:async(p:string)=>{if(p in files){const v=files[p];if(v instanceof Error)throw v;return v;}if(p.startsWith("/repo/.pi/agents/"))return `---\nname: role\n---\nContract body for ${p.slice(p.lastIndexOf("/")+1)}.`;if(p===BACKLINK)return `${WORKTREE}/.git\n`;throw err("missing","ENOENT");},writeFile:async(p,d,o)=>{writes.push({op:"write",p,d,mode:o.mode});files[p]=d;},mkdir:async p=>{writes.push({op:"mkdir",p});},rename:async(a,b)=>{writes.push({op:"rename",p:a,to:b});files[b]=files[a];delete files[a];},unlink:async p=>{writes.push({op:"unlink",p});delete files[p];},realpath:options.realpath??(async p=>p),lstat:options.lstat??(async()=>({isFile:()=>true,isSymbolicLink:()=>false}))};
 const sessionManager=options.sessionManager??{};
 const ctx:any={mode:"tui",model:{provider:"parent",id:"model"},sessionManager,modelRegistry:{find:(p:string,i:string)=>models.get(`${p}/${i}`),hasConfiguredAuth:(m:any)=>m.auth!==false,getAvailable:()=>[...available].map(x=>models.get(x)),getAll:()=>[...models.values()]},ui:{notify:(...x:any[])=>notices.push(x),select:async(t:string,c:string[])=>{prompts.push({kind:"select",t,c});return pop();},confirm:async(t:string,m:string)=>{prompts.push({kind:"confirm",t,m});return pop();}}};
 const h:any={files,events,hooks,commands,notices,prompts,writes,execs,models,available,pi,deps,ctx,request:()=>request,batch:()=>batch,add,profiles:()=>batch.profiles,start:async()=>{await hooks.get("session_start")({},ctx);},register:(receipt:any={state:"registered"})=>events.get("pi-tools:subagent-profiles:capability")({protocolVersion:2,correlationId:request.value.correlationId,register:(b:any)=>{batch=b;return receipt;}}),wizard:()=>commands.get("awf-subagent-models").handler("",ctx)};
 registerSubagentTools(pi,deps); return h;
}
const complete=(prefix="p/model")=>Object.fromEntries(PREFERENCE_FIELDS.map(k=>[k,prefix]));
const context=(args:any)=>({args,parent:{model:{provider:"parent",id:"model"}}});
const flush=()=>new Promise<void>(r=>queueMicrotask(r));
const nextTask=()=>new Promise<void>(resolve=>setTimeout(resolve,0));
function checkoutGit(before="a",after="b",common=COMMON){let heads=0;return(args:string[],cwd:string)=>{if(args.includes("--show-toplevel"))return{code:0,stdout:`${cwd}\n`};if(args.includes("--git-common-dir"))return{code:0,stdout:`${cwd===ROOT?COMMON:common}\n`};if(args.includes("--absolute-git-dir"))return{code:0,stdout:`${cwd===ROOT?COMMON:ADMIN}\n`};if(args.includes("HEAD"))return{code:0,stdout:`${heads++?after:before}\n`};if(args[0]==="status")return{code:0,stdout:" M x\n"};return{code:1,stdout:""};};}

test("negotiation correlations, receipts, results, deferred and once-only reporting",async()=>{
 const h=harness();assert.equal(h.request().name,"pi-tools:subagent-profiles:request");const cid=h.request().value.correlationId;
 h.events.get("pi-tools:subagent-profiles:capability")({protocolVersion:2,correlationId:"foreign",register:()=>assert.fail()});
 h.events.get("pi-tools:subagent-profiles:capability")({protocolVersion:1,register:()=>assert.fail()});
 h.events.get("pi-tools:subagent-profiles:capability")({protocolVersion:2,register:null});
 await h.start();await nextTask();assert.equal(h.notices.length,1);assert.match(h.notices[0][0],/incompatible capability/);
 h.events.get("pi-tools:subagent-profiles:capability")({protocolVersion:2,correlationId:cid,register:()=>({state:"late"})});await flush();assert.equal(h.notices.length,1);
 for(const receipt of [{state:"rejected",reason:"receipt no"},{state:"late"},{state:"registered"}]){const x=harness();x.register(receipt);await x.start();await nextTask();assert.equal(x.notices.length,receipt.state==="registered"?0:1);}
 for(const result of [
  {registrationId:"other",protocolVersion:2,state:"rejected"},
  {registrationId:"awf:subagent-profiles:v2",protocolVersion:1,state:"registered"},{registrationId:"awf:subagent-profiles:v2",protocolVersion:2,state:"rejected",reason:"result no"},{registrationId:"awf:subagent-profiles:v2",protocolVersion:2,state:"registered"}
 ]){const x=harness();x.events.get("pi-tools:subagent-profiles:registration-result")(result);await x.start();await nextTask();if(result.registrationId==="awf:subagent-profiles:v2"&&result.protocolVersion===2&&result.state==="registered")assert.equal(x.notices.length,0);}
 const pending=harness();pending.register({state:"pending"});await pending.start();pending.events.get("pi-tools:subagent-profiles:registration-result")({protocolVersion:2,registrationId:"awf:subagent-profiles:v2",state:"registered"});await nextTask();assert.equal(pending.notices.length,0);
 const missing=harness();await missing.start();await nextTask();assert.match(missing.notices[0][0],/missing, late/);
});

test("live session, preference notices, routing hook precedence, fallback, overflow, and dedupe",async()=>{
 const pre=harness();pre.register();await assert.rejects(pre.profiles()[0].selectModel(context({task:"x"})),/session_start/);
 const missing=harness();missing.register();await missing.start();await missing.profiles()[0].selectModel(context({task:"x"}));await missing.profiles()[1].selectModel(context({task:"x",breadth:"b",detail:"d"}));assert.equal(missing.notices.length,1);
 missing.ctx.sessionManager=undefined;await missing.start();await missing.profiles()[0].selectModel(context({task:"x"}));missing.ctx.sessionManager="x";await missing.start();await missing.profiles()[0].selectModel(context({task:"x"}));assert.equal(missing.notices.length,1);
 const bad=harness({files:{[GLOBAL]:"{"}});bad.register();await bad.start();await bad.profiles()[0].selectModel(context({task:"x",model:"p/model"}));assert.match(bad.notices[0][0],/implicit routing is blocked/);
 const good=harness({files:{[GLOBAL]:JSON.stringify(complete())},activeTools:["subagent_review_code"]});good.register();await good.start();assert.equal(await good.hooks.get("before_agent_start")({systemPrompt:"s",systemPromptOptions:{selectedTools:[]}}),undefined);const card=await good.hooks.get("before_agent_start")({systemPrompt:"s",systemPromptOptions:{}});assert.match(card.systemPrompt,/awf subagent routing/);assert.match((await good.hooks.get("before_agent_start")({systemPrompt:"s",systemPromptOptions:{selectedTools:"bad"}})).systemPrompt,/routing/);
 const skipped=harness({activeTools:"bad"});skipped.register();await skipped.start();assert.equal(await skipped.hooks.get("before_agent_start")({systemPrompt:"s"}),undefined);
 const maximum=harness({files:{[GLOBAL]:JSON.stringify(complete(`p/${"x".repeat(253)}`))},activeTools:["subagent_review_code"]});maximum.add(`p/${"x".repeat(253)}`);maximum.register();await maximum.start();const maxCard=await maximum.hooks.get("before_agent_start")({systemPrompt:"s"});assert.match(maxCard.systemPrompt,/awf subagent routing/);
});

test("all profiles select, prepare contracts, preserve state, and enforce commit policy",async()=>{
 const h=harness({files:{[GLOBAL]:JSON.stringify(complete())},exec:checkoutGit("a","b")});h.register();await h.start();const ps=h.profiles();assert.deepEqual(ps.map((p:any)=>[p.id,p.concurrency]),[["awf-grounding",10],["awf-explore",10],["awf-review-adr",10],["awf-review-plan",10],["awf-review-code",10],["awf-implement",1]]);
 const args=[{task:"g"},{task:"e",breadth:"broad",detail:"analysis"},{task:"adr"},{task:"plan"},{task:"code"},{task:"i",allowCommits:true}];
 for(let i=0;i<6;i++){const c=context(args[i]), selected=await ps[i].selectModel(c);assert.equal(`${selected.provider}/${selected.id}`,"p/model");const state=await ps[i].beforeRun?.(c);const prepared=await ps[i].prepare(c);assert.equal(prepared.cwd,ROOT);assert.match(prepared.systemPrompt,/Contract body/);assert.doesNotMatch(prepared.systemPrompt,/name: role/);if(i>=2&&i<=4)assert.match(prepared.systemPrompt,new RegExp(`${["adr","plan","code"][i-2]}-reviewer\\.md`));if(ps[i].afterRun){const out=await ps[i].afterRun({state:"completed"},state);assert.ok(out.profileData);assert.equal(Value.Check(ps[i].profileDataSchema,out.profileData),true);assert.deepEqual({requestedModel:out.profileData.requestedModel,resolvedProvider:out.profileData.resolvedProvider,resolvedModel:out.profileData.resolvedModel,resolutionSource:out.profileData.resolutionSource},{requestedModel:null,resolvedProvider:"p",resolvedModel:"model",resolutionSource:"global-role"});}}
 const explicitContext=context({task:"x",model:"p/model"});const explicit=await ps[0].selectModel(explicitContext);assert.equal(explicit.thinkingLevels.length,7);const explicitState=await ps[0].beforeRun(explicitContext);assert.deepEqual(explicitState,{requestedModel:"p/model",resolvedProvider:"p",resolvedModel:"model",resolutionSource:"requested"});assert.equal((await ps[0].selectModel({args:{task:"x"}})).provider,"p");
 const forbidden=harness({exec:checkoutGit("a","b")});forbidden.register();await forbidden.start();const ip=forbidden.profiles()[5],fc=context({task:"x",allowCommits:false});await ip.selectModel(fc);const state=await ip.beforeRun(fc),out=await ip.afterRun({},state);assert.match(out.failure,/committed despite/);assert.equal(out.profileData.commitVerification,"verified");assert.equal(Value.Check(ip.profileDataSchema,out.profileData),true);
 const unchanged=harness({exec:checkoutGit("a","a")});unchanged.register();await unchanged.start();const up=unchanged.profiles()[5],uc=context({task:"x",allowCommits:true});await up.selectModel(uc);const us=await up.beforeRun(uc),uo=await up.afterRun({},us);assert.match(uo.failure,/created no commit/);assert.equal(Value.Check(up.profileDataSchema,uo.profileData),true);
 for(const allowCommits of [false,true])for(const unavailableAt of ["before","after"]){let heads=0;const unavailable=harness({exec:(args:string[])=>args.includes("HEAD")&&((unavailableAt==="before"&&heads++===0)||(unavailableAt==="after"&&heads++===1))?{code:1,stdout:""}:args.includes("HEAD")?{code:0,stdout:"a\n"}:args[0]==="status"?{code:0,stdout:""}:{code:1,stdout:""}});unavailable.register();await unavailable.start();const vp=unavailable.profiles()[5],vc=context({task:"x",allowCommits});await vp.selectModel(vc);const vs=await vp.beforeRun(vc),vo=await vp.afterRun({},vs);assert.equal(vo.failure,undefined,`${allowCommits}/${unavailableAt}`);assert.equal(vo.profileData.commitVerification,"unavailable");assert.equal(Value.Check(vp.profileDataSchema,vo.profileData),true);}
});

test("routing callback state is context-isolated and does not reload after queueing",async()=>{const h=harness({files:{[GLOBAL]:JSON.stringify(complete("p/model"))}});h.add("p/other");h.register();await h.start();const profile=h.profiles()[1],first=context({task:"one",breadth:"targeted",detail:"paths"}),second=context({task:"two",breadth:"broad",detail:"analysis",model:"p/other"});await Promise.all([profile.selectModel(first),profile.selectModel(second)]);h.files[GLOBAL]=JSON.stringify(complete("p/other"));const [one,two]=await Promise.all([profile.beforeRun(first),profile.beforeRun(second)]);assert.deepEqual([one.resolvedModel,one.resolutionSource,one.breadth,one.detail],["model","global-role","targeted","paths"]);assert.deepEqual([two.requestedModel,two.resolvedModel,two.resolutionSource,two.breadth,two.detail],["p/other","other","requested","broad","analysis"]);await assert.rejects(async()=>profile.beforeRun(context({task:"unselected",breadth:"bounded",detail:"summary"})),/without completing selectModel/);});

test("contracts fail closed for missing and empty role artifacts",async()=>{for(const content of [err("no","ENOENT"),"---\nname: x\n---\n \n"]){const h=harness();h.deps.readFile=async()=>{if(content instanceof Error)throw content;return content;};h.register();await h.start();for(const [i,args] of [[0,{task:"x"}],[1,{task:"x",breadth:"targeted",detail:"paths"}],[2,{task:"x"}],[3,{task:"x"}],[4,{task:"x"}],[5,{task:"x",allowCommits:false}]] as any)await assert.rejects(h.profiles()[i].prepare(context(args)),/Missing Pi|no instruction body/);}});

const PRESET="Apply recommended GPT-5.6 preset", MANUAL="Configure each slot", UNSET="leave unset (use fallback chain)";
function registerPreset(h:any){for(const ref of Object.values(RECOMMENDED_PRESET))h.add(ref as string);h.register();return h.start();}
async function runWizard(answers:Answer[],options:any={}){const h=harness({...options,answers});await registerPreset(h);await h.wizard();return h;}

test("wizard guards TUI and covers cancellation and manual selection boundaries",async()=>{
 const non=harness();non.register();await non.start();non.ctx.mode="rpc";await non.wizard();assert.match(non.notices[0][0],/interactive TUI/);non.ctx.mode="tui";non.ctx.ui.select=undefined;await non.wizard();
 for(const a of [[undefined],[`User-global (${GLOBAL})`,false],[`User-global (${GLOBAL})`,true,undefined],[`User-global (${GLOBAL})`,true,MANUAL,undefined],[`User-global (${GLOBAL})`,true,MANUAL,...Array(8).fill(UNSET),false]]){const h=await runWizard(a);assert.equal(h.writes.length,0);}
 const manual=await runWizard([`User-global (${GLOBAL})`,true,MANUAL,"p/model",...Array(7).fill(UNSET),true]);assert.deepEqual(JSON.parse(manual.writes.find((x:any)=>x.op==="write").d),{default:"p/model"});assert.equal(manual.prompts.filter((x:any)=>x.kind==="select").length,10);
 const unavailable=harness({answers:[`User-global (${GLOBAL})`,true,...Array(8).fill(UNSET),true]});unavailable.register();await unavailable.start();await unavailable.wizard();assert.match(unavailable.notices[0][0],/Recommended preset unavailable/);
 const unauth=harness({answers:[`User-global (${GLOBAL})`,true,...Array(8).fill(UNSET),true]});for(const ref of Object.values(RECOMMENDED_PRESET))unauth.add(ref as string);unauth.models.get(RECOMMENDED_PRESET.large).auth=false;unauth.register();await unauth.start();await unauth.wizard();assert.match(unauth.notices[0][0],/unauthenticated/);
});

test("wizard preset, invalid current state, project ignore, atomic save, and saved-invalid notices",async()=>{
 const global=await runWizard([`User-global (${GLOBAL})`,true,PRESET,true]);assert.deepEqual(JSON.parse(global.writes.find((x:any)=>x.op==="write").d),RECOMMENDED_PRESET);assert.equal(global.writes.find((x:any)=>x.op==="write").mode,0o600);assert.match(global.notices.at(-1)[0],/saved/);
 const invalid=await runWizard([`User-global (${GLOBAL})`,true,PRESET,true],{files:{[GLOBAL]:"{"}});assert.match(invalid.prompts[1].m,/malformed-json/);
 const projectLabel=`Project-local (${LOCAL})`;
 const decline=await runWizard([projectLabel,true,PRESET,true,false],{exec:(a:string[])=>a.includes("--is-inside-work-tree")?{code:0}:{code:1}});assert.match(decline.notices.at(-1)[0],/must be gitignored/);
 const append=await runWizard([projectLabel,true,PRESET,true,true],{files:{[`${ROOT}/.gitignore`]:"existing"},exec:(a:string[])=>a.includes("--is-inside-work-tree")?{code:0}:{code:1}});assert.equal(append.files[`${ROOT}/.gitignore`],"existing\n.pi/awf-subagents.local.json\n");
 const emptyAppend=await runWizard([projectLabel,true,PRESET,true,true],{exec:(a:string[])=>a.includes("--is-inside-work-tree")?{code:0}:{code:1}});assert.match(emptyAppend.files[`${ROOT}/.gitignore`],/^\.pi/);
 const ignored=await runWizard([projectLabel,true,PRESET,true],{exec:()=>({code:0})});assert.equal(ignored.writes.filter((x:any)=>x.p.endsWith(".gitignore")).length,0);
 const outside=await runWizard([projectLabel,true,PRESET,true],{exec:()=>({code:1})});assert.match(outside.notices[0][0],/Not a git work tree/);
 const savedBad=await runWizard([`User-global (${GLOBAL})`,true,PRESET,true],{});savedBad.files[GLOBAL]="{";await savedBad.wizard;
});

test("wizard reports read, ignore, mkdir, stale, reread, write, rename, unlink and post-save invalid failures",async()=>{
 const label=`User-global (${GLOBAL})`, project=`Project-local (${LOCAL})`;
 const initial=harness({files:{[GLOBAL]:err("read")},answers:[label]});initial.register();await initial.start();await initial.wizard();assert.match(initial.notices.at(-1)[0],/Cannot read/);
 const nullRead=harness({answers:[label]});nullRead.deps.readFile=async()=>{throw null;};nullRead.register();await nullRead.start();await nullRead.wizard();assert.match(nullRead.notices.at(-1)[0],/Cannot read/);
 for(const [kind,setup,expected] of [
  ["ignore-read",(h:any)=>h.files[`${ROOT}/.gitignore`]=err("bad"),/Cannot read/],
  ["ignore-write",(h:any)=>{const old=h.deps.writeFile;h.deps.writeFile=async(p:string,...x:any[])=>{if(p.endsWith(".gitignore"))throw "primitive";return old(p,...x);};},/Cannot update/]
 ] as any){const h=harness({answers:[project,true,PRESET,true,true],exec:(a:string[])=>a.includes("--is-inside-work-tree")?{code:0}:{code:1}});await registerPreset(h);setup(h);await h.wizard();assert.match(h.notices.at(-1)[0],expected,kind);}
 for(const [op,expected] of [["mkdir",/Cannot create/],["writeFile",/Save failed/],["rename",/Save failed/]] as any){const h=harness({answers:[label,true,PRESET,true]});await registerPreset(h);h.deps[op]=async()=>{throw err(op)};await h.wizard();assert.match(h.notices.at(-1)[0],expected);}
 const cleanup=harness({answers:[label,true,PRESET,true]});await registerPreset(cleanup);cleanup.deps.rename=async()=>{throw err("rename")};cleanup.deps.unlink=async()=>{throw err("unlink")};await cleanup.wizard();assert.match(cleanup.notices.at(-1)[0],/rename/);
 const stale=harness({files:{[GLOBAL]:"{}"},answers:[label,true,PRESET,()=>{stale.files[GLOBAL]="{\"default\":\"p/model\"}";return true;}]});await registerPreset(stale);await stale.wizard();assert.match(stale.notices.at(-1)[0],/concurrently/);
 const reread=harness({answers:[label,true,PRESET,()=>{reread.deps.readFile=async()=>{throw err("again")};return true;}]});await registerPreset(reread);await reread.wizard();assert.match(reread.notices.at(-1)[0],/Cannot re-read/);
 const bad=harness({answers:[label,true,PRESET,true]});await registerPreset(bad);const original=bad.deps.rename;bad.deps.rename=async(a:string,b:string)=>{await original(a,b);bad.files[GLOBAL]="{";};await bad.wizard();assert.match(bad.notices.at(-1)[0],/saved, but they are invalid/);
});

test("verification checkout accepts roots and linked checkouts and strips terminal endings",async()=>{
 for(const [value,git,realpath,files] of [[undefined,checkoutGit(),undefined,undefined],[ROOT,checkoutGit(),undefined,undefined],["@.awf/worktrees/w",checkoutGit(),async(p:string)=>p===WORKTREE?WORKTREE:p,undefined],[`${WORKTREE} `,(a:string[],cwd:string)=>{const r=checkoutGit()(a,cwd);if(a.includes("--show-toplevel"))return{code:0,stdout:cwd};return r;},undefined,{[BACKLINK]:`${WORKTREE} /.git\r\n`}]] as any){const h=harness({exec:git,realpath,files});h.register();await h.start();const p=h.profiles()[5],c=context({task:"x",allowCommits:true,verificationCheckout:value});await p.selectModel(c);const s=await p.beforeRun(c);assert.ok(s.verificationCheckout);}
});

test("verification checkout rejects every insecure or stale identity branch",async()=>{
 const cases:any[]=[
  ["",{},/empty/],["@",{},/empty/],["/missing",{realpath:async()=>{throw err("no","ENOENT")}},/does not exist/],
  [WORKTREE,{lstat:async()=>{throw err("denied")},exec:checkoutGit()},/Cannot inspect/],[WORKTREE,{lstat:async()=>({isFile:()=>true,isSymbolicLink:()=>true}),exec:checkoutGit()},/symlink/],
  [WORKTREE,{exec:()=>({code:1})},/live registered/],[`${WORKTREE}/sub`,{exec:(a:string[],cwd:string)=>a.includes("--show-toplevel")?{code:0,stdout:`${WORKTREE}\n`}:{code:1}},/checkout root/],
  [WORKTREE,{exec:(a:string[],cwd:string)=>a.includes("--show-toplevel")?{code:0,stdout:`${cwd}\n`}:a.includes("--git-common-dir")?{code:1}:{code:0}},/live registered/],
  [WORKTREE,{exec:checkoutGit("a","b","/foreign/.git")},/same repository/],
  [WORKTREE,{exec:(a:string[],cwd:string)=>a.includes("--show-toplevel")?{code:0,stdout:`${cwd}\n`}:a.includes("--git-common-dir")?{code:0,stdout:`${COMMON}\n`}:{code:1}},/absolute Git directory/],
  ["/repo/copy",{exec:(a:string[],cwd:string)=>a.includes("--show-toplevel")?{code:0,stdout:`${cwd}\n`}:a.includes("--git-common-dir")||a.includes("--absolute-git-dir")?{code:0,stdout:"/external/git\n"}:{code:1}},/copied primary/],
  [WORKTREE,{files:{[BACKLINK]:err("gone","ENOENT")},exec:checkoutGit()},/live linked/],[WORKTREE,{files:{[BACKLINK]:err("denied")},exec:checkoutGit()},/Cannot read/],
  [WORKTREE,{files:{[BACKLINK]:"\n"},exec:checkoutGit()},/backlink is empty/],[WORKTREE,{lstat:async()=>({isFile:()=>false,isSymbolicLink:()=>false}),exec:checkoutGit()},/regular file/],
  [WORKTREE,{files:{[BACKLINK]:"/wrong/.git\n"},exec:checkoutGit()},/does not identify/]
 ];
 for(const [value,o,re] of cases){const h=harness(o);h.register();await h.start();await assert.rejects(h.profiles()[5].beforeRun(context({task:"x",allowCommits:true,verificationCheckout:value})),re,String(value));}
 const rootCanonical=harness({exec:checkoutGit(),realpath:async(p:string)=>{if(p===ROOT)throw err("root denied");return p;}});rootCanonical.register();await rootCanonical.start();await assert.rejects(rootCanonical.profiles()[5].beforeRun(context({task:"x",allowCommits:true,verificationCheckout:WORKTREE})),/project root/);
 const topCanonical=harness({exec:checkoutGit(),realpath:async(p:string)=>{if(p===WORKTREE)throw err("top missing");return p;}});topCanonical.register();await topCanonical.start();await assert.rejects(topCanonical.profiles()[5].beforeRun(context({task:"x",allowCommits:true,verificationCheckout:WORKTREE})),/does not exist/);
});

test("default factory composes production dependencies",async()=>{const events=new Map<string,any>(),hooks=new Map<string,any>();const pi:any={events:{on:(n:string,f:any)=>events.set(n,f),emit:()=>{}},on:(n:string,f:any)=>hooks.set(n,f),registerCommand:()=>{},getActiveTools:()=>[],exec:async()=>({code:1,stdout:""})};await extension(pi);assert.ok(hooks.has("session_start"));assert.ok(events.has("pi-tools:subagent-profiles:capability"));});
