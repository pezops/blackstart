const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

function run(cmd, args, opts = {}) {
  const res = spawnSync(cmd, args, { stdio: "pipe", encoding: "utf-8", ...opts });
  if (res.status !== 0) {
    throw new Error(`${cmd} ${args.join(" ")} failed:\n${res.stdout || ""}\n${res.stderr || ""}`);
  }
  return res.stdout || "";
}

async function downloadFile(url, outPath, token) {
  const res = await fetch(url, {
    headers: {
      Authorization: `Bearer ${token}`,
      Accept: "application/octet-stream",
    },
  });
  if (!res.ok) {
    throw new Error(`download failed ${url}: ${res.status} ${res.statusText}`);
  }
  const arr = await res.arrayBuffer();
  fs.writeFileSync(outPath, Buffer.from(arr));
}

function ensureDir(p) {
  fs.mkdirSync(p, { recursive: true });
}

function extractVersionsFromContent(content) {
  const versions = new Set();
  const re = /^\s*version:\s*["']?([0-9A-Za-z.+-]+)["']?\s*$/gm;
  for (const m of content.matchAll(re)) {
    versions.add(m[1]);
  }
  return versions;
}

function extractExistingVersions(indexPath) {
  if (!fs.existsSync(indexPath) || fs.statSync(indexPath).size === 0) {
    return new Set();
  }
  return extractVersionsFromContent(fs.readFileSync(indexPath, "utf-8"));
}

function hasBlackstartEntries(indexPath) {
  if (!fs.existsSync(indexPath) || fs.statSync(indexPath).size === 0) {
    return false;
  }
  const content = fs.readFileSync(indexPath, "utf-8");
  if (/^\s*blackstart:\s*\[\s*\]\s*$/m.test(content)) {
    return false;
  }
  return /^\s*blackstart:\s*$/m.test(content) && /^\s*-\s*apiVersion:\s*v2\s*$/m.test(content);
}

function hasStaleVersions(indexPath, validVersions) {
  const inIndex = extractExistingVersions(indexPath);
  for (const v of inIndex) {
    if (!validVersions.has(v)) {
      return true;
    }
  }
  return false;
}

function writeEmptyIndex(outPath) {
  ensureDir(path.dirname(outPath));
  fs.writeFileSync(outPath, "apiVersion: v1\nentries: {}\n");
}

function parseReleaseAssets(releases) {
  const assets = [];
  for (const rel of releases || []) {
    if (rel.draft) continue;
    for (const a of rel.assets || []) {
      const m = a.name.match(/^blackstart(?:-chart)?-([0-9A-Za-z.+-]+)\.tgz$/);
      if (!m) continue;
      assets.push({
        version: m[1],
        tag: rel.tag_name,
        name: a.name,
        url: a.browser_download_url,
      });
    }
  }
  return assets;
}

function createPaths(workDir, outputPath) {
  const chartPackagesDir = path.join(workDir, "chart-packages");
  return {
    chartPackagesDir,
    existingIndexPath: path.join(workDir, "existing-index.yaml"),
    mergedIndexPath: path.join(workDir, "merged-index.yaml"),
    repoIndexPath: path.join(chartPackagesDir, "index.yaml"),
    outputPath,
  };
}

async function buildChartIndex({ github, context, core }, deps = {}) {
  const ghToken = process.env.GH_TOKEN || process.env.GITHUB_TOKEN;
  if (!ghToken) {
    throw new Error("GH_TOKEN or GITHUB_TOKEN is required.");
  }

  const repoFull = process.env.GITHUB_REPOSITORY || `${context.repo.owner}/${context.repo.repo}`;
  const [owner, repo] = repoFull.split("/");

  const output = process.env.CHART_INDEX_OUTPUT;
  if (!output) {
    throw new Error("CHART_INDEX_OUTPUT is required.");
  }

  const existingIndexUrl = process.env.CHART_EXISTING_INDEX_URL || "";
  const includeLocal = (process.env.CHART_INCLUDE_LOCAL || "false").toLowerCase() === "true";
  const localVersion = process.env.CHART_LOCAL_VERSION || "";
  const localTag = process.env.CHART_LOCAL_TAG || "";
  const requireVersion = process.env.CHART_REQUIRE_VERSION || "";
  const printOutput = (process.env.CHART_PRINT_OUTPUT || "false").toLowerCase() === "true";
  const workDir = process.env.CHART_WORK_DIR || "/tmp";

  const runCommand = deps.runCommand || run;
  const fetchImpl = deps.fetchImpl || fetch;
  const downloadImpl =
    deps.downloadImpl ||
    (async (url, outPath) => {
      const res = await fetchImpl(url, {
        headers: {
          Authorization: `Bearer ${ghToken}`,
          Accept: "application/octet-stream",
        },
      });
      if (!res.ok) {
        throw new Error(`download failed ${url}: ${res.status} ${res.statusText}`);
      }
      const arr = await res.arrayBuffer();
      fs.writeFileSync(outPath, Buffer.from(arr));
    });

  const paths = createPaths(workDir, output);
  ensureDir(paths.chartPackagesDir);
  ensureDir(path.dirname(paths.outputPath));

  if (existingIndexUrl) {
    try {
      const res = await fetchImpl(existingIndexUrl);
      if (res.ok) {
        fs.writeFileSync(paths.existingIndexPath, Buffer.from(await res.arrayBuffer()));
      }
    } catch (_) {
      // Fallback below.
    }
  }

  if (!fs.existsSync(paths.existingIndexPath) || fs.statSync(paths.existingIndexPath).size === 0) {
    fs.writeFileSync(paths.existingIndexPath, "apiVersion: v1\nentries: {}\n");
  }
  fs.copyFileSync(paths.existingIndexPath, paths.mergedIndexPath);

  const releases = await github.paginate(github.rest.repos.listReleases, {
    owner,
    repo,
    per_page: 100,
  });
  const assets = parseReleaseAssets(releases);

  if (assets.length === 0 && !includeLocal) {
    writeEmptyIndex(paths.outputPath);
    core.info("No published chart assets found; wrote empty chart index.");
    return;
  }

  async function mergeAsset(asset) {
    const out = path.join(paths.chartPackagesDir, asset.name);
    await downloadImpl(asset.url, out, ghToken);
    runCommand("helm", [
      "repo",
      "index",
      paths.chartPackagesDir,
      "--url",
      `https://github.com/${repoFull}/releases/download/${asset.tag}`,
      "--merge",
      paths.mergedIndexPath,
    ]);
    fs.renameSync(paths.repoIndexPath, paths.mergedIndexPath);
    fs.rmSync(out, { force: true });
  }

  function packageAndMergeLocalChart() {
    runCommand("helm", [
      "package",
      "charts/blackstart",
      "--version",
      localVersion,
      "--app-version",
      localVersion,
      "--destination",
      paths.chartPackagesDir,
    ]);

    const source = path.join(paths.chartPackagesDir, `blackstart-${localVersion}.tgz`);
    const targetName = `blackstart-chart-${localVersion}.tgz`;
    const target = path.join(paths.chartPackagesDir, targetName);
    fs.renameSync(source, target);

    runCommand("helm", [
      "repo",
      "index",
      paths.chartPackagesDir,
      "--url",
      `https://github.com/${repoFull}/releases/download/${localTag}`,
      "--merge",
      paths.mergedIndexPath,
    ]);
    fs.renameSync(paths.repoIndexPath, paths.mergedIndexPath);
    fs.rmSync(target, { force: true });
  }

  const validVersions = new Set(assets.map((a) => a.version));
  const existingVersions = extractExistingVersions(paths.existingIndexPath);
  const missing = assets.filter((a) => !existingVersions.has(a.version));

  for (const asset of missing) {
    await mergeAsset(asset);
    core.info(`Merged missing chart version ${asset.version} from ${asset.tag}.`);
  }

  if (!hasBlackstartEntries(paths.mergedIndexPath) && assets.length > 0) {
    core.info("Merged index has no blackstart entries; rebuilding from all published chart assets.");
    fs.writeFileSync(paths.mergedIndexPath, "apiVersion: v1\nentries: {}\n");
    for (const asset of assets) {
      await mergeAsset(asset);
      core.info(`Rebuilt chart version ${asset.version} from ${asset.tag}.`);
    }
  }

  if (includeLocal) {
    if (!localVersion || !localTag) {
      throw new Error("CHART_LOCAL_VERSION and CHART_LOCAL_TAG are required when CHART_INCLUDE_LOCAL=true.");
    }
    packageAndMergeLocalChart();
    validVersions.add(localVersion);
  }

  if (hasStaleVersions(paths.mergedIndexPath, validVersions) && assets.length > 0) {
    core.info("Detected stale chart versions in merged index; rebuilding from currently published releases.");
    fs.writeFileSync(paths.mergedIndexPath, "apiVersion: v1\nentries: {}\n");
    for (const asset of assets) {
      await mergeAsset(asset);
    }
    if (includeLocal) {
      packageAndMergeLocalChart();
    }
  }

  fs.copyFileSync(paths.mergedIndexPath, paths.outputPath);
  const outContent = fs.readFileSync(paths.outputPath, "utf-8");

  if (/^\s*blackstart:\s*\[\s*\]\s*$/m.test(outContent)) {
    throw new Error(`Chart index ended with empty blackstart entries:\n${outContent}`);
  }
  if (requireVersion && !outContent.includes(`version: ${requireVersion}`)) {
    throw new Error(`Chart index missing expected version ${requireVersion}:\n${outContent}`);
  }

  if (printOutput) {
    core.startGroup("Generated chart index");
    core.info(outContent);
    core.endGroup();
  }
}

module.exports = buildChartIndex;
module.exports._test = {
  createPaths,
  extractExistingVersions,
  extractVersionsFromContent,
  hasBlackstartEntries,
  hasStaleVersions,
  parseReleaseAssets,
  writeEmptyIndex,
  run,
  downloadFile,
};
