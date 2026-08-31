import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

test("shared shell can reflow below the historical 760px floor", async () => {
  const [tokens, responsive] = await Promise.all([
    readFile(new URL("../src/styles/tokens.css", import.meta.url), "utf8"),
    readFile(new URL("../src/styles/responsive.css", import.meta.url), "utf8"),
  ]);
  assert.doesNotMatch(tokens, /min-width:\s*760px/);
  assert.match(tokens, /min-width:\s*320px/);
  assert.match(responsive, /max-width:\s*720px/);
  assert.match(responsive, /max-width:\s*480px/);
});

test("themes share geometry and the settings controls use a grouped layout", async () => {
  const [tokens, pages, settings] = await Promise.all([
    readFile(new URL("../src/styles/tokens.css", import.meta.url), "utf8"),
    readFile(new URL("../src/styles/pages.css", import.meta.url), "utf8"),
    readFile(new URL("../src/features/settings/SettingsPage.vue", import.meta.url), "utf8"),
  ]);
  const dayTheme = tokens.slice(tokens.indexOf(':root[data-theme="day"]'));
  assert.doesNotMatch(dayTheme, /--radius-(?:lg|md|sm|chip):/);
  assert.doesNotMatch(dayTheme, /steps\(/);
  assert.match(settings, /startup-settings-controls/);
  assert.match(pages, /\.startup-settings-controls/);
  assert.match(pages, /\.route-required-callout/);
});

test("navigation hierarchy is compact without removing accessible descriptions", async () => {
  const [app, shell] = await Promise.all([
    readFile(new URL("../src/App.vue", import.meta.url), "utf8"),
    readFile(new URL("../src/styles/shell.css", import.meta.url), "utf8"),
  ]);
  assert.match(app, /:aria-label="`\$\{item\.label\}：\$\{item\.description\}`"/);
  assert.match(shell, /sidebar nav button:not\(\.active\) small/);
  assert.match(shell, /\.feedback-panel/);
});
