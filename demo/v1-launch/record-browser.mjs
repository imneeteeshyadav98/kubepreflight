// Records the v1.0.0 launch demo as one continuous video: a sequence of
// custom HTML "scene" pages (title cards, terminal typing animation,
// findings, compare/rollback) interleaved with navigations into the real,
// sanitized report.html and the real embedded Console loaded with real
// SEC-TRUST-002 evidence -- so the "browser" footage in the final video is
// never a mockup, only the title/finding/compare cards are custom-built.
//
// Two recording modes, selected by VARIANT:
//   VARIANT=master   (default) -- the full 30.0s launch demo (title,
//                     terminal, findings, reports overview, report.html,
//                     Console, compare/rollback, closing title). Output:
//                     recordings/raw-capture.webm
//   VARIANT=linkedin -- a standalone 15.8s LinkedIn teaser that has to read
//                     correctly with no post caption for context: a
//                     dedicated opening title card, the same terminal/
//                     findings/reports-overview/report.html beats
//                     (trimmed to fit), and a dedicated closing CTA card.
//                     Output: recordings/linkedin-raw-capture.webm
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
//        VARIANT=linkedin BASE_URL=... OUT_DIR=... node record-browser.mjs
import { chromium } from 'playwright';
import { mkdirSync, readdirSync, renameSync } from 'node:fs';
import { join } from 'node:path';

const BASE_URL = process.env.BASE_URL || 'http://localhost:8899';
const OUT_DIR = process.env.OUT_DIR || './recordings';
const CHROMIUM_PATH = process.env.CHROMIUM_PATH || undefined;
const VARIANT = process.env.VARIANT || 'master';
const FADE_MS = 260;
const LINKEDIN_FADE_MS = 220;

mkdirSync(OUT_DIR, { recursive: true });

function wait(ms) {
  return new Promise((resolve) => setTimeout(resolve, Math.max(0, ms)));
}

// Navigates, optionally waits for the scene's own "ready" signal (its
// entrance animation finishing), then sleeps exactly enough that this
// scene's total on-screen time -- measured from the moment navigation
// started -- equals targetMs, fading out over the last fadeMs of that
// budget. Wall-clock-based on purpose: waiting for sceneReady takes a
// variable amount of time (typing animations, staggered card reveals),
// and a fixed extra wait() on top of that silently overshoots the
// intended total duration -- confirmed by an early cut of this script
// running 36.6s against a 30s target for exactly this reason.
async function playScene(page, url, targetMs, { waitForReady = true, fade = true, fadeMs = FADE_MS } = {}) {
  const start = Date.now();
  await page.goto(url, { waitUntil: 'load' });
  if (waitForReady) {
    await page.waitForFunction(() => window.sceneReady === true, { timeout: targetMs + 2000 });
  }
  const appliedFadeMs = fade ? fadeMs : 0;
  const elapsed = Date.now() - start;
  await wait(targetMs - appliedFadeMs - elapsed);
  if (fade) {
    await page.evaluate(() => window.fadeOut && window.fadeOut());
    await wait(appliedFadeMs);
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

// Fades out whatever page is currently loaded, real or custom -- unlike
// playScene()'s fade (which calls a scene's own window.fadeOut()), this
// works on the real report.html too, which has no such hook of its own.
async function fadeOutCurrentPage(page, ms) {
  await page.evaluate((duration) => {
    document.body.style.transition = `opacity ${duration}ms ease`;
    document.body.style.opacity = '0';
  }, ms);
  await wait(ms);
}

async function finishRecording(context, page, outputName) {
  await page.close();
  await context.close();

  // Playwright names the video file "page@<guid>.webm" -- rename to
  // something predictable for the render script to pick up. Matched by
  // that naming pattern rather than "not already a known final name", so
  // a stale raw-capture.webm/linkedin-raw-capture.webm from a prior run
  // sitting in the same OUT_DIR can't be mistaken for the new recording.
  const files = readdirSync(OUT_DIR).filter((f) => f.startsWith('page@') && f.endsWith('.webm'));
  if (files.length === 1) {
    renameSync(join(OUT_DIR, files[0]), join(OUT_DIR, outputName));
    console.log(`Recording saved: ${join(OUT_DIR, outputName)}`);
  } else {
    console.log(`Recording(s) saved in ${OUT_DIR}:`, files);
  }
}

async function recordMaster(browser) {
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

  console.log(`Master timeline: ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  await finishRecording(context, page, 'raw-capture.webm');
}

// Standalone 15.8s LinkedIn teaser: unlike the master recording, this has
// to read correctly on its own with no post caption for context -- a
// dedicated opening title card and closing CTA card bracket the same
// terminal/findings/report beats (trimmed to fit), all real evidence,
// same as the master.
async function recordLinkedInTeaser(browser) {
  const context = await browser.newContext({
    viewport: { width: 1920, height: 1080 },
    recordVideo: { dir: OUT_DIR, size: { width: 1920, height: 1080 } }
  });
  const page = await context.newPage();
  const t0 = Date.now();

  // 0.0s - 1.3s: LinkedIn-specific opening title card
  await playScene(page, `${BASE_URL}/scenes/08-linkedin-title-open.html`, 1300, { fadeMs: LINKEDIN_FADE_MS });

  // 1.3s - 6.1s: terminal typing animation
  await playScene(page, `${BASE_URL}/scenes/02-terminal.html`, 4800, { fadeMs: LINKEDIN_FADE_MS });

  // 6.1s - 9.9s: key findings
  await playScene(page, `${BASE_URL}/scenes/03-findings.html`, 3800, { fadeMs: LINKEDIN_FADE_MS });

  // 9.9s - 11.3s: report-format overview card
  await playScene(page, `${BASE_URL}/scenes/04-reports-overview.html`, 1400, { fade: false });

  // 11.3s - 13.8s: the real, sanitized report.html (same cosmetic
  // cluster-name redaction as the master recording -- see recordMaster())
  {
    const start = Date.now();
    await page.goto(`${BASE_URL}/report.html`, { waitUntil: 'load' });
    await redactClusterName(page);
    await overlay(page, 'CLI · JSON · Markdown · HTML');
    await wait(2500 - (Date.now() - start) - LINKEDIN_FADE_MS);
    await fadeOutCurrentPage(page, LINKEDIN_FADE_MS);
  }

  // 13.8s - 15.8s: LinkedIn-specific closing CTA card
  await playScene(page, `${BASE_URL}/scenes/09-linkedin-title-close.html`, 2000, { waitForReady: false, fade: false });

  console.log(`LinkedIn teaser timeline: ${((Date.now() - t0) / 1000).toFixed(1)}s`);
  await finishRecording(context, page, 'linkedin-raw-capture.webm');
}

async function main() {
  const browser = await chromium.launch({ executablePath: CHROMIUM_PATH });
  if (VARIANT === 'linkedin') {
    await recordLinkedInTeaser(browser);
  } else {
    await recordMaster(browser);
  }
  await browser.close();
}

main().catch((err) => {
  console.error('RECORD_FAIL', err);
  process.exit(1);
});
