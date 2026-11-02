export const CATEGORIES = [
  { code: "all", label: "\u5168\u90e8", labelKey: "nav.all" },
  { code: "chat", label: "\u804a\u5929", labelKey: "nav.chat" },
  { code: "image", label: "\u56fe\u7247", labelKey: "nav.image" },
  { code: "video", label: "\u89c6\u9891", labelKey: "nav.video" },
  { code: "audio", label: "\u97f3\u9891", labelKey: "nav.audio" },
  { code: "mine", label: "\u6211\u7684", labelKey: "nav.mine" },
] as const;

export const CATEGORY_TAG: Record<string, { label: string; labelKey: string; className: string }> = {
  chat: { label: "\u804a\u5929", labelKey: "nav.chat", className: "bg-blue-50 text-blue-600" },
  multi_collab: { label: "\u591a\u6a21\u578b", labelKey: "category.multiCollab", className: "bg-indigo-50 text-indigo-600" },
  image: { label: "\u56fe\u7247", labelKey: "nav.image", className: "bg-primary/10 text-primary" },
  video: { label: "\u89c6\u9891", labelKey: "nav.video", className: "bg-purple-50 text-purple-600" },
  audio: { label: "\u97f3\u9891", labelKey: "nav.audio", className: "bg-orange-50 text-orange-600" },
};

export const MODEL_ICONS: Record<string, string> = {
  chat: "\u{1F4AC}",
  multi_collab: "\u{1F916}",
  image: "\u{1F5BC}\uFE0F",
  video: "\u{1F3AC}",
  audio: "\u{1F3B5}",
};

// 智能体按「任务」分类（原工具箱的分类法，合并后成为唯一一套）。
export const AGENT_CATEGORIES = [
  { code: "all", label: "\u5168\u90e8", labelKey: "nav.all", icon: "chip.all" },
  { code: "video", label: "\u89c6\u9891\u5904\u7406", labelKey: "agentCategory.video", icon: "chip.video" },
  { code: "image", label: "\u56fe\u7247\u5904\u7406", labelKey: "agentCategory.image", icon: "chip.image" },
  { code: "content", label: "\u5185\u5bb9\u521b\u4f5c", labelKey: "agentCategory.content", icon: "chip.content" },
  { code: "ai_create", label: "AI \u521b\u4f5c", labelKey: "agentCategory.ai_create", icon: "chip.aiCreate" },
  { code: "assistant", label: "AI \u52a9\u624b", labelKey: "agentCategory.assistant", icon: "chip.assistant" },
  { code: "mine", label: "\u6211\u7684", labelKey: "nav.mine", icon: "chip.mine" },
] as const;

export const AGENT_CATEGORY_TAG: Record<string, { label: string; labelKey: string; className: string }> = {
  image: { label: "\u56fe\u7247\u5904\u7406", labelKey: "agentCategory.image", className: "bg-primary/10 text-primary" },
  video: { label: "\u89c6\u9891\u5904\u7406", labelKey: "agentCategory.video", className: "bg-purple-50 text-purple-600" },
  content: { label: "\u5185\u5bb9\u521b\u4f5c", labelKey: "agentCategory.content", className: "bg-indigo-50 text-indigo-600" },
  ai_create: { label: "AI \u521b\u4f5c", labelKey: "agentCategory.ai_create", className: "bg-rose-50 text-rose-600" },
  assistant: { label: "AI \u52a9\u624b", labelKey: "agentCategory.assistant", className: "bg-amber-50 text-amber-600" },
  multi_collab: { label: "\u591a\u6a21\u578b", labelKey: "category.multiCollab", className: "bg-indigo-50 text-indigo-600" },
  api: { label: "API", labelKey: "category.api", className: "bg-sky-50 text-sky-600" },
  tool: { label: "\u5de5\u5177", labelKey: "category.tool", className: "bg-amber-50 text-amber-600" },
};

const BROWSE_CATEGORIES = new Set(["video", "image", "content", "ai_create", "assistant"]);

export type AgentBrowseMeta = {
  category?: string;
  subtitle?: string;
  tags?: string[];
  badges?: string[];
  billing?: string;
};

// 智能体的展示配置统一放在 display_config.toolbox（历史键名，沿用以免改动后台表单）。
export function agentBrowseMeta(agent: { display_config?: unknown }): AgentBrowseMeta | null {
  const raw = (agent?.display_config as { toolbox?: AgentBrowseMeta & { enabled?: boolean } } | undefined)?.toolbox;
  if (!raw || raw.enabled === false) return null;
  return raw;
}

export function agentDisplayCategory(
  code: string,
  category?: string,
  displayConfig?: unknown,
) {
  const browse = agentBrowseMeta({ display_config: displayConfig });
  const browseCategory = String(browse?.category || "").trim();
  if (BROWSE_CATEGORIES.has(browseCategory)) return browseCategory;
  if (["infinite_canvas", "content_image_post", "video_creation", "video_creation_v2"].includes(code)) return "assistant";
  if (category === "image") return "image";
  if (category === "video") return "video";
  return "assistant";
}

// Hero gradient themes for the agent workspace banner.
export const AGENT_THEMES: Record<string, { gradient: string; iconBg: string; pill: string; accent: string }> = {
  amber: {
    gradient: "from-amber-50 via-orange-50 to-white",
    iconBg: "bg-amber-100 text-amber-600",
    pill: "bg-amber-100/70 text-amber-700",
    accent: "text-amber-600",
  },
  rose: {
    gradient: "from-rose-50 via-pink-50 to-white",
    iconBg: "bg-rose-100 text-rose-600",
    pill: "bg-rose-100/70 text-rose-700",
    accent: "text-rose-600",
  },
  violet: {
    gradient: "from-violet-50 via-purple-50 to-white",
    iconBg: "bg-violet-100 text-violet-600",
    pill: "bg-violet-100/70 text-violet-700",
    accent: "text-violet-600",
  },
  sky: {
    gradient: "from-sky-50 via-blue-50 to-white",
    iconBg: "bg-sky-100 text-sky-600",
    pill: "bg-sky-100/70 text-sky-700",
    accent: "text-sky-600",
  },
  emerald: {
    gradient: "from-orange-50 via-amber-50 to-white",
    iconBg: "bg-primary/10 text-primary",
    pill: "bg-primary/10 text-primary",
    accent: "text-primary",
  },
  fuchsia: {
    gradient: "from-fuchsia-50 via-pink-50 to-white",
    iconBg: "bg-fuchsia-100 text-fuchsia-600",
    pill: "bg-fuchsia-100/70 text-fuchsia-700",
    accent: "text-fuchsia-600",
  },
  comic: {
    gradient: "from-cyan-50 via-violet-50 to-white",
    iconBg: "bg-cyan-100 text-cyan-700",
    pill: "bg-cyan-100/70 text-cyan-700",
    accent: "text-cyan-600",
  },
};

export const FEATURE_CARDS = [
  {
    titleKey: "workspace.feature.multiView.title",
    descKey: "workspace.feature.multiView.desc",
    icon: "\u{1F310}",
    color: "bg-amber-50 text-amber-600",
  },
  {
    titleKey: "workspace.feature.fusion.title",
    descKey: "workspace.feature.fusion.desc",
    icon: "\u2728",
    color: "bg-purple-50 text-purple-600",
  },
  {
    titleKey: "workspace.feature.parallel.title",
    descKey: "workspace.feature.parallel.desc",
    icon: "\u26A1",
    color: "bg-blue-50 text-blue-600",
  },
  {
    titleKey: "workspace.feature.quality.title",
    descKey: "workspace.feature.quality.desc",
    icon: "\u{1F6E1}\uFE0F",
    color: "bg-pink-50 text-pink-600",
  },
];
