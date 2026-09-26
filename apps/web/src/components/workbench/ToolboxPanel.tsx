"use client";

import { useMemo } from "react";
import { useRouter } from "next/navigation";
import { clsx } from "clsx";
import { AgentIcon } from "./AgentIcon";
import { useI18n } from "@/i18n/I18nProvider";

export const TOOLBOX_CATEGORIES = [
  { code: "all", label: "全部" },
  { code: "video", label: "视频处理" },
  { code: "content", label: "内容创作" },
  { code: "ai_create", label: "AI 创作" },
  { code: "assistant", label: "AI 助手" },
  { code: "document", label: "文档处理" },
  { code: "image", label: "图片处理" },
  { code: "text", label: "文字处理" },
] as const;

export type ToolboxCategory = (typeof TOOLBOX_CATEGORIES)[number]["code"];

type ToolboxConfig = {
  enabled?: boolean;
  category?: string;
  subtitle?: string;
  tags?: string[];
  badges?: string[];
  billing?: string;
};

export type ToolboxAgent = {
  code: string;
  name: string;
  description?: string;
  icon?: string;
  is_enabled?: boolean;
  price_rule?: { billing_type?: string; unit_price?: number };
  display_config?: { toolbox?: ToolboxConfig; feature_tags?: string[] };
};

const BADGE_LABEL: Record<string, string> = {
  available: "可用",
  recommended: "推荐",
  before_create: "创作前",
  migrating: "迁移中",
};

export function toolboxConfig(agent: ToolboxAgent): ToolboxConfig | null {
  const toolbox = agent.display_config?.toolbox;
  if (!toolbox?.enabled || agent.is_enabled === false) return null;
  return toolbox;
}

export function ToolboxFilters({
  agents,
  category,
  query,
  onCategory,
  onQuery,
}: {
  agents: ToolboxAgent[];
  category: ToolboxCategory;
  query: string;
  onCategory: (code: ToolboxCategory) => void;
  onQuery: (value: string) => void;
}) {
  const { ts } = useI18n();
  const listed = agents.filter((agent) => toolboxConfig(agent));
  const counts = useMemo(() => {
    const map = new Map<string, number>();
    for (const agent of listed) {
      const code = toolboxConfig(agent)?.category || "";
      map.set(code, (map.get(code) || 0) + 1);
    }
    return map;
  }, [listed]);

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div className="px-3 pb-2 pt-1">
        <div className="text-[11px] font-medium uppercase tracking-wide text-gray-400">Filters</div>
        <div className="mt-1 flex items-end justify-between">
          <h2 className="text-base font-semibold text-gray-900 dark:text-gray-100">{ts("工具筛选")}</h2>
          <span className="text-xs tabular-nums text-gray-400">{listed.length} / {listed.length}</span>
        </div>
      </div>
      <div className="px-3">
        <input
          value={query}
          onChange={(event) => onQuery(event.target.value)}
          placeholder={ts("搜索工具、分类或标签")}
          className="h-9 w-full rounded-xl border border-gray-200 bg-gray-50 px-3 text-xs outline-none focus:border-primary dark:border-white/10 dark:bg-white/5 dark:text-gray-100"
        />
      </div>
      <div className="mt-3 flex-1 space-y-1 overflow-y-auto px-2">
        {TOOLBOX_CATEGORIES.map((item) => {
          const count = item.code === "all" ? listed.length : counts.get(item.code) || 0;
          const active = category === item.code;
          return (
            <button
              key={item.code}
              type="button"
              onClick={() => onCategory(item.code)}
              className={clsx(
                "flex w-full items-center justify-between rounded-xl px-3 py-2 text-left text-sm transition",
                active ? "bg-primary font-medium text-dark" : "text-gray-600 hover:bg-gray-50 dark:text-gray-300 dark:hover:bg-white/5"
              )}
            >
              <span>{ts(item.label)}</span>
              <span className={clsx("text-xs tabular-nums", active ? "text-dark/70" : "text-gray-400")}>{count}</span>
            </button>
          );
        })}
      </div>
      <div className="space-y-2 border-t border-gray-100 px-3 py-3 text-xs text-gray-500 dark:border-white/10">
        <button type="button" onClick={() => { onCategory("all"); onQuery(""); }} className="font-medium text-primary">{ts("清除筛选")}</button>
        <div className="grid grid-cols-2 gap-2">
          <div className="rounded-xl border border-gray-100 px-3 py-2 dark:border-white/10">
            <div>{ts("已接入")}</div>
            <div className="text-lg font-semibold text-gray-900 dark:text-gray-100">{listed.length}</div>
          </div>
          <div className="rounded-xl border border-gray-100 px-3 py-2 dark:border-white/10">
            <div>{ts("预留入口")}</div>
            <div className="text-lg font-semibold text-gray-900 dark:text-gray-100">0</div>
          </div>
        </div>
      </div>
    </div>
  );
}

export function ToolboxPanel({ agents, category, query }: { agents: ToolboxAgent[]; category: ToolboxCategory; query: string }) {
  const { ts, td } = useI18n();
  const router = useRouter();
  const keyword = query.trim().toLowerCase();
  const cards = agents.filter((agent) => {
    const toolbox = toolboxConfig(agent);
    if (!toolbox) return false;
    if (category !== "all" && toolbox.category !== category) return false;
    if (!keyword) return true;
    const tags = [...(toolbox.tags || []), ...(agent.display_config?.feature_tags || [])].join(" ");
    return `${agent.name} ${agent.description || ""} ${toolbox.subtitle || ""} ${tags}`.toLowerCase().includes(keyword);
  });

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto px-4 py-5 sm:px-6">
      <div className="text-[11px] font-medium uppercase tracking-wide text-gray-400">Workbench</div>
      <h1 className="mt-1 text-xl font-semibold text-gray-900 dark:text-gray-100">{ts("已接入的工具工作台")}</h1>
      <p className="mt-1 text-xs text-gray-400">{ts("点击卡片进入独立工具页")}</p>
      {cards.length === 0 ? (
        <div className="flex flex-1 items-center justify-center text-sm text-gray-400">{ts("这个分类还没有接入工具")}</div>
      ) : (
        <div className="mt-4 grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
          {cards.map((agent) => {
            const toolbox = toolboxConfig(agent)!;
            const categoryLabel = TOOLBOX_CATEGORIES.find((item) => item.code === toolbox.category)?.label || toolbox.category || "";
            const badges = toolbox.badges?.length ? toolbox.badges : ["available"];
            const migrating = badges.includes("migrating") && !badges.includes("available");
            const tags = toolbox.tags?.length ? toolbox.tags : agent.display_config?.feature_tags || [];
            const billing = toolbox.billing || (agent.price_rule?.billing_type === "per_request" && agent.price_rule.unit_price != null ? `${agent.price_rule.unit_price} 算力/次` : ts("按实际消耗"));
            const name = td(`agent.${agent.code}.name`, agent.name);
            return (
              <article key={agent.code} className="soft-card flex min-h-[220px] flex-col p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="flex h-10 w-10 items-center justify-center overflow-hidden rounded-xl bg-gray-900 text-lg text-white">
                    <AgentIcon value={agent.icon} alt={name} />
                  </div>
                  <div className="flex flex-wrap justify-end gap-1">
                    {badges.map((badge) => (
                      <span key={badge} className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] text-gray-600 dark:bg-white/10 dark:text-gray-300">{ts(BADGE_LABEL[badge] || badge)}</span>
                    ))}
                  </div>
                </div>
                <div className="mt-3 text-[11px] text-primary">{ts(categoryLabel)}</div>
                <h2 className="mt-1 text-base font-semibold text-gray-900 dark:text-gray-100">{name}</h2>
                <p className="mt-1 line-clamp-2 text-xs leading-5 text-gray-500 dark:text-gray-400">{toolbox.subtitle || td(`agent.${agent.code}.description`, agent.description || "")}</p>
                {tags.length > 0 && (
                  <div className="mt-3 flex flex-wrap gap-1.5">
                    {tags.slice(0, 4).map((tag) => (
                      <span key={tag} className="rounded-full border border-gray-200 px-2 py-0.5 text-[10px] text-gray-500 dark:border-white/10 dark:text-gray-400">{tag}</span>
                    ))}
                  </div>
                )}
                <div className="mt-auto flex items-end justify-between gap-3 pt-4">
                  <div>
                    <div className="text-xs text-gray-700 dark:text-gray-200">{billing}</div>
                    <div className="text-[10px] text-gray-400">{ts("独立工具工作台")}</div>
                  </div>
                  <button
                    type="button"
                    disabled={migrating}
                    onClick={() => router.push(`/app/agents/${encodeURIComponent(agent.code)}`)}
                    className="text-sm font-medium text-primary disabled:cursor-not-allowed disabled:text-gray-400"
                  >
                    {migrating ? ts("迁移中") : ts("打开")} →
                  </button>
                </div>
              </article>
            );
          })}
        </div>
      )}
    </div>
  );
}
