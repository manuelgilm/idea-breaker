// Copies static assets next to the compiled JS in dist/.
// Portable across OSes (used by `npm run build`).
import { cpSync, mkdirSync } from "node:fs";

const dist = new URL("../dist/", import.meta.url);
mkdirSync(dist, { recursive: true });
cpSync(new URL("../index.html", import.meta.url), new URL("../dist/index.html", import.meta.url));
cpSync(new URL("../src/style.css", import.meta.url), new URL("../dist/style.css", import.meta.url));
console.log("assets copied to dist/");
