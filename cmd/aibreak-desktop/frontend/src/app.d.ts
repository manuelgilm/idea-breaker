// Ambient declarations for the Wails-bound Go API.
//
// At `wails dev` / `wails build` time, Wails injects `window.go` at runtime.
// These declarations let `tsc` type-check the frontend without the generated
// `frontend/wailsjs` output (which is gitignored and regenerated on build).
//
// The bound struct is *desktop.App (package `internal/desktop`), so the
// namespace is `desktop` and bound methods do not expose context.Context.

interface Idea {
  id: string;
  title: string;
  body: string;
  tags: string[];
  created: string;
  updated: string;
}

interface IdeaCard {
  id: string;
  title: string;
  body: string;
  tags: string[];
  created: string;
  updated: string;
  score?: number;
}

interface Evaluation {
  run_id: string;
  idea_id: string;
  persona_id: string;
  persona_version: string;
  weight: number;
  status: string;
  score: number;
  rationale: string;
  error?: string;
  created: string;
}

interface FeasibilityScore {
  run_id: string;
  idea_id: string;
  total: number;
  breakdown: Evaluation[];
  requested: number;
  responded: number;
  spread: number;
  summary?: string;
  verdict?: string;
  created: string;
}

interface Feedback {
  id: string;
  idea_id: string;
  author: string;
  score: number;
  rationale: string;
  aspect?: string;
  created: string;
}

interface Resource {
  id: string;
  idea_id: string;
  url: string;
  title: string;
  kind: string;
  note: string;
  created: string;
}

interface PersonaView {
  id: string;
  name: string;
  prompt: string;
  weight: number;
  version: number;
  created: string;
  builtin: boolean;
}

interface ProviderInfo {
  name: string;
  model: string;
  active: boolean;
  hasKey: boolean;
}

interface APIKey {
  id: string;
  provider: string;
  label: string;
  hint: string;
  is_default: boolean;
  created: string;
}

interface Window {
  go: {
    desktop: {
      App: {
        // ideas
        ListIdeas(tag: string): Promise<Idea[]>;
        ListIdeaCards(): Promise<IdeaCard[]>;
        GetIdea(id: string): Promise<Idea>;
        CreateIdea(title: string, body: string, tags: string[]): Promise<Idea>;
        UpdateIdea(id: string, title: string, body: string, tags: string[]): Promise<Idea>;
        DeleteIdea(id: string): Promise<void>;
        // evaluation
        Evaluate(ideaID: string, personaIDs: string[], summarize: boolean): Promise<FeasibilityScore>;
        ListRuns(ideaID: string): Promise<FeasibilityScore[]>;
        // personas
        ListPersonas(): Promise<PersonaView[]>;
        CreatePersona(name: string, prompt: string, weight: number): Promise<PersonaView>;
        UpdatePersona(id: string, name: string, prompt: string, weight: number): Promise<PersonaView>;
        DeletePersona(id: string): Promise<void>;
        // resources
        ListResources(ideaID: string): Promise<Resource[]>;
        AddResource(ideaID: string, url: string, title: string, kind: string, note: string): Promise<Resource>;
        DeleteResource(id: string): Promise<void>;
        // feedback
        ListFeedback(ideaID: string): Promise<Feedback[]>;
        AddFeedback(ideaID: string, author: string, score: number, rationale: string, aspect: string): Promise<Feedback>;
        DeleteFeedback(id: string): Promise<void>;
        // provider
        GetProviders(): Promise<ProviderInfo[]>;
        SetActiveProvider(name: string): Promise<void>;
        SaveModel(provider: string, model: string): Promise<void>;
        ListAPIKeys(provider: string): Promise<APIKey[]>;
        AddAPIKey(provider: string, label: string, key: string): Promise<APIKey>;
        DeleteAPIKey(id: string): Promise<void>;
        SetDefaultAPIKey(id: string): Promise<void>;
      };
    };
  };
}
