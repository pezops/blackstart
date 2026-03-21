const test = require("node:test");
const assert = require("node:assert/strict");
const fs = require("fs");
const os = require("os");
const path = require("path");

const buildChartIndex = require("./build-chart-index.js");
const helpers = buildChartIndex._test;

function mkTempDir() {
  return fs.mkdtempSync(path.join(os.tmpdir(), "chart-index-test-"));
}

function withEnv(vars, fn) {
  const previous = {};
  for (const [k, v] of Object.entries(vars)) {
    previous[k] = process.env[k];
    process.env[k] = v;
  }
  try {
    return fn();
  } finally {
    for (const k of Object.keys(vars)) {
      if (previous[k] === undefined) {
        delete process.env[k];
      } else {
        process.env[k] = previous[k];
      }
    }
  }
}

function createCoreMock() {
  return {
    logs: [],
    groups: [],
    info(msg) {
      this.logs.push(String(msg));
    },
    startGroup(msg) {
      this.groups.push(String(msg));
    },
    endGroup() {},
  };
}

function createGithubMock(releases) {
  return {
    paginate: async () => releases,
    rest: { repos: { listReleases: {} } },
  };
}

function writeHelmIndex(indexPath, versions) {
  const lines = [
    "apiVersion: v1",
    "entries:",
    "  blackstart:",
  ];
  for (const v of versions) {
    lines.push("    - apiVersion: v2");
    lines.push("      name: blackstart");
    lines.push(`      version: ${v}`);
    lines.push(`      urls:`);
    lines.push(`        - https://example.invalid/blackstart-chart-${v}.tgz`);
  }
  fs.writeFileSync(indexPath, lines.join("\n") + "\n");
}

test("parseReleaseAssets filters drafts and non-chart assets", () => {
  const assets = helpers.parseReleaseAssets([
    {
      draft: true,
      tag_name: "v1.0.0",
      assets: [{ name: "blackstart-chart-1.0.0.tgz", browser_download_url: "u1" }],
    },
    {
      draft: false,
      tag_name: "v1.0.1",
      assets: [
        { name: "blackstart-chart-1.0.1.tgz", browser_download_url: "u2" },
        { name: "blackstart-1.0.2.tgz", browser_download_url: "u3" },
        { name: "notes.txt", browser_download_url: "u4" },
      ],
    },
  ]);

  assert.deepEqual(
    assets.map((a) => ({ version: a.version, tag: a.tag, name: a.name })),
    [
      { version: "1.0.1", tag: "v1.0.1", name: "blackstart-chart-1.0.1.tgz" },
      { version: "1.0.2", tag: "v1.0.1", name: "blackstart-1.0.2.tgz" },
    ],
  );
});

test("extractExistingVersions reads versions from index content", () => {
  const dir = mkTempDir();
  const idx = path.join(dir, "index.yaml");
  writeHelmIndex(idx, ["0.1.0", "0.1.1"]);

  const versions = helpers.extractExistingVersions(idx);
  assert.equal(versions.has("0.1.0"), true);
  assert.equal(versions.has("0.1.1"), true);
});

test("hasBlackstartEntries handles empty and populated index", () => {
  const dir = mkTempDir();
  const empty = path.join(dir, "empty.yaml");
  fs.writeFileSync(empty, "apiVersion: v1\nentries:\n  blackstart: []\n");
  assert.equal(helpers.hasBlackstartEntries(empty), false);

  const populated = path.join(dir, "pop.yaml");
  writeHelmIndex(populated, ["1.2.3"]);
  assert.equal(helpers.hasBlackstartEntries(populated), true);
});

test("buildChartIndex writes empty index when no assets and no local chart", async () => {
  const dir = mkTempDir();
  const output = path.join(dir, "out", "index.yaml");
  const core = createCoreMock();

  await withEnv(
    {
      GH_TOKEN: "token",
      GITHUB_REPOSITORY: "pezops/blackstart",
      CHART_INDEX_OUTPUT: output,
      CHART_WORK_DIR: dir,
      CHART_INCLUDE_LOCAL: "false",
    },
    () =>
      buildChartIndex(
        {
          github: createGithubMock([]),
          context: { repo: { owner: "pezops", repo: "blackstart" } },
          core,
        },
        {
          fetchImpl: async () => ({ ok: false }),
        },
      ),
  );

  const content = fs.readFileSync(output, "utf-8");
  assert.match(content, /entries:\s*\{\}/);
  assert.equal(core.logs.some((x) => x.includes("No published chart assets found")), true);
});

test("buildChartIndex merges assets, includes local chart, and prints output", async () => {
  const dir = mkTempDir();
  const output = path.join(dir, "site", "charts", "index.yaml");
  const core = createCoreMock();
  const chartPackagesDir = path.join(dir, "chart-packages");
  fs.mkdirSync(chartPackagesDir, { recursive: true });

  const releases = [
    {
      draft: false,
      tag_name: "v0.1.0",
      assets: [
        {
          name: "blackstart-chart-0.1.0.tgz",
          browser_download_url: "https://example.invalid/blackstart-chart-0.1.0.tgz",
        },
      ],
    },
  ];

  const runCommand = (cmd, args) => {
    assert.equal(cmd, "helm");
    if (args[0] === "package") {
      const version = args[args.indexOf("--version") + 1];
      const dest = args[args.indexOf("--destination") + 1];
      fs.writeFileSync(path.join(dest, `blackstart-${version}.tgz`), "local-chart");
      return;
    }
    if (args[0] === "repo" && args[1] === "index") {
      const chartDir = args[2];
      const mergePath = args[args.indexOf("--merge") + 1];
      const mergedVersions = helpers.extractExistingVersions(mergePath);
      const files = fs.readdirSync(chartDir).filter((f) => f.endsWith(".tgz"));
      for (const f of files) {
        const m = f.match(/^blackstart(?:-chart)?-([0-9A-Za-z.+-]+)\.tgz$/);
        if (m) mergedVersions.add(m[1]);
      }
      writeHelmIndex(path.join(chartDir, "index.yaml"), [...mergedVersions].sort());
      return;
    }
    throw new Error(`Unexpected command: ${cmd} ${args.join(" ")}`);
  };

  await withEnv(
    {
      GH_TOKEN: "token",
      GITHUB_REPOSITORY: "pezops/blackstart",
      CHART_INDEX_OUTPUT: output,
      CHART_WORK_DIR: dir,
      CHART_INCLUDE_LOCAL: "true",
      CHART_LOCAL_VERSION: "0.1.1",
      CHART_LOCAL_TAG: "v0.1.1",
      CHART_REQUIRE_VERSION: "0.1.1",
      CHART_PRINT_OUTPUT: "true",
    },
    () =>
      buildChartIndex(
        {
          github: createGithubMock(releases),
          context: { repo: { owner: "pezops", repo: "blackstart" } },
          core,
        },
        {
          fetchImpl: async () => ({ ok: false }),
          downloadImpl: async (_url, outPath) => {
            fs.writeFileSync(outPath, "remote-chart");
          },
          runCommand,
        },
      ),
  );

  const content = fs.readFileSync(output, "utf-8");
  assert.match(content, /version: 0\.1\.0/);
  assert.match(content, /version: 0\.1\.1/);
  assert.equal(core.groups.includes("Generated chart index"), true);
});

test("buildChartIndex errors when local mode is enabled without version/tag", async () => {
  const dir = mkTempDir();
  const output = path.join(dir, "out", "index.yaml");
  const core = createCoreMock();

  await assert.rejects(
    () =>
      withEnv(
        {
          GH_TOKEN: "token",
          GITHUB_REPOSITORY: "pezops/blackstart",
          CHART_INDEX_OUTPUT: output,
          CHART_WORK_DIR: dir,
          CHART_INCLUDE_LOCAL: "true",
          CHART_LOCAL_VERSION: "",
          CHART_LOCAL_TAG: "",
        },
        () =>
          buildChartIndex(
            {
              github: createGithubMock([]),
              context: { repo: { owner: "pezops", repo: "blackstart" } },
              core,
            },
            {
              fetchImpl: async () => ({ ok: false }),
              downloadImpl: async (_url, outPath) => fs.writeFileSync(outPath, "remote-chart"),
              runCommand: () => {},
            },
          ),
      ),
    /CHART_LOCAL_VERSION and CHART_LOCAL_TAG are required/,
  );
});
