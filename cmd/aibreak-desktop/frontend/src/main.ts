const App = window.go.main.App;

function $(id: string): HTMLElement {
  const el = document.getElementById(id);
  if (!el) throw new Error(`missing element #${id}`);
  return el;
}

function inputValue(id: string): string {
  return (document.getElementById(id) as HTMLInputElement).value;
}

function show(view: "ideas" | "detail"): void {
  ($("view-ideas") as HTMLElement).hidden = view !== "ideas";
  ($("view-detail") as HTMLElement).hidden = view !== "detail";
}

async function renderIdeas(): Promise<void> {
  const tag = inputValue("tag-filter").trim();
  const ideas = await App.ListIdeas(tag);
  const list = $("ideas-list");
  list.innerHTML = "";
  for (const idea of ideas) {
    const li = document.createElement("li");
    li.textContent = idea.title;
    li.addEventListener("click", () => renderDetail(idea.id));
    list.appendChild(li);
  }
}

async function renderDetail(id: string): Promise<void> {
  const idea = await App.GetIdea(id);
  ($("detail-title") as HTMLElement).textContent = idea.title;
  ($("detail-body") as HTMLElement).textContent = idea.body;
  ($("evaluate-result") as HTMLElement).textContent = "";
  await Promise.all([renderHistory(id), renderFeedback(id)]);
  ($("evaluate-btn") as HTMLButtonElement).onclick = () => runEvaluate(id);
  ($("fb-add-btn") as HTMLButtonElement).onclick = () => addFeedback(id);
  show("detail");
}

async function runEvaluate(id: string): Promise<void> {
  const out = $("evaluate-result") as HTMLElement;
  out.textContent = "Evaluating…";
  const raw = inputValue("personas-input").trim();
  const personaIDs = raw === "" ? [] : raw.split(",").map((s) => s.trim()).filter((s) => s !== "");
  const summarize = (document.getElementById("summary-check") as HTMLInputElement).checked;
  try {
    const score = await App.Evaluate(id, personaIDs, summarize);
    let text = `Total: ${score.total.toFixed(1)} (${score.responded}/${score.requested} personas)\n`;
    for (const ev of score.breakdown) {
      text += ev.status === "success"
        ? `  ${ev.persona_id}: ${ev.score}/5 — ${ev.rationale}\n`
        : `  ${ev.persona_id}: failed — ${ev.error}\n`;
    }
    if (score.verdict || score.summary) {
      text += `Verdict: ${score.verdict ?? ""}\n${score.summary ?? ""}\n`;
    }
    out.textContent = text;
    await renderHistory(id);
  } catch (e) {
    out.textContent = `Error: ${e}`;
  }
}

async function renderHistory(id: string): Promise<void> {
  const runs = await App.ListRuns(id);
  const list = $("history-list");
  list.innerHTML = "";
  for (const r of runs) {
    const li = document.createElement("li");
    li.textContent = r.verdict
      ? `run ${r.run_id}: total ${r.total.toFixed(1)} verdict ${r.verdict}`
      : `run ${r.run_id}: total ${r.total.toFixed(1)} (${r.responded}/${r.requested})`;
    list.appendChild(li);
  }
}

async function renderFeedback(id: string): Promise<void> {
  const all = await App.ListFeedback(id);
  const list = $("feedback-list");
  list.innerHTML = "";
  for (const f of all) {
    const li = document.createElement("li");
    li.textContent = `${f.author}: ${f.score}/5 — ${f.rationale}`;
    list.appendChild(li);
  }
}

async function addFeedback(id: string): Promise<void> {
  const author = inputValue("fb-author").trim();
  const score = Number(inputValue("fb-score"));
  await App.AddFeedback(id, author, score, inputValue("fb-rationale"), inputValue("fb-aspect"));
  await renderFeedback(id);
}

document.querySelector('button[data-view="ideas"]')?.addEventListener("click", () => {
  show("ideas");
  void renderIdeas();
});
($("tag-filter-btn") as HTMLButtonElement).addEventListener("click", () => void renderIdeas());
($("back-btn") as HTMLButtonElement).addEventListener("click", () => {
  show("ideas");
  void renderIdeas();
});

void renderIdeas();
