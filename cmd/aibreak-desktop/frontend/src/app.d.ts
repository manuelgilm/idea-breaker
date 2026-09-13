// Ambient declarations for the Wails-bound Go API.
//
// At `wails dev` / `wails build` time, Wails injects `window.go` at runtime.
// These declarations let `tsc` type-check the frontend without the generated
// `frontend/wailsjs` output (which is gitignored and regenerated on build).

interface Idea {
  id: string;
  title: string;
  body: string;
  tags: string[];
  created: string;
  updated: string;
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

interface Window {
  go: {
    main: {
      App: {
        ListIdeas(tag: string): Promise<Idea[]>;
        GetIdea(id: string): Promise<Idea>;
        Evaluate(ideaID: string, personaIDs: string[], summarize: boolean): Promise<FeasibilityScore>;
        ListRuns(ideaID: string): Promise<FeasibilityScore[]>;
        ListFeedback(ideaID: string): Promise<Feedback[]>;
        AddFeedback(ideaID: string, author: string, score: number, rationale: string, aspect: string): Promise<Feedback>;
      };
    };
  };
}
