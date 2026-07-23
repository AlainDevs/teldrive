import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

import { build, createServer, loadConfigFromFile, normalizePath, resolveConfig } from "vite";

const dirname = path.dirname(fileURLToPath(import.meta.url));
const appRoot = dirname;
const configFile = path.join(appRoot, "vite.config.mts");
const appNodeModules = path.resolve(appRoot, "node_modules");
const fileBrowserSource = path.join(appNodeModules, "file-browser/src/index.ts");
const assetsDir = path.join(appRoot, "dist/assets");
const reactRuntimeIds = ["react", "react-dom", "react/jsx-runtime", "react/jsx-dev-runtime"];

async function loadBuildConfig() {
  const result = await loadConfigFromFile(
    { command: "build", mode: "production" },
    configFile,
    appRoot,
  );

  assert.ok(result, "Vite config should load");

  return result.config;
}

async function resolveBuildConfig() {
  return resolveConfig(
    { configFile, root: appRoot },
    "build",
    "production",
    "production",
  );
}

test("file-browser package resolves to its source export", async () => {
  const packageJson = JSON.parse(
    await readFile(path.join(appNodeModules, "file-browser/package.json"), "utf8"),
  );

  assert.equal(packageJson.exports?.["."]?.import, "./src/index.ts");
  assert.equal(
    normalizePath(path.resolve(appNodeModules, "file-browser", packageJson.exports["."].import)),
    normalizePath(fileBrowserSource),
  );
});

test("React runtimes are deduped to app node_modules for sibling source", async () => {
  const config = await loadBuildConfig();

  assert.ok(config.resolve?.dedupe, "resolve.dedupe should be configured");
  assert.equal(config.resolve.preserveSymlinks, true);
  for (const id of reactRuntimeIds) {
    assert.ok(config.resolve.dedupe.includes(id), `${id} should be deduped`);
  }

  const resolved = await resolveBuildConfig();
  for (const id of reactRuntimeIds) {
    assert.ok(
      resolved.resolve.dedupe.includes(id),
      `${id} should remain deduped after config resolution`,
    );
  }

  const devServer = await createServer({ configFile, root: appRoot });
  try {
    for (const id of reactRuntimeIds) {
      const resolution = await devServer.pluginContainer.resolveId(id, fileBrowserSource);
      assert.ok(resolution, `${id} should resolve from file-browser source`);
      assert.match(
        normalizePath(resolution.id),
        new RegExp(`^${normalizePath(appNodeModules).replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}/`),
        `${id} should resolve from app node_modules, got ${resolution.id}`,
      );
    }
  } finally {
    await devServer.close();
  }
});

test("production bundle contains one React core", async () => {
  await build({ configFile, root: appRoot });

  const assets = await readdir(assetsDir);
  const reactCoreAssets = [];
  const reactVersions = new Set();
  for (const asset of assets.filter((name) => name.endsWith(".js"))) {
    const source = await readFile(path.join(assetsDir, asset), "utf8");
    if (source.includes("__CLIENT_INTERNALS_DO_NOT_USE_OR_WARN_USERS_THEY_CANNOT_UPGRADE")) {
      reactCoreAssets.push(asset);
    }
    for (const match of source.matchAll(/\.version="(19\.[^"]+)"/g)) {
      reactVersions.add(match[1]);
    }
  }

  assert.equal(
    reactCoreAssets.length,
    1,
    `expected React internals in one chunk, found ${JSON.stringify(reactCoreAssets)}`,
  );
  assert.deepEqual([...reactVersions], ["19.2.6"]);
});
