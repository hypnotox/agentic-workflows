declare module "pi-tools/subagent-profile" {
  import type { TSchema, Static } from "typebox";
  export interface ConcreteModel { provider: string; id: string; thinkingLevels: readonly string[] }
  export interface ExecutionOutcome { state: "running" | "completed" | "failed" | "cancelled"; report?: string; failure?: string; usage: unknown; activity: unknown[]; omittedActivity: number; retries: number; retryActive: boolean }
  export interface ProfileDefinition<P extends TSchema = TSchema, D extends TSchema = TSchema, S = unknown> { id:string; toolName:string; label:string; description:string; promptSnippet?:string; promptGuidelines?:string[]; parameters:P; profileDataSchema:D; concurrency?:number; exclusiveParentBatch?:boolean; selectModel(context:any): ConcreteModel | Promise<ConcreteModel>; prepare(context:any): any; beforeRun?(context:any): S | Promise<S>; afterRun?(outcome:ExecutionOutcome,state:S|undefined): any }
  export interface ProfileCapability { protocolVersion:number; correlationId?:string; register(batch:{registrationId:string;profiles:ProfileDefinition[];suppressDefault?:boolean}): {state:"pending"|"registered"|"rejected"|"late";reason?:string} }
  export interface ProfileRegistrationResult { protocolVersion:2; registrationId:string; correlationId?:string; state:"registered"|"rejected"; reason?:string }
  export const SUBAGENT_PROFILE_CAPABILITY_EVENT: "pi-tools:subagent-profiles:capability";
  export const SUBAGENT_PROFILE_PROTOCOL_VERSION: 2;
  export const SUBAGENT_PROFILE_REGISTRATION_RESULT_EVENT: "pi-tools:subagent-profiles:registration-result";
  export const SUBAGENT_PROFILE_REQUEST_EVENT: "pi-tools:subagent-profiles:request";
}
