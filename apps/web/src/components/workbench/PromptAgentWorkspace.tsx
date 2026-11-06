"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";
import {
  Brain,
  Check,
  Copy,
  Download,
  History,
  Loader2,
  Pencil,
  Plus,
  RefreshCw,
  Send,
  Sparkles,
  Square,
  Trash2,
  X,
} from "lucide-react";
import { API_URL, api, legacyAuthHeaders } from "@/lib/api";
import { readEventStream } from "@/lib/eventStream";
import { useI18n } from "@/i18n/I18nProvider";
import { AgentIcon } from "./AgentIcon";
import { AgentRichText } from "./AgentRichText";
import { REVEAL_TICK_MS, revealStep } from "./promptAgentReveal";

type WorkflowLike = {
  code?: string;
  name?: string;
  description?: string;
  icon?: string;
  display_config?: {
    theme?: string;
    hero_tags?: string[];
    toolbox?: { subtitle?: string; tags?: string[]; billing?: string };
  };
  runtime_config?: {
    agent_mode?: string;
    chat_model_code?: string;
    greeting?: string;
    quick_prompts?: string[];
    input_placeholder?: string;
    disclaimer?: string;
    max_input_chars?: number;
  };
};

type Message = {
  id?: number;
  role: "user" | "assistant";
  content: string;
  thinking?: string;
  stopped?: boolean;
};

type HistoryItem = { public_id: string; title: string; updated_at: string; message_count: number };

type ConversationPayload = {
  public_id: string;
  title: string;
  messages: Array<{ id: number; role: string; content: string; created_at: string }>;
};

const DEFAULT_MAX_INPUT = 4000;

function copyToClipboard(text: string) {
  if (!text) return;
  if (navigator.clipboard?.writeText) {
    void navigator.clipboard.writeText(text).catch(() => undefined);
    return;
  }
  const area = document.createElement("textarea");
  area.value = text;
  area.style.position = "fixed";
  area.style.opacity = "0";
  document.body.appendChild(area);
  area.select();
  document.execCommand("copy");
  document.body.removeChild(area);
}

function downloadMarkdown(name: string, messages: Message[]) {
  const body = messages
    .map((m) => `## ${m.role === "user" ? "我" : name}\n\n${m.content}`)
    .join("\n\n---\n\n");
  const blob = new Blob([body], { type: "text/markdown;charset=utf-8" });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement("a");
  anchor.href = url;
  anchor.download = `${name}-${new Date().toISOString().slice(0, 10)}.md`;
  anchor.click();
  URL.revokeObjectURL(url);
}

function MessageAction({
  label,
  onClick,
  icon,
  danger,
}: {
  label: string;
  onClick: () => void;
  icon: ReactNode;
  danger?: boolean;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      title={label}
      aria-label={label}
      className={`inline-flex h-7 items-center gap-1 rounded-lg px-1.5 text-[11px] transition ${
        danger
          ? "text-gray-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-500/10"
          : "text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/10 dark:hover:text-gray-200"
      }`}
    >
      {icon}
    </button>
  );
}

// 思考过程默认折叠：部分模型会把整篇草稿当思考吐出来，展开会淹没正文。
// 折叠态显示字数和进行中状态，足以让用户知道"它还在工作"。
function ThinkingBlock({ text, streaming }: { text: string; streaming: boolean }) {
  const { ts } = useI18n();
  const [open, setOpen] = useState(false);
  return (
    <div className="max-w-[95%] rounded-2xl border border-dashed border-gray-200 bg-gray-50/70 px-3 py-2 dark:border-white/10 dark:bg-white/[0.03]">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        aria-expanded={open}
        className="flex w-full items-center gap-1.5 text-[11px] font-medium text-gray-500 transition hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-200"
      >
        <Brain size={12} className="text-primary" />
        <span>{ts("思考过程")}</span>
        {text ? <span className="text-gray-400">· {text.length} {ts("字")}</span> : null}
        {streaming ? <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-primary" aria-hidden /> : null}
        <span className="text-gray-400">{open ? "▾" : "▸"}</span>
      </button>
      {open ? (
        <div className="mt-1.5 max-h-52 overflow-auto whitespace-pre-wrap text-[12px] leading-relaxed text-gray-500 dark:text-gray-400">
          {text}
        </div>
      ) : null}
    </div>
  );
}

export function PromptAgentWorkspace({ workflow }: { workflow: WorkflowLike }) {
  const { t, ts } = useI18n();
  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [stage, setStage] = useState("");
  const [conversationId, setConversationId] = useState("");
  const [conversationTitle, setConversationTitle] = useState("");
  const [error, setError] = useState("");
  const [needRecharge, setNeedRecharge] = useState(false);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [showHistory, setShowHistory] = useState(false);
  const [historyBusy, setHistoryBusy] = useState(false);
  const [renamingId, setRenamingId] = useState("");
  const [renameDraft, setRenameDraft] = useState("");
  const [editingIndex, setEditingIndex] = useState<number | null>(null);
  const [editDraft, setEditDraft] = useState("");
  const [copiedIndex, setCopiedIndex] = useState<number | null>(null);
  const [waited, setWaited] = useState(0);

  const messagesEndRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const abortRef = useRef<AbortController | null>(null);
  // 打字机缓冲：pending 待揭示、received 已收到的全文、finalize 等揭示完成后执行。
  const revealRef = useRef<{
    pending: string;
    received: string;
    timer: ReturnType<typeof setInterval> | null;
    done: boolean;
    finalize: (() => void) | null;
  }>({ pending: "", received: "", timer: null, done: false, finalize: null });

  const agentCode = workflow.code || "";
  const agentName = workflow.name || ts("智能体");
  const greeting = (workflow.runtime_config?.greeting as string | undefined)?.trim() || "";
  const subtitle = (workflow.display_config?.toolbox?.subtitle as string | undefined)?.trim() || workflow.description || "";
  const placeholder = (workflow.runtime_config?.input_placeholder as string | undefined)?.trim() || ts("输入你的需求，越具体越好");
  const disclaimer = (workflow.runtime_config?.disclaimer as string | undefined)?.trim() || "";
  const chatModelCode = (workflow.runtime_config?.chat_model_code as string | undefined)?.trim() || "";
  const billingLabel = (workflow.display_config?.toolbox?.billing as string | undefined)?.trim() || t("workspace.tokenBilling");
  const maxInput = Number(workflow.runtime_config?.max_input_chars || DEFAULT_MAX_INPUT) || DEFAULT_MAX_INPUT;
  const quickPrompts = useMemo(() => {
    const raw = workflow.runtime_config?.quick_prompts;
    return Array.isArray(raw) ? raw.filter((item): item is string => typeof item === "string" && item.trim() !== "") : [];
  }, [workflow.runtime_config?.quick_prompts]);
  const heroTags = useMemo(() => {
    const raw = workflow.display_config?.hero_tags;
    return Array.isArray(raw) ? raw.filter((item): item is string => typeof item === "string" && item.trim() !== "") : [];
  }, [workflow.display_config?.hero_tags]);

  const loadHistory = useCallback(async () => {
    if (!agentCode) return;
    setHistoryBusy(true);
    try {
      const data = await api<{ items?: HistoryItem[] }>(`/api/agents/${encodeURIComponent(agentCode)}/conversations`);
      setHistory(Array.isArray(data?.items) ? data.items : []);
    } catch {
      setHistory([]);
    } finally {
      setHistoryBusy(false);
    }
  }, [agentCode]);

  useEffect(() => {
    void loadHistory();
  }, [loadHistory]);

  useEffect(() => {
    if (stickRef.current) messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages, stage]);

  const resetChat = useCallback(() => {
    abortRef.current?.abort();
    setMessages([]);
    setConversationId("");
    setConversationTitle("");
    setInput("");
    setError("");
    setNeedRecharge(false);
    setStage("");
    setEditingIndex(null);
    setShowHistory(false);
  }, []);

  const openConversation = useCallback(
    async (id: string) => {
      abortRef.current?.abort();
      setHistoryBusy(true);
      try {
        const data = await api<ConversationPayload>(`/api/agents/${encodeURIComponent(agentCode)}/conversations/${id}`);
        setMessages(
          (data.messages || [])
            .filter((m) => m.role === "user" || m.role === "assistant")
            .map((m) => ({ id: m.id, role: m.role as "user" | "assistant", content: m.content })),
        );
        setConversationId(id);
        setConversationTitle(data.title || "");
        setError("");
        setNeedRecharge(false);
        setEditingIndex(null);
        setShowHistory(false);
        stickRef.current = true;
      } catch (e) {
        setError(e instanceof Error ? e.message : ts("会话加载失败"));
      } finally {
        setHistoryBusy(false);
      }
    },
    [agentCode, ts],
  );

  const patchLastAssistant = useCallback((patch: (m: Message) => Message) => {
    setMessages((prev) => {
      if (prev.length === 0) return prev;
      const index = prev.length - 1;
      if (prev[index].role !== "assistant") return prev;
      const next = [...prev];
      next[index] = patch(next[index]);
      return next;
    });
  }, []);

  // 打字机：把上游推来的文本按稳定速率显示，突发到达的整段也会逐字出现。
  const startReveal = useCallback(() => {
    const state = revealRef.current;
    if (state.timer) return;
    state.timer = setInterval(() => {
      const current = revealRef.current;
      if (!current.pending) {
        // 没有积压就停表，避免中断后留下空转的定时器。
        if (current.timer) clearInterval(current.timer);
        current.timer = null;
        if (current.done) {
          current.done = false;
          const finalize = current.finalize;
          current.finalize = null;
          finalize?.();
        }
        return;
      }
      const take = current.pending.slice(0, revealStep(current.pending.length));
      current.pending = current.pending.slice(take.length);
      patchLastAssistant((m) => ({ ...m, content: m.content + take }));
    }, REVEAL_TICK_MS);
  }, [patchLastAssistant]);

  const resetReveal = useCallback(() => {
    const state = revealRef.current;
    if (state.timer) clearInterval(state.timer);
    state.pending = "";
    state.received = "";
    state.timer = null;
    state.done = false;
    state.finalize = null;
  }, []);

  const pushRevealDelta = useCallback((text: string) => {
    const state = revealRef.current;
    state.received += text;
    state.pending += text;
    startReveal();
  }, [startReveal]);

  // 先把积压一次性显示完（停止/失败时用），不丢已收到的内容。
  const flushReveal = useCallback(() => {
    const state = revealRef.current;
    if (state.timer) clearInterval(state.timer);
    state.timer = null;
    if (state.pending) {
      const rest = state.pending;
      state.pending = "";
      patchLastAssistant((m) => ({ ...m, content: m.content + rest }));
    }
    state.done = false;
    state.finalize = null;
  }, [patchLastAssistant]);

  const finishReveal = useCallback((finalize: () => void) => {
    const state = revealRef.current;
    state.done = true;
    state.finalize = finalize;
    if (!state.pending) {
      if (state.timer) clearInterval(state.timer);
      state.timer = null;
      state.done = false;
      state.finalize = null;
      finalize();
      return;
    }
    startReveal();
  }, [startReveal]);

  useEffect(() => () => {
    if (revealRef.current.timer) clearInterval(revealRef.current.timer);
    revealRef.current.timer = null;
  }, []);

  // 等待首字时按秒计时，配合后端 ping 给出"还在工作"的反馈。
  useEffect(() => {
    if (!busy) return;
    const timer = setInterval(() => setWaited((value) => value + 1), 1000);
    return () => clearInterval(timer);
  }, [busy]);

  // 统一流式请求：/chat 与 /rerun 共用同一套事件协议。
  // delta 走打字机缓冲，thinking/ping 直接交给调用方处理。
  const streamRequest = useCallback(
    async (
      path: string,
      body: Record<string, unknown>,
      handlers: { onDelta: (text: string) => void; onThinking?: (text: string) => void; onPing?: (seconds: number) => void },
    ): Promise<{ conversationId: string; messageId?: number; userMessageId?: number; content: string; received: string }> => {
      const controller = new AbortController();
      abortRef.current = controller;

      const locale = typeof window === "undefined" ? "zh-CN" : localStorage.getItem("site_locale") || "zh-CN";
      const result = { conversationId, messageId: undefined as number | undefined, userMessageId: undefined as number | undefined, content: "", received: "" };
      try {
        const response = await fetch(`${API_URL}${path}`, {
          method: "POST",
          credentials: "include",
          signal: controller.signal,
          headers: {
            "Content-Type": "application/json",
            "Accept-Language": locale,
            "X-Locale": locale,
            ...legacyAuthHeaders(),
          },
          body: JSON.stringify(body),
        });
        const contentType = response.headers.get("content-type") || "";
        if (response.status === 401) throw new Error(ts("登录已过期，请重新登录"));
        if (!response.ok || !contentType.includes("text/event-stream")) {
          const payload = await response.json().catch(() => null);
          throw new Error(payload?.message || `请求失败（HTTP ${response.status}）`);
        }
        if (!response.body) throw new Error(ts("当前浏览器不支持流式输出"));

        for await (const { event, data: raw } of readEventStream(response.body)) {
          if (raw === "[DONE]") continue;
          let data: Record<string, unknown>;
          try {
            data = JSON.parse(raw) as Record<string, unknown>;
          } catch {
            continue;
          }
          if (event === "meta") {
            if (typeof data.conversation_id === "string" && data.conversation_id) {
              result.conversationId = data.conversation_id;
              setConversationId(data.conversation_id);
            }
          } else if (event === "delta") {
            const text = String(data.text || "");
            if (text) {
              result.received += text;
              handlers.onDelta(text);
            }
          } else if (event === "thinking") {
            const text = String(data.text || "");
            if (text) handlers.onThinking?.(text);
          } else if (event === "ping") {
            const seconds = Number(data.elapsed_seconds || 0);
            if (seconds > 0) handlers.onPing?.(seconds);
          } else if (event === "error") {
            if (String(data.code || "") === "INSUFFICIENT_BALANCE") setNeedRecharge(true);
            throw new Error(String(data.message || ts("生成失败")));
          } else if (event === "done") {
            if (typeof data.conversation_id === "string" && data.conversation_id) {
              result.conversationId = data.conversation_id;
              setConversationId(data.conversation_id);
            }
            if (typeof data.message_id === "number") result.messageId = data.message_id;
            if (typeof data.user_message_id === "number") result.userMessageId = data.user_message_id;
            if (typeof data.content === "string" && data.content) result.content = data.content;
          }
        }
        return result;
      } finally {
        abortRef.current = null;
      }
    },
    [conversationId, ts],
  );

  const send = useCallback(
    async (text?: string, options?: { replaceFrom?: number }) => {
      const message = (text ?? input).trim();
      if (!message || busy) return;
      if (Array.from(message).length > maxInput) {
        setError(ts("消息过长，请精简后再发送"));
        return;
      }
      setInput("");
      setError("");
      setNeedRecharge(false);
      setBusy(true);
      setStage(ts("正在思考…"));
      setWaited(0);
      stickRef.current = true;
      setEditingIndex(null);
      resetReveal();

      const base = options?.replaceFrom !== undefined ? messages.slice(0, options.replaceFrom) : messages;
      setMessages([...base, { role: "user", content: message }, { role: "assistant", content: "" }]);

      try {
        const result = await streamRequest(
          `/api/agents/${encodeURIComponent(agentCode)}/chat`,
          { conversation_id: conversationId || undefined, message },
          {
            onDelta: pushRevealDelta,
            onThinking: (chunk) => patchLastAssistant((m) => ({ ...m, thinking: (m.thinking || "") + chunk })),
            onPing: (seconds) => setWaited((value) => Math.max(value, seconds)),
          },
        );
        finishReveal(() => {
          setMessages((prev) => {
            const next = [...prev];
            if (next.length >= 2) {
              next[next.length - 2] = { ...next[next.length - 2], id: result.userMessageId };
              next[next.length - 1] = { ...next[next.length - 1], id: result.messageId, content: result.content || next[next.length - 1].content };
            }
            return next;
          });
          if (!conversationTitle) setConversationTitle(message.slice(0, 24));
          setBusy(false);
          setStage("");
          void loadHistory();
        });
      } catch (e) {
        flushReveal();
        if (e instanceof DOMException && e.name === "AbortError") {
          // 用户主动停止：保留已生成的部分，并标记为已停止。
          setMessages((prev) => {
            const next = [...prev];
            const last = next[next.length - 1];
            if (last?.role === "assistant") next[next.length - 1] = { ...last, stopped: true };
            return next;
          });
          setBusy(false);
          setStage("");
          return;
        }
        // 失败：撤回本轮乐观消息，把内容放回输入框，方便直接重试。
        setMessages(base);
        setInput(message);
        setError(e instanceof Error ? e.message : ts("生成失败"));
        setBusy(false);
        setStage("");
      }
    },
    [agentCode, busy, conversationId, conversationTitle, finishReveal, flushReveal, input, loadHistory, maxInput, messages, patchLastAssistant, pushRevealDelta, resetReveal, streamRequest, ts],
  );

  // 重新生成 / 编辑并重跑：已落库走 rerun（服务端截断后续轮次），未落库直接重发。
  const rerunFrom = useCallback(
    async (userIndex: number, newContent?: string) => {
      if (busy) return;
      const source = messages[userIndex];
      if (!source || source.role !== "user") return;
      const content = (newContent ?? source.content).trim();
      if (!content) return;
      if (Array.from(content).length > maxInput) {
        setError(ts("消息过长，请精简后再发送"));
        return;
      }

      if (!source.id || !conversationId) {
        await send(content, { replaceFrom: userIndex });
        return;
      }
      setError("");
      setNeedRecharge(false);
      setBusy(true);
      setStage(ts("正在重新生成…"));
      setWaited(0);
      setEditingIndex(null);
      stickRef.current = true;
      resetReveal();
      setMessages((prev) => [...prev.slice(0, userIndex), { ...source, content }, { role: "assistant", content: "" }]);

      try {
        const result = await streamRequest(
          `/api/agents/${encodeURIComponent(agentCode)}/conversations/${conversationId}/messages/${source.id}/rerun`,
          newContent !== undefined ? { content } : {},
          {
            onDelta: pushRevealDelta,
            onThinking: (chunk) => patchLastAssistant((m) => ({ ...m, thinking: (m.thinking || "") + chunk })),
            onPing: (seconds) => setWaited((value) => Math.max(value, seconds)),
          },
        );
        finishReveal(() => {
          setMessages((prev) => {
            const next = [...prev];
            if (next.length >= 2) {
              next[next.length - 1] = {
                ...next[next.length - 1],
                id: result.messageId,
                content: result.content || next[next.length - 1].content,
              };
            }
            return next;
          });
          setBusy(false);
          setStage("");
          void loadHistory();
        });
      } catch (e) {
        flushReveal();
        if (e instanceof DOMException && e.name === "AbortError") {
          // 重新生成被停止：服务端已截断旧回复，这里移除空白气泡保持界面一致。
          setMessages((prev) => prev.filter((m, i) => !(i === prev.length - 1 && m.role === "assistant" && !m.content)));
          setBusy(false);
          setStage("");
          return;
        }
        setMessages(messages);
        setError(e instanceof Error ? e.message : ts("重新生成失败"));
        setBusy(false);
        setStage("");
      }
    },
    [agentCode, busy, conversationId, finishReveal, flushReveal, loadHistory, maxInput, messages, patchLastAssistant, pushRevealDelta, resetReveal, send, streamRequest, ts],
  );

  const deleteMessage = useCallback(
    async (index: number) => {
      if (busy) return;
      const message = messages[index];
      if (!message) return;
      if (!message.id || !conversationId) {
        setMessages((prev) => prev.filter((_, i) => i !== index));
        return;
      }
      try {
        await api(`/api/agents/${encodeURIComponent(agentCode)}/conversations/${conversationId}/messages/${message.id}`, {
          method: "DELETE",
        });
        setMessages((prev) => prev.filter((_, i) => i !== index));
      } catch (e) {
        setError(e instanceof Error ? e.message : ts("删除失败"));
      }
    },
    [agentCode, busy, conversationId, messages, ts],
  );

  const removeConversation = useCallback(
    async (id: string) => {
      try {
        await api(`/api/agents/${encodeURIComponent(agentCode)}/conversations/${id}`, { method: "DELETE" });
        setHistory((prev) => prev.filter((item) => item.public_id !== id));
        if (conversationId === id) resetChat();
      } catch (e) {
        setError(e instanceof Error ? e.message : ts("删除会话失败"));
      }
    },
    [agentCode, conversationId, resetChat, ts],
  );

  const submitRename = useCallback(
    async (id: string) => {
      const title = renameDraft.trim();
      if (!title) return;
      try {
        await api(`/api/agents/${encodeURIComponent(agentCode)}/conversations/${id}`, {
          method: "PATCH",
          body: JSON.stringify({ title }),
        });
        setHistory((prev) => prev.map((item) => (item.public_id === id ? { ...item, title } : item)));
        if (conversationId === id) setConversationTitle(title);
        setRenamingId("");
        setRenameDraft("");
      } catch (e) {
        setError(e instanceof Error ? e.message : ts("重命名失败"));
      }
    },
    [agentCode, conversationId, renameDraft, ts],
  );

  const stop = useCallback(() => {
    abortRef.current?.abort();
  }, []);

  const charCount = Array.from(input).length;
  const overLimit = charCount > maxInput;

  return (
    <div className="flex h-full min-h-0 flex-col bg-white dark:bg-gray-950">
      <header className="flex items-center gap-3 border-b border-gray-200/70 px-4 py-3 dark:border-white/10 md:px-6 lg:pr-48">
        <button
          type="button"
          onClick={() => setShowHistory((v) => !v)}
          className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-xl border border-gray-200 text-gray-500 transition hover:bg-gray-50 dark:border-white/10 dark:text-gray-400 dark:hover:bg-white/5"
          aria-label={ts("智能体历史")}
          title={ts("智能体历史")}
        >
          <History size={16} />
        </button>
        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-lg">
          <AgentIcon code={agentCode} value={workflow.icon} className="h-5 w-5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <h2 className="truncate text-sm font-semibold text-gray-900 dark:text-gray-100">{conversationTitle || agentName}</h2>
            {chatModelCode ? (
              <span className="hidden shrink-0 rounded-full bg-gray-100 px-2 py-0.5 text-[10px] text-gray-500 dark:bg-white/10 dark:text-gray-400 sm:inline">
                {ts("按实际消耗")}
              </span>
            ) : null}
          </div>
          <p className="truncate text-xs text-gray-400">{subtitle}</p>
        </div>
        {messages.length > 0 ? (
          <button
            type="button"
            onClick={() => downloadMarkdown(agentName, messages)}
            className="hidden h-8 items-center gap-1 rounded-lg border border-gray-200 px-2.5 text-xs text-gray-500 transition hover:bg-gray-50 dark:border-white/10 dark:text-gray-400 dark:hover:bg-white/5 sm:inline-flex"
          >
            <Download size={13} />
            {ts("导出")}
          </button>
        ) : null}
        <button
          type="button"
          onClick={resetChat}
          className="inline-flex h-8 items-center gap-1 rounded-lg bg-primary px-2.5 text-xs font-medium text-white transition hover:opacity-90"
        >
          <Plus size={13} />
          {ts("新建")}
        </button>
      </header>

      {showHistory ? (
        <section className="max-h-72 overflow-auto border-b border-gray-200/70 bg-gray-50/60 px-4 py-2 dark:border-white/10 dark:bg-white/[0.03]">
          {historyBusy && history.length === 0 ? (
            <div className="flex items-center justify-center gap-2 py-4 text-xs text-gray-400">
              <Loader2 size={13} className="animate-spin" />
              {ts("加载中…")}
            </div>
          ) : history.length === 0 ? (
            <div className="py-4 text-center text-xs text-gray-400">{ts("暂无历史会话")}</div>
          ) : (
            <ul className="mx-auto max-w-5xl divide-y divide-gray-200/70 dark:divide-white/5">
              {history.map((item) => (
                <li key={item.public_id} className="flex items-center gap-2 py-1.5">
                  {renamingId === item.public_id ? (
                    <form
                      className="flex min-w-0 flex-1 items-center gap-1"
                      onSubmit={(e) => {
                        e.preventDefault();
                        void submitRename(item.public_id);
                      }}
                    >
                      <input
                        value={renameDraft}
                        onChange={(e) => setRenameDraft(e.target.value)}
                        maxLength={60}
                        autoFocus
                        className="soft-input min-w-0 flex-1 px-2 py-1 text-xs"
                      />
                      <button type="submit" className="rounded-md p-1 text-primary hover:bg-primary/10" aria-label={ts("保存")}>
                        <Check size={13} />
                      </button>
                      <button
                        type="button"
                        onClick={() => setRenamingId("")}
                        className="rounded-md p-1 text-gray-400 hover:bg-gray-100 dark:hover:bg-white/10"
                        aria-label={ts("取消")}
                      >
                        <X size={13} />
                      </button>
                    </form>
                  ) : (
                    <>
                      <button
                        type="button"
                        onClick={() => void openConversation(item.public_id)}
                        className={`flex min-w-0 flex-1 items-center gap-2 px-1 py-1 text-left text-xs transition ${
                          conversationId === item.public_id ? "text-primary" : "text-gray-600 dark:text-gray-300"
                        }`}
                      >
                        <span className="min-w-0 flex-1 truncate">{item.title || ts("未命名会话")}</span>
                        <span className="shrink-0 text-[10px] text-gray-400">{item.message_count} {ts("条")}</span>
                        <span className="hidden shrink-0 text-[10px] text-gray-400 sm:inline">
                          {new Date(item.updated_at).toLocaleString()}
                        </span>
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          setRenamingId(item.public_id);
                          setRenameDraft(item.title || "");
                        }}
                        className="rounded-md p-1 text-gray-400 hover:bg-gray-100 hover:text-gray-700 dark:hover:bg-white/10"
                        aria-label={ts("重命名")}
                        title={ts("重命名")}
                      >
                        <Pencil size={12} />
                      </button>
                      <button
                        type="button"
                        onClick={() => void removeConversation(item.public_id)}
                        className="rounded-md p-1 text-gray-400 hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-500/10"
                        aria-label={ts("删除会话")}
                        title={ts("删除会话")}
                      >
                        <Trash2 size={12} />
                      </button>
                    </>
                  )}
                </li>
              ))}
            </ul>
          )}
        </section>
      ) : null}

      <div
        className="min-h-0 flex-1 overflow-y-auto px-4 py-6 md:px-6"
        onScroll={(e) => {
          const el = e.currentTarget;
          stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
        }}
      >
        <div className="mx-auto max-w-5xl space-y-5">
          {messages.length === 0 ? (
            <div className="mx-auto max-w-3xl">
              <div className="soft-card overflow-hidden p-6 md:p-8">
                <div className="flex items-start gap-4">
                  <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-2xl">
                    <AgentIcon code={agentCode} value={workflow.icon} className="h-7 w-7" />
                  </div>
                  <div className="min-w-0 flex-1">
                    <h3 className="text-lg font-semibold text-gray-900 dark:text-gray-100">{agentName}</h3>
                    {subtitle ? <p className="mt-1 text-xs leading-relaxed text-gray-500 dark:text-gray-400">{subtitle}</p> : null}
                    {heroTags.length > 0 ? (
                      <div className="mt-2 flex flex-wrap gap-1.5">
                        {heroTags.map((tag) => (
                          <span key={tag} className="rounded-full bg-primary/10 px-2 py-0.5 text-[10px] font-medium text-primary">
                            {tag}
                          </span>
                        ))}
                      </div>
                    ) : null}
                  </div>
                </div>
                {greeting ? (
                  <div className="mt-5 flex gap-3 rounded-2xl bg-gray-50 px-4 py-3 dark:bg-white/[0.04]">
                    <Sparkles size={15} className="mt-0.5 shrink-0 text-primary" />
                    <p className="text-sm leading-relaxed text-gray-700 dark:text-gray-200">{greeting}</p>
                  </div>
                ) : null}
              </div>

              {quickPrompts.length > 0 ? (
                <div className="mt-4 grid gap-2 sm:grid-cols-2">
                  {quickPrompts.map((prompt, index) => (
                    <button
                      key={prompt}
                      type="button"
                      onClick={() => void send(prompt)}
                      className="group flex items-start gap-2.5 rounded-2xl border border-gray-200 bg-white px-3.5 py-3 text-left text-xs leading-relaxed text-gray-600 transition hover:border-primary/40 hover:text-primary hover:shadow-sm dark:border-white/10 dark:bg-white/[0.03] dark:text-gray-300"
                    >
                      <span className="mt-0.5 shrink-0 rounded-md bg-gray-100 px-1.5 py-0.5 text-[10px] font-semibold text-gray-500 group-hover:bg-primary/10 group-hover:text-primary dark:bg-white/10 dark:text-gray-400">
                        {String(index + 1).padStart(2, "0")}
                      </span>
                      <span className="min-w-0 flex-1">{prompt}</span>
                    </button>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}

          {messages.map((message, index) => {
            if (message.role === "user") {
              const isEditing = editingIndex === index;
              return (
                <div key={`${index}-${message.id || "local"}`} className="flex flex-col items-end gap-1">
                  {isEditing ? (
                    <div className="w-full max-w-[92%] rounded-2xl border border-primary/30 bg-white p-2 shadow-sm dark:bg-gray-900">
                      <textarea
                        value={editDraft}
                        onChange={(e) => setEditDraft(e.target.value)}
                        rows={Math.min(8, Math.max(2, editDraft.split("\n").length))}
                        className="soft-input w-full resize-y px-3 py-2 text-sm"
                        autoFocus
                      />
                      <div className="mt-2 flex items-center justify-end gap-2">
                        <button
                          type="button"
                          onClick={() => setEditingIndex(null)}
                          className="rounded-lg px-2.5 py-1 text-xs text-gray-500 hover:bg-gray-100 dark:hover:bg-white/10"
                        >
                          {ts("取消")}
                        </button>
                        <button
                          type="button"
                          onClick={() => void rerunFrom(index, editDraft)}
                          disabled={!editDraft.trim()}
                          className="rounded-lg bg-primary px-2.5 py-1 text-xs font-medium text-white disabled:opacity-40"
                        >
                          {ts("保存并重新生成")}
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="max-w-[85%] rounded-2xl rounded-br-md bg-primary px-4 py-2.5 text-sm leading-relaxed text-white shadow-sm whitespace-pre-wrap">
                      {message.content}
                    </div>
                  )}
                  {!isEditing && !busy ? (
                    <div className="flex items-center gap-0.5">
                      <MessageAction label={ts("复制")} onClick={() => copyToClipboard(message.content)} icon={<Copy size={12} />} />
                      <MessageAction
                        label={ts("编辑并重新生成")}
                        onClick={() => {
                          setEditingIndex(index);
                          setEditDraft(message.content);
                        }}
                        icon={<Pencil size={12} />}
                      />
                      <MessageAction label={ts("删除")} onClick={() => void deleteMessage(index)} icon={<Trash2 size={12} />} danger />
                    </div>
                  ) : null}
                </div>
              );
            }

            const isLast = index === messages.length - 1;
            const streaming = busy && isLast;
            return (
              <div key={`${index}-${message.id || "local"}`} className="flex flex-col items-start gap-1">
                {message.thinking ? <ThinkingBlock text={message.thinking} streaming={streaming && !message.content} /> : null}
                {message.content ? (
                  <div className="soft-card max-w-[95%] px-4 py-3 text-sm leading-relaxed text-gray-700 dark:text-gray-200">
                    <AgentRichText content={message.content} />
                    {streaming ? <span className="ml-0.5 inline-block h-4 w-1.5 animate-pulse rounded-sm bg-primary align-text-bottom" /> : null}
                  </div>
                ) : null}
                {!streaming && message.content ? (
                  <div className="flex items-center gap-0.5">
                    <MessageAction
                      label={copiedIndex === index ? ts("已复制") : ts("复制回复")}
                      onClick={() => {
                        copyToClipboard(message.content);
                        setCopiedIndex(index);
                        setTimeout(() => setCopiedIndex((current) => (current === index ? null : current)), 1500);
                      }}
                      icon={copiedIndex === index ? <Check size={12} /> : <Copy size={12} />}
                    />
                    <MessageAction
                      label={ts("重新生成")}
                      onClick={() => {
                        const previousUser = [...messages.slice(0, index)].reverse().findIndex((m) => m.role === "user");
                        if (previousUser < 0) return;
                        void rerunFrom(index - previousUser - 1);
                      }}
                      icon={<RefreshCw size={12} />}
                    />
                    <MessageAction label={ts("删除")} onClick={() => void deleteMessage(index)} icon={<Trash2 size={12} />} danger />
                    {message.stopped ? <span className="ml-1 text-[10px] text-gray-400">{ts("已停止 · 未保存")}</span> : null}
                  </div>
                ) : null}
              </div>
            );
          })}

          {busy && !(messages[messages.length - 1]?.role === "assistant" && messages[messages.length - 1]?.content) ? (
            <div className="flex items-center gap-2 text-xs text-gray-500 dark:text-gray-400">
              <Loader2 size={14} className="animate-spin text-primary" />
              <span role="status" aria-live="polite">
                {stage || ts("正在思考…")}
                {waited > 0 ? ` · ${waited}s` : ""}
              </span>
            </div>
          ) : null}

          {needRecharge ? (
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs text-amber-700 dark:border-amber-400/20 dark:bg-amber-500/10 dark:text-amber-300">
              <span>{ts("算力不足，充值后可继续对话。")}</span>
              <a href="/app/recharge" className="rounded-lg bg-amber-500 px-2.5 py-1 font-medium text-white hover:opacity-90">
                {ts("去充值")}
              </a>
            </div>
          ) : null}

          {error ? (
            <div className="flex items-start gap-2 rounded-xl border border-red-200 bg-red-50 px-3 py-2 text-xs text-red-500 dark:border-red-500/20 dark:bg-red-500/10">
              <span className="min-w-0 flex-1">{error}</span>
              <button type="button" onClick={() => setError("")} className="shrink-0 rounded p-0.5 hover:bg-red-100 dark:hover:bg-red-500/20" aria-label={ts("关闭")}>
                <X size={12} />
              </button>
            </div>
          ) : null}

          <div ref={messagesEndRef} />
        </div>
      </div>

      <footer className="border-t border-gray-200/70 px-4 py-3 dark:border-white/10 md:px-6">
        <form
          className="mx-auto max-w-5xl"
          onSubmit={(e) => {
            e.preventDefault();
            void send();
          }}
        >
          <div className="flex items-end gap-2">
            <div className="relative min-w-0 flex-1">
              <textarea
                value={input}
                onChange={(e) => setInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" && !e.shiftKey && !e.nativeEvent.isComposing) {
                    e.preventDefault();
                    void send();
                  }
                }}
                rows={1}
                placeholder={placeholder}
                aria-label={placeholder}
                className={`soft-input block max-h-40 min-h-[44px] w-full resize-y px-3 py-2.5 text-sm ${overLimit ? "border-red-300 dark:border-red-500/40" : ""}`}
              />
              {charCount > maxInput * 0.8 ? (
                <span className={`pointer-events-none absolute bottom-1.5 right-2 text-[10px] ${overLimit ? "text-red-500" : "text-gray-400"}`}>
                  {charCount}/{maxInput}
                </span>
              ) : null}
            </div>
            {busy ? (
              <button
                type="button"
                onClick={stop}
                className="inline-flex h-[44px] w-[44px] shrink-0 items-center justify-center rounded-xl border border-gray-300 text-gray-600 transition hover:bg-gray-50 dark:border-white/15 dark:text-gray-300 dark:hover:bg-white/5"
                aria-label={ts("停止生成")}
                title={ts("停止生成")}
              >
                <Square size={14} />
              </button>
            ) : (
              <button
                type="submit"
                disabled={!input.trim() || overLimit}
                className="inline-flex h-[44px] w-[44px] shrink-0 items-center justify-center rounded-xl bg-primary text-white transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
                aria-label={ts("发送")}
                title={ts("发送")}
              >
                <Send size={16} />
              </button>
            )}
          </div>
          <div className="mt-1.5 flex items-start justify-between gap-3">
            <div className="min-w-0 text-[10px] leading-relaxed text-gray-400">
              <p>
                {billingLabel} · {t("workspace.tokenBilling")} · <a className="underline underline-offset-2 hover:text-primary" href="/app/pricing">{t("workspace.viewPricing")}</a>
              </p>
              {disclaimer ? <p className="mt-0.5">{disclaimer}</p> : null}
            </div>
            <p className="hidden shrink-0 text-[10px] text-gray-400 sm:block">{ts("Enter 发送 · Shift + Enter 换行")}</p>
          </div>
        </form>
      </footer>
    </div>
  );
}

export default PromptAgentWorkspace;
