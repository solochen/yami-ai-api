import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";

// 展示层品牌隔离：角色替换页面的文案与接口字段不能出现上游品牌、产品名或内部协议字段。
// 运行时适配层（lib/realtime/renderStage.ts）只做内部绑定，不参与用户可见文案扫描。
const FILES = [
  "apps/web/src/components/workbench/CharacterStudioWorkspace.tsx",
  "apps/web/src/lib/characterStudio.ts",
  "apps/web/src/i18n/source-locales/supplements/en-US/characterStudio.ts",
];

const FORBIDDEN = [
  "vidu",
  "aliyun",
  "alirtc",
  "artc",
  "trtc",
  "agora",
  "volcengine",
  "s2-editing",
  "s2_editing",
  "s_editing",
  "subject_replacement",
  "live_id",
  "render_uid",
  "switch_prompt",
  "conn_init",
  "client_secret",
];

test("character studio display layer stays vendor-neutral", () => {
  for (const relative of FILES) {
    const file = path.resolve(relative);
    const text = fs.readFileSync(file, "utf8");
    const lower = text.toLowerCase();
    for (const term of FORBIDDEN) {
      const matcher = new RegExp(`(^|[^a-z0-9_])${term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}([^a-z0-9_]|$)`, "i");
      assert.equal(matcher.test(lower), false, `${relative} exposes forbidden term: ${term}`);
    }
  }
});

test("character studio page keeps the open-pipeline narrative", () => {
  const page = fs.readFileSync(path.resolve(FILES[0]), "utf8");
  assert.match(page, /OPEN REALTIME PIPELINE/);
  assert.match(page, /开放视频模型内核/);
  assert.match(page, /项目代码开源/);
});
