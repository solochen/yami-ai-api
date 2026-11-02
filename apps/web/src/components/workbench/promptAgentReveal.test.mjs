import test from "node:test";
import assert from "node:assert/strict";
import { revealStep, REVEAL_TICK_MS } from "./promptAgentReveal.ts";

test("空积压不吐字", () => {
  assert.equal(revealStep(0), 0);
  assert.equal(revealStep(-5), 0);
});

test("正常流式保持逐字观感", () => {
  assert.equal(revealStep(1), 1);
  assert.equal(revealStep(20), 1);
  assert.equal(revealStep(21), 2);
});

test("积压越多吐字越快", () => {
  const sizes = [10, 50, 150, 300, 1000, 2000, 5000];
  const steps = sizes.map(revealStep);
  for (let i = 1; i < steps.length; i += 1) {
    assert.ok(steps[i] >= steps[i - 1], `step 应单调不减: ${sizes[i - 1]}->${sizes[i]}`);
  }
});

test("大量积压在 2 秒左右追平", () => {
  for (const backlog of [1500, 3000, 6000]) {
    const seconds = (backlog / revealStep(backlog)) * (REVEAL_TICK_MS / 1000);
    assert.ok(seconds <= 2.1, `${backlog} 字需 ${seconds.toFixed(2)}s，超过 2.1s`);
  }
});

test("单次吐字有上限，避免一帧跳完", () => {
  assert.ok(revealStep(100000) <= 64);
});
