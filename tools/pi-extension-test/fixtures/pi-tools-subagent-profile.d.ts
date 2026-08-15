declare module "pi-tools/subagent-profile" {
  import type { Static, TSchema } from "typebox";

  export type ThinkingLevel = "off" | "minimal" | "low" | "medium" | "high" | "xhigh" | "max";
  export interface ConcreteModel {
    provider: string;
    id: string;
    thinkingLevels: ThinkingLevel[];
  }
  export type JsonValue = string | number | boolean | null | JsonValue[] | { [key: string]: JsonValue };
  export type ToolPolicy =
    | { mode: "allowlist"; tools: string[] }
    | { mode: "inherit"; deny: string[] };
  export interface PreparedRun {
    cwd: string;
    systemPrompt: string;
    prompt: string;
    toolPolicy: ToolPolicy;
  }
  export interface ExecutionUsage {
    input: number;
    output: number;
    cacheRead: number;
    cacheWrite: number;
    cacheWrite1h?: number;
    reasoning?: number;
    totalTokens: number;
    cost: { input: number; output: number; cacheRead: number; cacheWrite: number; total: number };
  }
  export interface ExecutionActivity {
    kind: "tool_start" | "tool_end" | "retry_start" | "retry_end" | "diagnostic";
    text: string;
  }
  export interface ExecutionOutcome {
    state: "running" | "completed" | "failed" | "cancelled";
    report?: string;
    failure?: string;
    usage: ExecutionUsage;
    activity: ExecutionActivity[];
    omittedActivity: number;
    retries: number;
    retryActive: boolean;
  }
  export interface ProfileContext<TArgs> {
    args: TArgs;
    parent: {
      cwd: string;
      activeTools: string[];
      model: ConcreteModel;
      thinkingLevel: ThinkingLevel;
      trusted: boolean;
    };
    signal: AbortSignal;
  }
  export interface PostRunResult<TProfileData extends JsonValue = JsonValue> {
    report?: string;
    failure?: string;
    profileData?: TProfileData;
  }
  export interface ProfileDefinition<
    TParameters extends TSchema = TSchema,
    TProfileData extends TSchema = TSchema,
    TState = unknown,
  > {
    id: string;
    toolName: string;
    label: string;
    description: string;
    promptSnippet?: string;
    promptGuidelines?: string[];
    parameters: TParameters;
    profileDataSchema: TProfileData;
    concurrency?: number;
    exclusiveParentBatch?: boolean;
    selectModel(context: ProfileContext<Static<TParameters>>): ConcreteModel | Promise<ConcreteModel>;
    selectThinkingLevel?(context: ProfileContext<Static<TParameters>>): ThinkingLevel | undefined;
    prepare(context: ProfileContext<Static<TParameters>>): Promise<PreparedRun> | PreparedRun;
    beforeRun?(context: ProfileContext<Static<TParameters>>): Promise<TState> | TState;
    afterRun?(
      outcome: ExecutionOutcome,
      state: TState | undefined,
    ): Promise<PostRunResult<Static<TProfileData> & JsonValue> | undefined> | PostRunResult<Static<TProfileData> & JsonValue> | undefined;
  }
  export interface ProfileRegistration {
    registrationId: string;
    profiles: ProfileDefinition[];
    suppressDefault?: boolean;
  }
  export type ProfileRegistrationState = "pending" | "registered" | "rejected" | "late";
  export interface ProfileRegistrationReceipt {
    state: ProfileRegistrationState;
    reason?: string;
  }
  export interface ProfileRegistrationResult {
    protocolVersion: 2;
    registrationId: string;
    state: "registered" | "rejected";
    reason?: string;
  }
  export interface ProfileCapabilityRequest {
    protocolVersion: number;
    correlationId: string;
  }
  export interface ProfileCapability {
    protocolVersion: number;
    correlationId?: string;
    register(batch: ProfileRegistration): ProfileRegistrationReceipt;
  }
}
