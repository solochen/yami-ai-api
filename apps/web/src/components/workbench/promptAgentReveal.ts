// 逐字揭示：上游可能把整段回复攒完才一次性下发（部分中转线路如此），
// 这里把到达的文本放进缓冲队列，按稳定速率吐字，保证任何上游都有"打字机"观感。
export const REVEAL_TICK_MS = 20;

// 积压越多，单次吐字越多；正常流式时积压很小，保持逐字观感。
export function revealStep(backlog: number): number {
  if (backlog <= 0) return 0;
  if (backlog <= 20) return 1;
  if (backlog <= 60) return 2;
  if (backlog <= 160) return 4;
  if (backlog <= 400) return 8;
  if (backlog <= 1200) return 16;
  return Math.min(64, Math.ceil(backlog / 40));
}
