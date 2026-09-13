const App = window.go.desktop.App;

type View = "ideas" | "detail" | "personas" | "provider";

const state: {
  view: View;
  ideaId: string;
  personas: PersonaView[];
  editingPersona: PersonaView | null;
  expandedRun: string;
} = {
  view: "ideas",
  ideaId: "",
  personas: [],
  editingPersona: null,
  expandedRun: "",
};

const RESOURCE_KINDS = ["article", "repo", "paper", "video", "other"];

function $(id: string): HTMLElement {
  const el = document.getElementById(id);
  if (!el) throw new Error(`missing element #${id}`);
  return el;
}

function inputValue(id: string): string {
  return (document.getElementById(id) as HTMLInputElement).value;
}

function el<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function clear(node: HTMLElement): void {
  node.innerHTML = "";
}

function scoreColor(score: number | undefined | null): string {
  if (score === undefined || score === null) return "gray";
  if (score <= 50) return "red";
  if (score < 70) return "yellow";
  return "green";
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  return isNaN(d.getTime()) ? "" : d.toLocaleString();
}

function snippet(body: string, max = 180): string {
  const t = (body || "").trim();
  if (!t) return "";
  return t.length > max ? t.slice(0, max).trimEnd() + "…" : t;
}

function parseTags(raw: string): string[] {
  return raw.split(",").map((s) => s.trim()).filter((s) => s !== "");
}

function tagsText(tags: string[]): string {
  return (tags || []).join(", ");
}

function showError(message: string): void {
  const el = document.getElementById("error-banner");
  if (el) {
    el.textContent = message;
    el.classList.remove("hidden");
  }
}

function clearError(): void {
  const el = document.getElementById("error-banner");
  if (el) el.classList.add("hidden");
}

let successTimer: number | undefined;
function showSuccess(message: string): void {
  const el = document.getElementById("success-toast");
  if (!el) return;
  el.textContent = message;
  el.classList.remove("hidden");
  if (successTimer !== undefined) window.clearTimeout(successTimer);
  successTimer = window.setTimeout(() => el.classList.add("hidden"), 3000);
}

// ---------------------------------------------------------------------------
// Shell
// ---------------------------------------------------------------------------

function renderWelcome(): void {
  const app = $("app");
  app.innerHTML = `
    <div class="welcome">
      <h1>aibreak</h1>
      <p class="tagline">Evaluate your ideas with AI personas.</p>
      <button id="start-btn" class="primary start-btn">Start</button>
    </div>
  `;
  $("start-btn").addEventListener("click", () => {
    state.view = "ideas";
    void renderApp();
  });
}

function renderApp(): void {
  const app = $("app");
  app.innerHTML = `
    <div class="layout">
      <aside class="sidebar">
        <div class="brand">aibreak</div>
        <div class="group-label">Registry</div>
        <button class="nav-item" data-view="ideas">Ideas</button>
        <button class="nav-item" data-view="personas">Personas</button>
        <div class="group-label">LLM Provider</div>
        <button class="nav-item" data-view="provider">OpenAI</button>
      </aside>
      <main class="main" id="main"></main>
    </div>
    <div id="error-banner" class="error-banner hidden" style="position: fixed; bottom: 16px; right: 16px; max-width: 420px;"></div>
    <div id="success-toast" class="success-banner hidden" style="position: fixed; bottom: 16px; right: 16px; max-width: 420px;"></div>
  `;

  document.querySelectorAll<HTMLButtonElement>(".nav-item").forEach((btn) => {
    btn.addEventListener("click", () => {
      const view = btn.dataset.view as View;
      if (view === "ideas") state.ideaId = "";
      state.view = view;
      void renderView();
    });
  });

  void renderView();
}

function markActive(): void {
  document.querySelectorAll<HTMLButtonElement>(".nav-item").forEach((btn) => {
    btn.classList.toggle("active", btn.dataset.view === state.view);
  });
}

async function renderView(): Promise<void> {
  clearError();
  markActive();
  const main = $("main");
  clear(main);
  switch (state.view) {
    case "ideas":
      await renderIdeas(main);
      break;
    case "detail":
      await renderDetail(main);
      break;
    case "personas":
      await renderPersonas(main);
      break;
    case "provider":
      await renderProvider(main);
      break;
  }
}

// ---------------------------------------------------------------------------
// Ideas
// ---------------------------------------------------------------------------

async function renderIdeas(main: HTMLElement): Promise<void> {
  main.innerHTML = `
    <div class="toolbar">
      <h2>Ideas</h2>
      <input id="idea-search" type="text" placeholder="Search ideas…" style="flex:1; max-width: 320px;" />
      <button id="create-toggle" class="primary">Create Idea</button>
    </div>
    <div id="create-panel" class="panel hidden">
      <h3>Register idea</h3>
      <div class="field"><label>Title</label><input id="new-title" type="text" /></div>
      <div class="field"><label>Body</label><textarea id="new-body"></textarea></div>
      <div class="field"><label>Tags (comma-separated)</label><input id="new-tags" type="text" /></div>
      <div class="row">
        <button id="create-submit" class="primary">Create</button>
        <button id="create-cancel" class="small">Cancel</button>
      </div>
    </div>
    <div id="cards" class="card-grid"></div>
  `;

  $("create-toggle").addEventListener("click", () => {
    $("create-panel").classList.toggle("hidden");
  });
  $("create-cancel").addEventListener("click", () => {
    $("create-panel").classList.add("hidden");
  });
  $("create-submit").addEventListener("click", () => {
    void createIdea();
  });
  $("idea-search").addEventListener("input", () => {
    void loadCards();
  });

  await loadCards();
}

async function loadCards(): Promise<void> {
  const grid = $("cards");
  clear(grid);
  try {
    const cards = await App.ListIdeaCards();
    const query = inputValue("idea-search").trim().toLowerCase();
    const filtered = query === "" ? cards : cards.filter((c) => matchesCard(c, query));
    if (filtered.length === 0) {
      grid.appendChild(el("p", "hint", query === "" ? "No ideas yet. Create one to get started." : "No ideas match your search."));
      return;
    }
    for (const card of filtered) {
      grid.appendChild(ideaCard(card));
    }
  } catch (e) {
    showError(String(e));
  }
}

function matchesCard(card: IdeaCard, query: string): boolean {
  if (card.title.toLowerCase().includes(query)) return true;
  if (card.body.toLowerCase().includes(query)) return true;
  return (card.tags || []).some((t) => t.toLowerCase().includes(query));
}

function ideaCard(card: IdeaCard): HTMLElement {
  const root = el("div", "card");
  root.addEventListener("click", () => {
    state.ideaId = card.id;
    state.view = "detail";
    void renderView();
  });

  const head = el("div", "card-head");
  head.appendChild(el("span", "card-title", card.title));
  head.appendChild(el("span", "dot " + scoreColor(card.score)));
  root.appendChild(head);

  if (card.body) root.appendChild(el("div", "card-body", snippet(card.body)));

  if (card.tags && card.tags.length > 0) {
    const tags = el("div", "card-tags");
    for (const t of card.tags) tags.appendChild(el("span", "tag", t));
    root.appendChild(tags);
  }

  const actions = el("div", "card-actions");
  const editBtn = el("button", "link", "Edit");
  editBtn.addEventListener("click", (e) => {
    e.stopPropagation();
    state.ideaId = card.id;
    state.view = "detail";
    void renderView();
  });
  const delBtn = el("button", "link", "Delete");
  delBtn.addEventListener("click", (e) => {
    e.stopPropagation();
    void deleteIdea(card.id, card.title);
  });
  actions.appendChild(editBtn);
  actions.appendChild(delBtn);
  root.appendChild(actions);

  return root;
}

async function createIdea(): Promise<void> {
  const title = inputValue("new-title").trim();
  const body = inputValue("new-body").trim();
  const tags = parseTags(inputValue("new-tags"));
  if (!title) {
    showError("Title is required.");
    return;
  }
  try {
    await App.CreateIdea(title, body, tags);
    $("create-panel").classList.add("hidden");
    (document.getElementById("new-title") as HTMLInputElement).value = "";
    (document.getElementById("new-body") as HTMLInputElement).value = "";
    (document.getElementById("new-tags") as HTMLInputElement).value = "";
    await loadCards();
  } catch (e) {
    showError(String(e));
  }
}

async function deleteIdea(id: string, title: string): Promise<void> {
  try {
    await App.DeleteIdea(id);
    await loadCards();
  } catch (e) {
    showError(String(e));
  }
}

// ---------------------------------------------------------------------------
// Detail
// ---------------------------------------------------------------------------

async function renderDetail(main: HTMLElement): Promise<void> {
  main.innerHTML = `
    <div class="toolbar">
      <button id="back-btn" class="small">← Back</button>
      <h2 style="margin:0;">Idea</h2>
      <span></span>
    </div>
    <div class="panel">
      <div class="field"><label>Title</label><input id="d-title" type="text" /></div>
      <div class="field"><label>Body</label><textarea id="d-body" rows="4"></textarea></div>
      <div class="field"><label>Tags (comma-separated)</label><input id="d-tags" type="text" /></div>
      <div class="hint" id="d-meta"></div>
      <div class="row" style="margin-top: 8px;">
        <button id="d-save" class="primary">Save</button>
        <button id="d-delete" class="danger">Delete</button>
      </div>
    </div>

    <div class="panel">
      <h3>Evaluate</h3>
      <div class="checkbox-row" id="persona-checks"></div>
      <div class="row" style="margin-top: 10px;">
        <label style="margin:0; display:flex; align-items:center; gap:6px; color: var(--text);">
          <input id="summary-check" type="checkbox" checked /> Synthesize summary
        </label>
      </div>
      <div class="row" style="margin-top: 10px;">
        <button id="evaluate-btn" class="primary">Evaluate</button>
      </div>
      <div id="evaluate-result" style="margin-top: 12px;"></div>
    </div>

    <div class="panel">
      <h3>Resources</h3>
      <div id="resources-list"></div>
      <div class="row" style="margin-top: 10px;">
        <input id="res-url" type="text" placeholder="https://…" style="flex:1;" />
        <input id="res-title" type="text" placeholder="Title" style="flex:1;" />
        <select id="res-kind" style="flex:0 0 auto;">${RESOURCE_KINDS.map((k) => `<option value="${k}">${k}</option>`).join("")}</select>
        <button id="res-add" class="primary">Add</button>
      </div>
      <div class="row" style="margin-top: 8px;">
        <input id="res-note" type="text" placeholder="Note (optional)" style="flex:1;" />
      </div>
    </div>

    <div class="panel">
      <h3>History</h3>
      <ul id="history-list" class="list"></ul>
    </div>

    <div class="panel">
      <h3>Feedback</h3>
      <ul id="feedback-list" class="list"></ul>
      <div class="row" style="margin-top: 10px;">
        <input id="fb-author" type="text" placeholder="author" style="flex:1;" />
        <input id="fb-score" type="number" min="0" max="5" placeholder="0-5" style="flex:0 0 60px;" />
        <input id="fb-aspect" type="text" placeholder="aspect" style="flex:0 0 120px;" />
      </div>
      <div class="row" style="margin-top: 8px;">
        <input id="fb-rationale" type="text" placeholder="rationale" style="flex:1;" />
        <button id="fb-add" class="primary">Add feedback</button>
      </div>
    </div>
  `;

  $("back-btn").addEventListener("click", () => {
    state.ideaId = "";
    state.view = "ideas";
    void renderView();
  });
  $("d-save").addEventListener("click", () => void saveIdea());
  $("d-delete").addEventListener("click", () => void deleteIdeaFromDetail());
  $("evaluate-btn").addEventListener("click", () => void runEvaluate());
  $("res-add").addEventListener("click", () => void addResource());
  $("fb-add").addEventListener("click", () => void addFeedback());

  try {
    const idea = await App.GetIdea(state.ideaId);
    (document.getElementById("d-title") as HTMLInputElement).value = idea.title;
    (document.getElementById("d-body") as HTMLTextAreaElement).value = idea.body;
    (document.getElementById("d-tags") as HTMLInputElement).value = tagsText(idea.tags);
    $("d-meta").textContent = `Created ${formatDate(idea.created)} · Updated ${formatDate(idea.updated)}`;
  } catch (e) {
    showError(String(e));
  }

  await Promise.all([loadPersonas(), renderPersonaChecks(), renderResources(), renderHistory(), renderFeedback()]);
}

async function saveIdea(): Promise<void> {
  const title = inputValue("d-title").trim();
  const body = inputValue("d-body").trim();
  const tags = parseTags(inputValue("d-tags"));
  if (!title) {
    showError("Title is required.");
    return;
  }
  try {
    await App.UpdateIdea(state.ideaId, title, body, tags);
    await renderView();
  } catch (e) {
    showError(String(e));
  }
}

async function deleteIdeaFromDetail(): Promise<void> {
  try {
    await App.DeleteIdea(state.ideaId);
    state.ideaId = "";
    state.view = "ideas";
    await renderView();
  } catch (e) {
    showError(String(e));
  }
}

async function loadPersonas(): Promise<void> {
  try {
    state.personas = await App.ListPersonas();
  } catch (e) {
    showError(String(e));
    state.personas = [];
  }
}

function renderPersonaChecks(): void {
  const box = $("persona-checks");
  clear(box);
  for (const p of state.personas) {
    const label = el("label");
    const input = document.createElement("input");
    input.type = "checkbox";
    input.value = p.id;
    input.checked = true;
    label.appendChild(input);
    label.appendChild(document.createTextNode(p.name + (p.builtin ? "" : "")));
    box.appendChild(label);
  }
}

async function runEvaluate(): Promise<void> {
  const out = $("evaluate-result");
  clear(out);
  out.appendChild(el("div", "hint", "Evaluating…"));

  const selected: string[] = [];
  document.querySelectorAll<HTMLInputElement>("#persona-checks input").forEach((c) => {
    if (c.checked) selected.push(c.value);
  });
  const summarize = (document.getElementById("summary-check") as HTMLInputElement).checked;

  try {
    const score = await App.Evaluate(state.ideaId, selected, summarize);
    clear(out);
    out.appendChild(renderScore(score));
    await Promise.all([renderHistory(), loadCardsIfVisible()]);
  } catch (e) {
    clear(out);
    showError(String(e));
  }
}

function renderBreakdown(score: FeasibilityScore): HTMLElement {
  const breakdown = el("div", "breakdown");
  for (const ev of score.breakdown) {
    const line = el("div", "eval-line");
    if (ev.status === "success") {
      line.textContent = `${ev.persona_id}: ${ev.score}/5 — ${ev.rationale}`;
    } else {
      line.textContent = `${ev.persona_id}: failed — ${ev.error || ""}`;
    }
    breakdown.appendChild(line);
  }
  return breakdown;
}

function renderScore(score: FeasibilityScore): HTMLElement {
  const root = el("div");
  const total = el("div", "score-total " + scoreColor(score.total), score.total.toFixed(1));
  root.appendChild(total);
  root.appendChild(el("div", "hint", `${score.responded}/${score.requested} personas responded · spread ${score.spread.toFixed(1)}`));

  root.appendChild(renderBreakdown(score));

  if (score.verdict || score.summary) {
    const v = el("div", "verdict", score.verdict ? `Verdict: ${score.verdict}` : "");
    root.appendChild(v);
    if (score.summary) root.appendChild(el("div", "hint", score.summary));
  }
  return root;
}

async function loadCardsIfVisible(): Promise<void> {
  // The cards grid only exists on the ideas view; refresh it if present.
  const grid = document.getElementById("cards");
  if (grid) await loadCards();
}

async function renderHistory(): Promise<void> {
  const list = $("history-list");
  clear(list);
  try {
    const runs = await App.ListRuns(state.ideaId);
    if (runs.length === 0) {
      list.appendChild(el("li", "meta", "No evaluations yet."));
      return;
    }
    for (const r of runs) {
      const li = el("li");
      const toggle = el("button", "run-toggle",
        `Total ${r.total.toFixed(1)} · ${r.responded}/${r.requested} · spread ${r.spread.toFixed(1)}`);
      toggle.type = "button";
      toggle.addEventListener("click", () => {
        state.expandedRun = state.expandedRun === r.run_id ? "" : r.run_id;
        void renderHistory();
      });
      li.appendChild(toggle);

      if (state.expandedRun === r.run_id) {
        li.appendChild(renderBreakdown(r));
        if (r.verdict) li.appendChild(el("div", "verdict", `Verdict: ${r.verdict}`));
        if (r.summary) li.appendChild(el("div", "hint", r.summary));
      }
      li.appendChild(el("div", "meta", formatDate(r.created)));
      list.appendChild(li);
    }
  } catch (e) {
    showError(String(e));
  }
}

async function renderResources(): Promise<void> {
  const list = $("resources-list");
  clear(list);
  try {
    const resources = await App.ListResources(state.ideaId);
    if (resources.length === 0) {
      list.appendChild(el("div", "hint", "No resources yet."));
      return;
    }
    for (const r of resources) {
      const row = el("div", "row");
      row.style.marginBottom = "6px";
      const a = el("a", undefined, r.title || r.url) as HTMLAnchorElement;
      a.href = r.url;
      a.textContent = r.title || r.url;
      row.appendChild(a);
      row.appendChild(el("span", "meta", `[${r.kind}]`));
      const del = el("button", "small danger", "Remove");
      del.addEventListener("click", () => void removeResource(r.id));
      row.appendChild(del);
      list.appendChild(row);
    }
  } catch (e) {
    showError(String(e));
  }
}

async function addResource(): Promise<void> {
  const url = inputValue("res-url").trim();
  const title = inputValue("res-title").trim();
  const kind = inputValue("res-kind");
  const note = inputValue("res-note").trim();
  if (!url) {
    showError("URL is required.");
    return;
  }
  try {
    await App.AddResource(state.ideaId, url, title, kind, note);
    (document.getElementById("res-url") as HTMLInputElement).value = "";
    (document.getElementById("res-title") as HTMLInputElement).value = "";
    (document.getElementById("res-note") as HTMLInputElement).value = "";
    await renderResources();
  } catch (e) {
    showError(String(e));
  }
}

async function removeResource(id: string): Promise<void> {
  try {
    await App.DeleteResource(id);
    await renderResources();
  } catch (e) {
    showError(String(e));
  }
}

async function renderFeedback(): Promise<void> {
  const list = $("feedback-list");
  clear(list);
  try {
    const all = await App.ListFeedback(state.ideaId);
    if (all.length === 0) {
      list.appendChild(el("li", "meta", "No feedback yet."));
      return;
    }
    for (const f of all) {
      const li = el("li");
      li.appendChild(el("div", undefined, `${f.author}: ${f.score}/5 — ${f.rationale}`));
      if (f.aspect) li.appendChild(el("div", "meta", f.aspect));
      const del = el("button", "small danger", "Delete");
      del.style.marginTop = "6px";
      del.addEventListener("click", () => void deleteFeedback(f.id));
      li.appendChild(del);
      list.appendChild(li);
    }
  } catch (e) {
    showError(String(e));
  }
}

async function deleteFeedback(id: string): Promise<void> {
  try {
    await App.DeleteFeedback(id);
    await renderFeedback();
  } catch (e) {
    showError(String(e));
  }
}

async function addFeedback(): Promise<void> {
  const author = inputValue("fb-author").trim();
  const score = Number(inputValue("fb-score"));
  const rationale = inputValue("fb-rationale").trim();
  const aspect = inputValue("fb-aspect").trim();
  if (!author || isNaN(score) || score < 0 || score > 5) {
    showError("Author and a score 0-5 are required.");
    return;
  }
  try {
    await App.AddFeedback(state.ideaId, author, score, rationale, aspect);
    (document.getElementById("fb-author") as HTMLInputElement).value = "";
    (document.getElementById("fb-score") as HTMLInputElement).value = "";
    (document.getElementById("fb-rationale") as HTMLInputElement).value = "";
    (document.getElementById("fb-aspect") as HTMLInputElement).value = "";
    await renderFeedback();
  } catch (e) {
    showError(String(e));
  }
}

// ---------------------------------------------------------------------------
// Personas
// ---------------------------------------------------------------------------

async function renderPersonas(main: HTMLElement): Promise<void> {
  main.innerHTML = `
    <h2>Personas</h2>
    <div class="panel" id="persona-form">
      <h3>Create persona</h3>
      <div class="row">
        <input id="p-name" type="text" placeholder="name" style="flex:1;" />
        <input id="p-weight" type="number" step="0.1" value="1" style="flex:0 0 80px;" />
      </div>
      <div class="field" style="margin-top: 8px;">
        <textarea id="p-prompt" placeholder="system prompt"></textarea>
      </div>
      <button id="p-create" class="primary">Create</button>
      <div class="hint" style="margin-top:6px;">The ID and version are generated automatically.</div>
    </div>
    <div class="panel hidden" id="persona-edit-form">
      <h3>Edit persona</h3>
      <div class="row">
        <input id="pe-name" type="text" style="flex:1;" />
        <input id="pe-weight" type="number" step="0.1" value="1" style="flex:0 0 80px;" />
      </div>
      <div class="field" style="margin-top: 8px;">
        <textarea id="pe-prompt"></textarea>
      </div>
      <div class="row">
        <button id="pe-save" class="primary">Save</button>
        <button id="pe-cancel" class="small">Cancel</button>
      </div>
    </div>
    <ul id="personas-list" class="list"></ul>
  `;
  $("p-create").addEventListener("click", () => void createPersona());
  $("pe-save").addEventListener("click", () => void savePersona());
  $("pe-cancel").addEventListener("click", () => void cancelEditPersona());
  await loadPersonas();
  renderPersonasList();
}

function renderPersonasList(): void {
  const list = $("personas-list");
  clear(list);
  for (const p of state.personas) {
    const li = el("li");
    const head = el("div");
    head.appendChild(el("span", undefined, p.name));
    head.appendChild(el("span", "meta", ` ${p.id}`));
    head.appendChild(el("span", "meta", ` v${p.version}`));
    if (p.builtin) {
      head.appendChild(el("span", "badge builtin", "built-in"));
    }
    li.appendChild(head);
    li.appendChild(el("div", "meta", p.prompt));
    if (!p.builtin) {
      const row = el("div", "row");
      row.style.marginTop = "6px";
      const edit = el("button", "small", "Edit");
      edit.addEventListener("click", () => startEditPersona(p));
      const del = el("button", "small danger", "Delete");
      del.addEventListener("click", () => void deletePersona(p.id));
      row.appendChild(edit);
      row.appendChild(del);
      li.appendChild(row);
    }
    list.appendChild(li);
  }
}

function startEditPersona(p: PersonaView): void {
  state.editingPersona = p;
  $("persona-edit-form").classList.remove("hidden");
  (document.getElementById("pe-name") as HTMLInputElement).value = p.name;
  (document.getElementById("pe-prompt") as HTMLTextAreaElement).value = p.prompt;
  (document.getElementById("pe-weight") as HTMLInputElement).value = String(p.weight);
}

function cancelEditPersona(): void {
  state.editingPersona = null;
  $("persona-edit-form").classList.add("hidden");
}

async function savePersona(): Promise<void> {
  const target = state.editingPersona;
  if (!target) return;
  const name = inputValue("pe-name").trim();
  const prompt = inputValue("pe-prompt").trim();
  const weight = Number(inputValue("pe-weight")) || 0;
  if (!name || !prompt) {
    showError("Name and prompt are required.");
    return;
  }
  try {
    await App.UpdatePersona(target.id, name, prompt, weight);
    cancelEditPersona();
    await loadPersonas();
    renderPersonasList();
  } catch (e) {
    showError(String(e));
  }
}

async function createPersona(): Promise<void> {
  const name = inputValue("p-name").trim();
  const prompt = inputValue("p-prompt").trim();
  const weight = Number(inputValue("p-weight")) || 0;
  if (!name || !prompt) {
    showError("Name and prompt are required.");
    return;
  }
  try {
    await App.CreatePersona(name, prompt, weight);
    (document.getElementById("p-name") as HTMLInputElement).value = "";
    (document.getElementById("p-prompt") as HTMLInputElement).value = "";
    await loadPersonas();
    renderPersonasList();
  } catch (e) {
    showError(String(e));
  }
}

async function deletePersona(id: string): Promise<void> {
  try {
    await App.DeletePersona(id);
    await loadPersonas();
    renderPersonasList();
  } catch (e) {
    showError(String(e));
  }
}

// ---------------------------------------------------------------------------
// Provider
// ---------------------------------------------------------------------------

async function renderProvider(main: HTMLElement): Promise<void> {
  main.innerHTML = `
    <h2>LLM Provider</h2>
    <div class="panel">
      <h3>OpenAI</h3>
      <div class="hint" id="provider-status">Loading…</div>
    </div>
    <div class="panel">
      <h3>API keys</h3>
      <ul id="keys-list" class="list"></ul>
      <div class="row" style="margin-top: 10px;">
        <input id="key-label" type="text" placeholder="label (optional)" style="flex:1;" />
        <input id="key-value" type="password" placeholder="sk-…" style="flex:1;" />
      </div>
      <div class="row" style="margin-top: 8px;">
        <button id="key-add" class="primary">Add key</button>
      </div>
      <div class="hint" style="margin-top:6px;">Keys are stored in your OS keyring; only a masked suffix is shown.</div>
    </div>
  `;
  $("key-add").addEventListener("click", () => void addAPIKey());
  await loadProviderInfo();
  await renderKeys();
}

async function loadProviderInfo(): Promise<void> {
  const info = await App.GetProviderInfo();
  $("provider-status").textContent = `Provider: ${info.provider} · Model: ${info.model}`;
}

async function renderKeys(): Promise<void> {
  const list = $("keys-list");
  clear(list);
  try {
    const keys = await App.ListAPIKeys();
    if (keys.length === 0) {
      list.appendChild(el("li", "meta", "No API keys registered."));
      return;
    }
    for (const k of keys) {
      const li = el("li");
      const head = el("div");
      head.appendChild(el("span", undefined, k.label || "Unnamed"));
      head.appendChild(el("span", "meta", ` ${k.hint}`));
      if (k.is_default) {
        head.appendChild(el("span", "badge builtin", "default"));
      }
      li.appendChild(head);
      const row = el("div", "row");
      row.style.marginTop = "6px";
      if (!k.is_default) {
        const setDef = el("button", "small", "Set default");
        setDef.addEventListener("click", () => void setDefaultAPIKey(k.id));
        row.appendChild(setDef);
      }
      const del = el("button", "small danger", "Delete");
      del.addEventListener("click", () => void deleteAPIKey(k.id));
      row.appendChild(del);
      li.appendChild(row);
      list.appendChild(li);
    }
  } catch (e) {
    showError(String(e));
  }
}

async function addAPIKey(): Promise<void> {
  const label = inputValue("key-label").trim();
  const key = inputValue("key-value").trim();
  if (!key) {
    showError("Enter an API key first.");
    return;
  }
  try {
    await App.AddAPIKey(label, key);
    (document.getElementById("key-label") as HTMLInputElement).value = "";
    (document.getElementById("key-value") as HTMLInputElement).value = "";
    showSuccess("API key added.");
    await renderKeys();
  } catch (e) {
    showError(String(e));
  }
}

async function deleteAPIKey(id: string): Promise<void> {
  try {
    await App.DeleteAPIKey(id);
    showSuccess("API key deleted.");
    await renderKeys();
  } catch (e) {
    showError(String(e));
  }
}

async function setDefaultAPIKey(id: string): Promise<void> {
  try {
    await App.SetDefaultAPIKey(id);
    showSuccess("Default key updated.");
    await renderKeys();
  } catch (e) {
    showError(String(e));
  }
}

// ---------------------------------------------------------------------------
// Boot
// ---------------------------------------------------------------------------

renderWelcome();
