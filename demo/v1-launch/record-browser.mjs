// Records the full v1.0.0 launch demo as one continuous video: a sequence
// of custom HTML "scene" pages (title cards, terminal typing animation,
// findings, compare/rollback) interleaved with navigations into the real,
// sanitized report.html and the real embedded Console loaded with real
// SEC-TRUST-002 evidence -- so the "browser" footage in the final video is
// never a mockup, only the title/finding/compare cards are custom-built.
//
// Requires `playwright` installed locally (not a project dependency --
// this is a one-off recording tool, not something the built product or
// its CI needs). Requires BASE_URL to point at a static server whose root
// contains:
//   /scenes/<NN-name>.html   -- the custom scene pages in this directory
//   /report.html             -- the real sanitized scan report.html
//   /console/                -- the real built Console (web/dist)
//   /console/fixtures/*.json -- real sanitized evidence, co-located with
//                                the Console so fetch() stays same-origin
//
// Usage: BASE_URL=http://localhost:PORT OUT_DIR=./recordings node record-browser.mjs
import { chromium } from 'playwright';
import { mkdirSync, readdirSync, renameSync } from 'node:fs';
import { join } from 'node:path';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8899';
const OUT_DIR = process.env.OUT_DIR || './recordings';
const CHROMIUM_PATH = process.env.CHROMIUM_PATH || undefined;
const FADE_MS = 260;

mkdirSync(OUT_DIR, { recursive: true });

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, Math.max(0, ms)));
}

// Navigates, optionally waits for the scene's own "ready" signal (its
// entrance animation finishing), then sleeps exactly enough that this
// scene's total on-screen time -- measured from the moment navigation
// started -- equals targetMs, fading out over the last FADE_MS of that
// budget. Wall-clock-based on purpose: waiting for sceneReady takes a
// variable amount of time (typing animations, staggered card reveals),
// and a fixed extra wait() on top of that silently overshoots the
// intended total duration -- confirmed by an early cut of this script
// running 36.6s against a 30s target for exactly this reason.
async function playScene(page, url, targetMs, { waitForReady = true, fade = true } = {}) {
  const start = Date.now();
  await page.goto(url, { waitUntil: 'load' });
  if (waitForReady) {
    await page.waitForFunction(() => window.sceneReady === true, { timeout: targetMs + 2000 });
  }
  const fadeMs = fade ? FADE_MS : 0;
  const elapsed = Date.now() - start;
  await wait(targetMs - fadeMs - elapsed);
  if (fade) {
    await page.evaluate(() => window.fadeOut && window.fadeOut());
    await wait(fadeMs);
  }
}

async function redactClusterName(page) {
  await page.evaluate(() => {
    const target = 'kp-v1-rc-smoke';
    const replacement = 'redacted-eks-cluster';
    document.title = document.title.split(target).join(replacement);
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    const nodes = [];
    let node;
    while ((node = walker.nextNode())) nodes.push(node);
    for (const n of nodes) {
      if (n.nodeValue && n.nodeValue.includes(target)) {
        n.nodeValue = n.nodeValue.split(target).join(replacement);
      }
    }
  });
}

async function overlay(page, text) {
  // A solid full-width strip pinned to the bottom edge, not a floating box
  // over page content -- the real report.html and Console pages are dense
  // enough that a floating caption landed on top of real table rows/notes
  // in an early cut, reading as a rendering glitch rather than a caption.
  // A screen-edge bar can never collide with content that way.
  await page.evaluate((caption) => {
    const el = document.createElement('div');
    el.textContent = caption;
    Object.assign(el.style, {
      position: 'fixed',
      left: '0',
      right: '0',
      bottom: '0',
      width: '100%',
      textAlign: 'center',
      fontFamily: '"Plex Mono", ui-monospace, monospace',
      fontSize: '24px',
      color: '#eaf2ef',
      background: '#0a1211',
      borderTop: '1px solid #23302e',
      padding: '18px 20px',
      zIndex: 999999,
      letterSpacing: '0.01em'
    });
    document.body.appendChild(el);
  }, text);
}

async function main() {
  const browser = await chromium.launch({ executablePath: CHROMIUM_PATH });
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    recordVideo: { dir: OUT_DIR, size: { width: 1920, height: 1080 } }
  });
  const page = await context.newPage();
  const t0 = Date.now();

  // 0.0s - 3.0s: opening title card (no entrance animation to wait for)
  await playScene(page, `${BASE_URL}/scenes/01-title-open.html`, 3000, { waitForReady: false });

  // 3.0s - 8.0s: terminal typing animation (real command, real captured output)
  await playScene(page, `${BASE_URL}/scenes/02-terminal.html`, 5000);

  // 8.0s - 12.0s: key findings
  await playScene(page, `${BASE_URL}/scenes/03-findings.html`, 4000);

  // 12.0s - 13.5s: report-format overview card
  await playScene(page, `${BASE_URL}/scenes/04-reports-overview.html`, 1500, { fade: false });

  // 13.5s - 16.0s: the real, sanitized report.html
  {
    const start = Date.now();
    await page.goto(`${BASE_URL}/report.html`, { waitUntil: 'load' });
    // Cosmetic public-distribution redaction: the sanitized evidence still
    // carries the real disposable-cluster identifier "kp-v1-rc-smoke" (not
    // a secret -- no ARN/account-id -- but visibly inconsistent with the
    // "redacted-eks-cluster" name the terminal scene deliberately shows).
    // This mutates only the live DOM of *this recording's* browser tab,
    // after page load -- evidence/scan-report.html on disk is untouched,
    // and score/verdict/findings/remediation text is untouched.
    await redactClusterName(page);
    await overlay(page, 'CLI · JSON · Markdown · HTML');
    await wait(2500 - (Date.now() - start));
  }

  // 16.0s - 22.0s: the real embedded Console, loaded with real evidence
  {
    const start = Date.now();
    await page.goto(
      `${BASE_URL}/console/?findings=${BASE_URL}/console/fixtures/findings.json` +
        `&plan=${BASE_URL}/console/fixtures/upgrade-plan.json` +
        `&rollback=${BASE_URL}/console/fixtures/rollback-assessment.json`,
      { waitUntil: 'networkidle' }
    );
    await overlay(page, 'Reviewable evidence, not a pass/fail guess');
    await wait(1300);
    await page.getByRole('tab', { name: /Findings/i }).click();
    await wait(1300);
    await page.getByRole('tab', { name: /Next Actions/i }).click();
    await wait(1300);
    await page.getByRole('tab', { name: /Rollback/i }).click();
    await wait(6000 - (Date.now() - start));
  }

  // 22.0s - 27.0s: compare + rollback summary card
  await playScene(page, `${BASE_URL}/scenes/06-compare-rollback.html`, 5000);

  // 27.0s - 30.0s: closing title card
  await playScene(page, `${BASE_URL}/scenes/07-title-close.html`, 3000, { waitForReady: false, fade: false });

  console.log(`Total scripted timeline: ${((Date.now() - t0) / 1000).toFixed(1)}s`);

  await page.close();
  await context.close();
  await browser.close();

  // Playwright names the video file after an internal page GUID -- rename
  // to something predictable for the render script to pick up.
  const files = readdirSync(OUT_DIR).filter((f) => f.endsWith('.webm'));
  if (files.length === 1) {
    renameSync(join(OUT_DIR, files[0]), join(OUT_DIR, 'raw-capture.webm'));
    console.log(`Recording saved: ${join(OUT_DIR, 'raw-capture.webm')}`);
  } else {
    console.log(`Recording(s) saved in ${OUT_DIR}:`, files);
  }
}

main().catch((err) => {
  console.error('RECORD_FAIL', err);
  process.exit(1);
});
