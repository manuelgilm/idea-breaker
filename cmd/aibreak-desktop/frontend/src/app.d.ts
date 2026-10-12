// Ambient type declarations for the UI.
//
// The DTOs below mirror the Go JSON tags shared by the Wails bindings and the
// HTTP API. The Wails `window.go` / `window.runtime` globals are injected by
// Wails at `wails dev` / `wails build` time and are only present in the desktop
// binary; `client.ts` falls back to the HTTP transport when they are absent.

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
  score?: number | null;
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

interface PersonaResult {
  name: string;
  status: string;
  score: number;
  rationale: string;
  error?: string;
}

interface Window {
  go: {
    desktop: {
      App: {
        // ideas
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
      };
    };
  };
  runtime: {
    EventsOn(eventName: string, callback: (...data: any[]) => void): () => void;
    EventsOff(eventName: string, additionalEventNames?: string[]): void;
  };
}
