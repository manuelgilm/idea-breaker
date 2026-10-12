// Transport layer for the UI. One Client interface, two implementations:
//
//   - wailsClient: used by the desktop binary (window.go bindings + events).
//   - httpClient:  used by the web service (fetch against /v1, SSE for progress).
//
// main.ts only ever talks to the Client, so the same rendering code serves both
// the desktop app and the browser.

export interface Client {
  listIdeaCards(): Promise<IdeaCard[]>;
  createIdea(title: string, body: string, tags: string[]): Promise<Idea>;
  getIdea(id: string): Promise<Idea>;
  updateIdea(id: string, title: string, body: string, tags: string[]): Promise<Idea>;
  deleteIdea(id: string): Promise<void>;

  listPersonas(): Promise<PersonaView[]>;
  createPersona(name: string, prompt: string, weight: number): Promise<PersonaView>;
  updatePersona(id: string, name: string, prompt: string, weight: number): Promise<PersonaView>;
  deletePersona(id: string): Promise<void>;

  listRuns(ideaID: string): Promise<FeasibilityScore[]>;
  evaluate(
    ideaID: string,
    personaIDs: string[],
    summarize: boolean,
    onPersona: (result: PersonaResult) => void,
  ): Promise<FeasibilityScore>;

  listResources(ideaID: string): Promise<Resource[]>;
  addResource(ideaID: string, url: string, title: string, kind: string, note: string): Promise<Resource>;
  deleteResource(ideaID: string, id: string): Promise<void>;

  listFeedback(ideaID: string): Promise<Feedback[]>;
  addFeedback(ideaID: string, author: string, score: number, rationale: string, aspect: string): Promise<Feedback>;
  deleteFeedback(ideaID: string, id: string): Promise<void>;

  getProviders(): Promise<ProviderInfo[]>;
  setActiveProvider(name: string): Promise<void>;
  saveModel(provider: string, model: string): Promise<void>;
}

const BUILTIN_PERSONAS = new Set(["skeptic", "optimist", "engineer"]);

// ---------------------------------------------------------------------------
// HTTP client (web service)
// ---------------------------------------------------------------------------

function httpError(res: Response): Promise<Error> {
  return (async () => {
    let message = `${res.status} ${res.statusText}`;
    try {
      const env = await res.json();
      if (env?.error?.message) message = String(env.error.message);
    } catch {
      /* not JSON */
    }
    return new Error(message);
  })();
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch("/v1" + path, {
    method,
    headers: body !== undefined ? { "Content-Type": "application/json" } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) throw await httpError(res);
  if (res.status === 204) return undefined as T;
  const text = await res.text();
  return text ? (JSON.parse(text) as T) : (undefined as T);
}

function toPersonaView(p: any): PersonaView {
  return {
    id: p.id,
    name: p.name,
    prompt: p.system_prompt,
    weight: p.weight,
    version: p.version,
    created: p.created,
    builtin: BUILTIN_PERSONAS.has(p.id),
  };
}

interface SSEMessage {
  event: string;
  data: string;
}

function parseSSE(chunk: string): SSEMessage | null {
  let event = "message";
  const data: string[] = [];
  for (const line of chunk.split("\n")) {
    if (line.startsWith("event:")) event = line.slice(6).trim();
    else if (line.startsWith("data:")) data.push(line.slice(5).replace(/^ /, ""));
  }
  if (data.length === 0) return null;
  return { event, data: data.join("\n") };
}

const httpClient: Client = {
  listIdeaCards: () => request<IdeaCard[]>("GET", "/ideas/cards"),
  createIdea: (title, body, tags) => request<Idea>("POST", "/ideas", { title, body, tags }),
  getIdea: (id) => request<Idea>("GET", `/ideas/${encodeURIComponent(id)}`),
  updateIdea: (id, title, body, tags) =>
    request<Idea>("PATCH", `/ideas/${encodeURIComponent(id)}`, { title, body, tags }),
  deleteIdea: (id) => request<void>("DELETE", `/ideas/${encodeURIComponent(id)}`),

  listPersonas: async () =>
    (await request<any[]>("GET", "/personas")).map(toPersonaView),
  createPersona: async (name, systemPrompt, weight) =>
    toPersonaView(await request<any>("POST", "/personas", { name, system_prompt: systemPrompt, weight })),
  updatePersona: async (id, name, systemPrompt, weight) =>
    toPersonaView(await request<any>("PATCH", `/personas/${encodeURIComponent(id)}`, { name, system_prompt: systemPrompt, weight })),
  deletePersona: (id) => request<void>("DELETE", `/personas/${encodeURIComponent(id)}`),

  listRuns: (ideaID) => request<FeasibilityScore[]>("GET", `/ideas/${encodeURIComponent(ideaID)}/evaluations`),
  evaluate: async (ideaID, personaIDs, summarize, onPersona) => {
    const res = await fetch(`/v1/ideas/${encodeURIComponent(ideaID)}/evaluate/stream`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ personas: personaIDs, summary: summarize }),
    });
    if (!res.ok) throw await httpError(res);
    if (!res.body) throw new Error("streaming is not supported by this browser");

    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    let buffer = "";
    let done: FeasibilityScore | undefined;
    let failure: string | undefined;

    let finished = false;
    while (!finished) {
      const { value, done: streamDone } = await reader.read();
      if (value) buffer += decoder.decode(value, { stream: true });
      let idx: number;
      while ((idx = buffer.indexOf("\n\n")) >= 0) {
        const msg = parseSSE(buffer.slice(0, idx));
        buffer = buffer.slice(idx + 2);
        if (!msg) continue;
        if (msg.event === "persona") {
          onPersona(JSON.parse(msg.data) as PersonaResult);
        } else if (msg.event === "done") {
          done = JSON.parse(msg.data) as FeasibilityScore;
        } else if (msg.event === "error") {
          const env = JSON.parse(msg.data);
          failure = env?.error?.message ?? "evaluation failed";
        }
      }
      finished = streamDone;
    }

    if (failure) throw new Error(failure);
    if (!done) throw new Error("evaluation stream ended without a result");
    return done;
  },

  listResources: (ideaID) => request<Resource[]>("GET", `/ideas/${encodeURIComponent(ideaID)}/resources`),
  addResource: (ideaID, url, title, kind, note) =>
    request<Resource>("POST", `/ideas/${encodeURIComponent(ideaID)}/resources`, { url, title, kind, note }),
  deleteResource: (ideaID, id) => request<void>("DELETE", `/ideas/${encodeURIComponent(ideaID)}/resources/${encodeURIComponent(id)}`),

  listFeedback: (ideaID) => request<Feedback[]>("GET", `/ideas/${encodeURIComponent(ideaID)}/feedback`),
  addFeedback: (ideaID, author, score, rationale, aspect) =>
    request<Feedback>("POST", `/ideas/${encodeURIComponent(ideaID)}/feedback`, { author, score, rationale, aspect }),
  deleteFeedback: (ideaID, id) => request<void>("DELETE", `/ideas/${encodeURIComponent(ideaID)}/feedback/${encodeURIComponent(id)}`),

  getProviders: async () => {
    const infos = await request<any[]>("GET", "/providers");
    return infos.map((p) => ({ name: p.name, model: p.model, active: p.active, hasKey: p.has_key }));
  },
  setActiveProvider: (name) => request<void>("PUT", `/providers/${encodeURIComponent(name)}`, { active: true }),
  saveModel: (provider, model) => request<void>("PUT", `/providers/${encodeURIComponent(provider)}`, { model }),
};

// ---------------------------------------------------------------------------
// Wails client (desktop)
// ---------------------------------------------------------------------------

const wailsClient: Client = {
  listIdeaCards: () => window.go.desktop.App.ListIdeaCards(),
  createIdea: (title, body, tags) => window.go.desktop.App.CreateIdea(title, body, tags),
  getIdea: (id) => window.go.desktop.App.GetIdea(id),
  updateIdea: (id, title, body, tags) => window.go.desktop.App.UpdateIdea(id, title, body, tags),
  deleteIdea: (id) => window.go.desktop.App.DeleteIdea(id),

  listPersonas: () => window.go.desktop.App.ListPersonas(),
  createPersona: (name, prompt, weight) => window.go.desktop.App.CreatePersona(name, prompt, weight),
  updatePersona: (id, name, prompt, weight) => window.go.desktop.App.UpdatePersona(id, name, prompt, weight),
  deletePersona: (id) => window.go.desktop.App.DeletePersona(id),

  listRuns: (ideaID) => window.go.desktop.App.ListRuns(ideaID),
  evaluate: async (ideaID, personaIDs, summarize, onPersona) => {
    const off = window.runtime.EventsOn("evaluation:persona", (result: PersonaResult) => onPersona(result));
    try {
      return await window.go.desktop.App.Evaluate(ideaID, personaIDs, summarize);
    } finally {
      off();
    }
  },

  listResources: (ideaID) => window.go.desktop.App.ListResources(ideaID),
  addResource: (ideaID, url, title, kind, note) =>
    window.go.desktop.App.AddResource(ideaID, url, title, kind, note),
  deleteResource: (_ideaID, id) => window.go.desktop.App.DeleteResource(id),

  listFeedback: (ideaID) => window.go.desktop.App.ListFeedback(ideaID),
  addFeedback: (ideaID, author, score, rationale, aspect) =>
    window.go.desktop.App.AddFeedback(ideaID, author, score, rationale, aspect),
  deleteFeedback: (_ideaID, id) => window.go.desktop.App.DeleteFeedback(id),

  getProviders: () => window.go.desktop.App.GetProviders(),
  setActiveProvider: (name) => window.go.desktop.App.SetActiveProvider(name),
  saveModel: (provider, model) => window.go.desktop.App.SaveModel(provider, model),
};

// createClient picks the transport: Wails bindings when running in the desktop
// binary, HTTP otherwise (the web service).
export function createClient(): Client {
  if (typeof window !== "undefined" && window.go?.desktop?.App) {
    return wailsClient;
  }
  return httpClient;
}
