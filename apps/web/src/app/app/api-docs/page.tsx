"use client";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { Check, Copy, KeyRound, Search } from "lucide-react";
import { ReferralShareButton } from "@/components/ReferralShareButton";
import { WorkbenchTopActions } from "@/components/WorkbenchTopActions";
import { useI18n } from "@/i18n/I18nProvider";
import { API_URL, apiForLocale } from "@/lib/api";
import { useSiteBranding } from "@/components/SiteBrand";
import {
  ERRORS,
  SECTIONS,
  authHeaders,
  collectModels,
  curlExample,
  endpointFor,
  modelsFor,
  outcomeCopy,
  pretty,
  requestBody,
  requestFields,
  responseExample,
  sectionVisible,
  streamSample,
  succeededExample,
  type ApiDocRecord,
  type ChatProtocol,
  type DocModel,
  type PublicModel,
  type SectionId,
} from "./contract";

const PROTOCOLS: Array<{ id: ChatProtocol; label: string; path: string }> = [
  { id: "openai", label: "OpenAI 兼容", path: "/v1/chat/completions" },
  { id: "anthropic", label: "Anthropic Messages", path: "/v1/messages" },
  { id: "gemini", label: "Gemini generateContent", path: "/v1beta/models/{model}:generateContent" },
];

function CopyButton({ value, label, copiedLabel }: { value: string; label: string; copiedLabel: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <button
      type="button"
      className="mcdl-button-secondary mcdl-hover-control mcdl-hover-press inline-flex shrink-0 items-center gap-1.5 px-3 py-1.5 text-[0.75rem]"
      onClick={async () => {
        try {
          await navigator.clipboard.writeText(value);
          setCopied(true);
          window.setTimeout(() => setCopied(false), 1400);
        } catch {
          setCopied(false);
        }
      }}
    >
      {copied ? <Check size={13} /> : <Copy size={13} />}
      {copied ? copiedLabel : label}
    </button>
  );
}

function CodeBlock({ title, value, copyLabel, copiedLabel }: { title: string; value: string; copyLabel: string; copiedLabel: string }) {
  return (
    <section className="mcdl-editorial-card overflow-hidden">
      <div className="flex items-center justify-between gap-3 px-5 py-4">
        <h2 className="text-[1.05rem]">{title}</h2>
        <CopyButton value={value} label={copyLabel} copiedLabel={copiedLabel} />
      </div>
      <pre className="mcdl-stage mx-3 mb-3 overflow-x-auto p-4 text-[0.75rem] leading-6 text-[#f5efeb]" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{value}</pre>
    </section>
  );
}

export default function ApiDocsPage() {
  const { t, ts, locale } = useI18n();
  const { api_docs_enabled, api_docs_operations } = useSiteBranding();
  const [docs, setDocs] = useState<ApiDocRecord[]>([]);
  const [publicModels, setPublicModels] = useState<PublicModel[]>([]);
  const [loading, setLoading] = useState(true);
  const [section, setSection] = useState<SectionId>("start");
  const [protocol, setProtocol] = useState<ChatProtocol>("openai");
  const [modelKey, setModelKey] = useState("");
  const [search, setSearch] = useState("");
  const [baseUrl, setBaseUrl] = useState("http://localhost:8080");
  const [apiKey, setApiKey] = useState("");
  const [taskNo, setTaskNo] = useState("");
  const [bodyText, setBodyText] = useState("");
  const [stream, setStream] = useState(false);
  const [result, setResult] = useState("");
  const [sending, setSending] = useState(false);
  const documentPane = useRef<HTMLElement>(null);

  useEffect(() => {
    documentPane.current?.scrollTo({ top: 0 });
  }, [section]);

  useEffect(() => {
    const resolved = defaultBaseUrl();
    if (resolved) setBaseUrl(resolved);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    if (api_docs_enabled === false) return () => controller.abort();
    setLoading(true);
    Promise.all([
      apiForLocale<{ items: ApiDocRecord[] }>("/api/api-docs", locale, { signal: controller.signal }).catch((error) => {
        if (error?.name === "AbortError") throw error;
        return { items: [] as ApiDocRecord[] };
      }),
      apiForLocale<PublicModel[]>("/api/models", locale, { signal: controller.signal }).catch((error) => {
        if (error?.name === "AbortError") throw error;
        return [] as PublicModel[];
      }),
    ]).then(([docResult, modelResult]) => {
      setDocs(docResult.items || []);
      setPublicModels(Array.isArray(modelResult) ? modelResult : []);
      setLoading(false);
    }).catch((error) => {
      if (error?.name !== "AbortError") setLoading(false);
    });
    return () => controller.abort();
  }, [locale, api_docs_enabled]);

  const models = useMemo(() => collectModels(docs, publicModels), [docs, publicModels]);
  const visibleSections = SECTIONS.filter((item) => sectionVisible(item.id, api_docs_operations));
  const activeSection = visibleSections.find((item) => item.id === section) || visibleSections[0];
  const query = search.trim().toLowerCase();
  const sectionModels = modelsFor(models, activeSection?.id || "start");
  const filteredModels = sectionModels.filter((model) => !query || [model.name, model.modelId, model.description].join(" ").toLowerCase().includes(query));
  const activeModel = filteredModels.find((model) => model.key === modelKey) || filteredModels[0];
  const activeProtocol: ChatProtocol = activeSection?.id === "chat" ? protocol : "openai";
  const endpoint = endpointFor(activeSection?.id || "start", activeProtocol, activeModel?.modelId || "", taskNo);
  const body = requestBody(activeSection?.id || "start", activeProtocol, activeModel);
  const fields = requestFields(activeSection?.id || "start", activeProtocol, body);
  const response = responseExample(activeSection?.id || "start", activeProtocol, activeModel?.modelId || "");
  const done = succeededExample(activeSection?.id || "start");
  const curl = curlExample(baseUrl, endpoint.method, endpoint.path, activeProtocol, body);
  const bodyKey = body ? JSON.stringify(body) : "";
  const needsTaskNo = endpoint.path.includes("{task_no}");
  const showModels = activeSection?.id === "chat" || activeSection?.id === "image" || activeSection?.id === "video" || activeSection?.id === "audio";

  useEffect(() => {
    setBodyText(bodyKey ? pretty(JSON.parse(bodyKey)) : "");
    setStream(false);
    setResult("");
  }, [activeSection?.id, activeProtocol, activeModel?.key, bodyKey]);

  if (api_docs_enabled === false) {
    return <div className="flex h-full items-center justify-center bg-[var(--mcdl-color-surface)] px-6"><div className="mcdl-editorial-card px-8 py-12 text-center text-sm text-[var(--mcdl-text-body)]">{ts("API 文档暂未开放")}</div></div>;
  }

  const selectSection = (id: SectionId) => {
    setSection(id);
    setModelKey("");
  };

  const send = async () => {
    if (!apiKey.trim()) {
      setResult(ts("请填写 API Key。"));
      return;
    }
    if (needsTaskNo || endpoint.path.includes("{model}")) {
      setResult(needsTaskNo ? ts("请填写 task_no。") : ts("请选择模型。"));
      return;
    }
    setSending(true);
    setResult("");
    try {
      const headers: Record<string, string> = {};
      if (activeProtocol === "anthropic") {
        headers["x-api-key"] = apiKey.trim();
        headers["anthropic-version"] = "2023-06-01";
      } else if (activeProtocol === "gemini") {
        headers["x-goog-api-key"] = apiKey.trim();
      } else {
        headers.Authorization = "Bearer " + apiKey.trim();
      }
      let payload: Record<string, unknown> | undefined;
      if (endpoint.method === "POST") {
        headers["Content-Type"] = "application/json";
        const parsed = JSON.parse(bodyText || "{}") as Record<string, unknown>;
        if (stream) parsed.stream = true;
        payload = parsed;
      }
      let url = baseUrl.replace(/\/+$/, "") + endpoint.path;
      if (stream && activeProtocol === "gemini") {
        url = url.replace(":generateContent", ":streamGenerateContent");
        url += url.includes("?") ? "&alt=sse" : "?alt=sse";
      }
      const response = await fetch(url, { method: endpoint.method, headers, body: payload ? JSON.stringify(payload) : undefined });
      const raw = await response.text();
      if (/<html[\s>]/i.test(raw)) {
        setResult("HTTP " + response.status + "\n\n" + ts("该地址返回 HTML。请将 Base URL 改为 API 地址；本地直连为 http://localhost:8080。"));
        return;
      }
      setResult("HTTP " + response.status + "\n\n" + raw);
    } catch (error) {
      setResult(error instanceof SyntaxError ? ts("请求体不是有效 JSON。") : error instanceof Error ? error.message : ts("请求失败。请检查 Base URL、API Key 与网络。"));
    } finally {
      setSending(false);
    }
  };

  return (
    <div className="mcdl-api-docs flex h-full min-h-0 bg-[var(--mcdl-color-surface)] text-[var(--mcdl-text-title)]">
      <aside className="hidden h-full w-[18rem] shrink-0 flex-col bg-[var(--mcdl-color-surface-subtle)] lg:flex">
        <div className="px-4 pb-4 pt-5">
          <p className="flex items-center gap-2 text-[0.72rem] font-bold uppercase tracking-[0.16em] text-[var(--mcdl-color-accent)]"><span className="h-1.5 w-1.5 rounded-full bg-[var(--mcdl-color-accent)]" />API</p>
          <p className="mt-2 text-[1.7rem] leading-none" style={{ fontFamily: "var(--mcdl-font-display)" }}>{ts("文档中心")}</p>
          <label className="mt-5 flex items-center gap-2 rounded-full bg-[var(--mcdl-field-surface)] px-3 py-2.5">
            <Search size={14} className="text-[var(--mcdl-text-muted)]" />
            <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("apiDocs.search")} className="min-w-0 flex-1 bg-transparent text-[0.8rem] outline-none" />
          </label>
        </div>
        <nav className="flex-1 space-y-1 overflow-y-auto px-3 pb-5" aria-label={ts("文档目录")}>
          {visibleSections.filter((item) => sectionMatches(item.id, item.label, query, models)).map((item) => {
            const matches = query ? models.filter((model) => model.section === item.id && [model.name, model.modelId].join(" ").toLowerCase().includes(query)).slice(0, 6) : [];
            const selected = activeSection?.id === item.id;
            return (
              <div key={item.id}>
                <button type="button" onClick={() => selectSection(item.id)} className={"mcdl-hover-quiet mcdl-hover-press w-full rounded-2xl px-3 py-2.5 text-left " + (selected ? "bg-[var(--mcdl-accent-soft)]" : "")}>
                  <span className="block text-[0.92rem] font-semibold">{ts(item.label)}</span>
                  <span className="mt-0.5 block text-[0.72rem] text-[var(--mcdl-text-body)]">{ts(item.hint)}</span>
                </button>
                {matches.length > 0 && <div className="mb-2 mt-1 space-y-1 pl-3">{matches.map((model) => <button key={model.key} type="button" onClick={() => { selectSection(item.id); setModelKey(model.key); }} className="mcdl-hover-quiet block w-full truncate rounded-xl px-3 py-1.5 text-left text-[0.75rem] text-[var(--mcdl-text-body)]">{model.name}</button>)}</div>}
              </div>
            );
          })}
        </nav>
      </aside>

      <main ref={documentPane} className="min-h-0 min-w-0 flex-1 overflow-y-auto">
        <div className="sticky top-0 z-20 flex items-center justify-between gap-3 bg-[var(--mcdl-overlay-surface)] px-4 py-3 backdrop-blur sm:px-6">
          <Link href="/app" className="mcdl-button-secondary mcdl-hover-control mcdl-hover-press px-4 py-2 text-[0.8rem]">{t("apiDocs.backWorkspace")}</Link>
          <div className="hidden lg:block"><WorkbenchTopActions /></div>
        </div>
        {loading || !activeSection ? <DocsSkeleton /> : (
          <div className="mx-auto flex w-full max-w-[76rem] flex-col gap-5 px-4 py-6 sm:px-6 sm:py-8">
            <div className="lg:hidden">
              <label className="mb-3 flex items-center gap-2 rounded-full bg-[var(--mcdl-field-surface)] px-3 py-2.5">
                <Search size={14} />
                <input value={search} onChange={(event) => setSearch(event.target.value)} placeholder={t("apiDocs.search")} className="min-w-0 flex-1 bg-transparent text-[0.8rem] outline-none" />
              </label>
              <select value={activeSection.id} onChange={(event) => selectSection(event.target.value as SectionId)} className="w-full rounded-2xl px-3 py-3 text-sm" aria-label={ts("文档目录")}>
                {visibleSections.map((item) => <option key={item.id} value={item.id}>{ts(item.label)}</option>)}
              </select>
            </div>

            <header>
              <p className="flex items-center gap-2 text-[0.72rem] font-bold uppercase tracking-[0.16em] text-[var(--mcdl-color-accent)]"><span className="h-1.5 w-1.5 rounded-full bg-[var(--mcdl-color-accent)]" />API</p>
              <h1 className="mt-3 text-[clamp(1.75rem,2.5vw,2.5rem)] leading-none">{ts(activeSection.id === "start" ? "API 参考" : activeSection.label)}</h1>
              <p className="mt-4 max-w-3xl text-[0.95rem] leading-7 text-[var(--mcdl-text-body)]">{ts(outcomeCopy(activeSection.id, activeProtocol))}</p>
            </header>

            {activeSection.id === "start" && <Conventions />}

            {activeSection.id === "chat" && (
              <div className="grid gap-2 sm:grid-cols-3" role="tablist" aria-label={ts("协议")}>
                {PROTOCOLS.map((item) => (
                  <button key={item.id} type="button" role="tab" aria-selected={activeProtocol === item.id} onClick={() => setProtocol(item.id)} className={"mcdl-hover-press rounded-2xl px-4 py-3 text-left " + (activeProtocol === item.id ? "bg-[var(--mcdl-color-accent)] text-[var(--mcdl-color-on-accent)]" : "mcdl-hover-quiet bg-[var(--mcdl-color-surface-card)] text-[var(--mcdl-text-body)]")}>
                    <span className="block text-sm font-semibold">{item.label}</span>
                    <code className="mt-1 block truncate text-[0.72rem] opacity-80">{item.path}</code>
                  </button>
                ))}
              </div>
            )}

            <section className="mcdl-editorial-card p-5">
              <div className="flex flex-wrap items-center gap-3">
                <span className="rounded-full bg-[var(--mcdl-color-accent)] px-2.5 py-1 text-[0.72rem] font-bold text-[var(--mcdl-color-on-accent)]" style={{ fontFamily: "var(--mcdl-font-numeric)" }}>{endpoint.method}</span>
                <code className="min-w-0 flex-1 break-all text-[0.85rem]" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{baseUrl.replace(/\/+$/, "") + endpoint.path}</code>
                <CopyButton value={baseUrl.replace(/\/+$/, "") + endpoint.path} label={ts("复制地址")} copiedLabel={ts("已复制")} />
              </div>
              <div className="mt-4 flex flex-wrap items-end justify-between gap-3">
                <label className="min-w-[16rem] flex-1 text-[0.75rem] text-[var(--mcdl-text-body)]">
                  Base URL
                  <input value={baseUrl} onChange={(event) => setBaseUrl(event.target.value)} className="mt-1.5 w-full rounded-2xl px-3 py-2.5 text-[0.85rem]" spellCheck={false} />
                </label>
                <Link href="/app/settings" className="mcdl-button-secondary mcdl-hover-control mcdl-hover-press inline-flex items-center gap-1.5 px-4 py-2 text-[0.8rem]"><KeyRound size={14} />{t("apiDocs.manageKeys")}</Link>
              </div>
              <p className="mt-2 text-[0.75rem] leading-5 text-[var(--mcdl-text-muted)]">{ts("直连 API 使用 http://localhost:8080。当前站点若反代 /v1 与 /v1beta，可使用站点源。")}</p>
              <div className="mt-4 grid gap-2 sm:grid-cols-2">
                {authHeaders(activeProtocol).map(([name, value]) => (
                  <div key={name} className="rounded-[1rem] bg-[var(--mcdl-color-surface-subtle)] px-3 py-2">
                    <div className="text-[0.72rem] text-[var(--mcdl-text-muted)]">{name}</div>
                    <code className="text-[0.8rem]" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{value}</code>
                  </div>
                ))}
              </div>
              {(activeSection.id === "task" || activeSection.id === "events") && (
                <label className="mt-4 block text-[0.75rem] text-[var(--mcdl-text-body)]">
                  task_no
                  <input value={taskNo} onChange={(event) => setTaskNo(event.target.value)} placeholder="task_no" className="mt-1.5 w-full rounded-2xl px-3 py-2.5 text-[0.85rem]" spellCheck={false} />
                </label>
              )}
            </section>

            {showModels && (
              <section className="mcdl-editorial-card p-5">
                <div className="flex flex-wrap items-end justify-between gap-3">
                  <div>
                    <h2 className="text-[1.15rem]">{ts("模型")}</h2>
                    <p className="mt-1 text-[0.8rem] text-[var(--mcdl-text-body)]">{ts("请求参数 model 使用 ID，不使用展示名称。")}</p>
                  </div>
                  <span className="text-[0.8rem] text-[var(--mcdl-text-muted)]" style={{ fontFamily: "var(--mcdl-font-numeric)" }}>{filteredModels.length}</span>
                </div>
                {filteredModels.length === 0 ? <p className="mt-4 text-sm text-[var(--mcdl-text-body)]">{ts("没有匹配的模型。")}</p> : (
                  <div className="mt-4 grid max-h-[24rem] gap-2 overflow-y-auto sm:grid-cols-2">
                    {filteredModels.map((model) => {
                      const selected = activeModel?.key === model.key;
                      return (
                        <button key={model.key} type="button" onClick={() => setModelKey(model.key)} className={"mcdl-hover-quiet mcdl-hover-press rounded-[1.1rem] px-3 py-3 text-left " + (selected ? "bg-[var(--mcdl-accent-soft)]" : "bg-[var(--mcdl-color-surface-subtle)]")}>
                          <span className="block truncate text-sm font-semibold">{model.name}</span>
                          <code className="mt-1 block truncate text-[0.75rem] text-[var(--mcdl-text-body)]" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{model.modelId}</code>
                        </button>
                      );
                    })}
                  </div>
                )}
                {activeModel && (
                  <div className="mt-4 flex flex-wrap items-center gap-3">
                    <p className="max-w-3xl text-[0.8rem] leading-6 text-[var(--mcdl-text-body)]">{activeModel.description}</p>
                    <CopyButton value={activeModel.modelId} label={ts("复制模型 ID")} copiedLabel={ts("已复制")} />
                  </div>
                )}
                {activeModel && activeModel.tags.length > 0 && <div className="mt-3 flex flex-wrap gap-2">{activeModel.tags.map((tag) => <span key={tag} className="rounded-full bg-[var(--mcdl-color-surface-subtle)] px-2.5 py-1 text-[0.7rem] uppercase tracking-[0.08em] text-[var(--mcdl-text-body)]">{tag}</span>)}</div>}
              </section>
            )}

            {fields.length > 0 && (
              <section className="mcdl-editorial-card p-5">
                <h2 className="text-[1.15rem]">{ts("参数")}</h2>
                <div className="mt-4 overflow-x-auto">
                  <table className="w-full min-w-[40rem] text-left text-sm">
                    <thead className="text-[0.72rem] uppercase tracking-[0.08em] text-[var(--mcdl-text-muted)]">
                      <tr><th className="px-2 py-2 font-medium">{ts("参数")}</th><th className="px-2 py-2 font-medium">{ts("类型")}</th><th className="px-2 py-2 font-medium">{ts("必填")}</th><th className="px-2 py-2 font-medium">{ts("说明")}</th></tr>
                    </thead>
                    <tbody>
                      {fields.map((field) => (
                        <tr key={field.name} className="align-top">
                          <td className="px-2 py-3"><code style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{field.name}</code></td>
                          <td className="px-2 py-3 text-[var(--mcdl-text-body)]"><code style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{field.type}</code></td>
                          <td className="px-2 py-3">{ts(field.required)}</td>
                          <td className="px-2 py-3 leading-6 text-[var(--mcdl-text-body)]">{ts(field.help)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
            )}

            <CodeBlock title="cURL" value={curl} copyLabel={ts("复制")} copiedLabel={ts("已复制")} />
            {body && <CodeBlock title={ts("请求")} value={pretty(body)} copyLabel={ts("复制")} copiedLabel={ts("已复制")} />}
            <CodeBlock title={ts("响应")} value={pretty(response)} copyLabel={ts("复制")} copiedLabel={ts("已复制")} />
            {done && <CodeBlock title={ts("任务完成")} value={pretty(done)} copyLabel={ts("复制")} copiedLabel={ts("已复制")} />}

            {(activeSection.id === "chat" || activeSection.id === "start") && (
              <section className="mcdl-editorial-card p-5">
                <h2 className="text-[1.15rem]">{ts("流式响应")}</h2>
                <p className="mt-2 text-[0.8rem] leading-6 text-[var(--mcdl-text-body)]">{activeProtocol === "gemini" ? ts("将 generateContent 替换为 streamGenerateContent，并附加查询参数 alt=sse。") : ts("设置 stream 为 true。增量文本位于 data 行，结束标记为 data: [DONE]。")}</p>
                <pre className="mcdl-stage mt-4 overflow-x-auto p-4 text-[0.75rem] leading-6 text-[#f5efeb]" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{streamSample(activeProtocol)}</pre>
              </section>
            )}

            {(activeSection.id === "image" || activeSection.id === "video" || activeSection.id === "audio") && (
              <section className="mcdl-editorial-card p-5">
                <h2 className="text-[1.15rem]">{ts("约束")}</h2>
                <ul className="mt-3 list-disc space-y-2 pl-5 text-[0.85rem] leading-6 text-[var(--mcdl-text-body)]">
                  <li>{ts("创建接口不支持幂等键。调用方需要保存 task_no。")}</li>
                  <li>{ts("参考图、参考视频和参考音频必须是上游可访问的公网 URL。")}</li>
                  {activeSection.id === "image" && <li>{ts("本地参考图使用 POST /v1/images/edits，Content-Type 为 multipart/form-data，字段为 model、prompt、image。")}</li>}
                  <li>{ts("账号级限流约为 120 次/分钟。超限返回 HTTP 429，响应体为 { code, message }。")}</li>
                </ul>
              </section>
            )}

            <section className="mcdl-editorial-card p-5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <h2 className="text-[1.15rem]">{ts("调试")}</h2>
                <span className="rounded-full bg-[var(--mcdl-color-surface-subtle)] px-2.5 py-1 text-[0.72rem] text-[var(--mcdl-text-body)]">{ts("API Key 不写入存储")}</span>
              </div>
              <div className="mt-4 grid gap-3 lg:grid-cols-[1fr_16rem_auto]">
                <input value={baseUrl.replace(/\/+$/, "") + endpoint.path} readOnly className="rounded-2xl px-3 py-2.5 text-[0.75rem]" aria-label={ts("接口地址")} />
                <input value={apiKey} onChange={(event) => setApiKey(event.target.value)} type="password" autoComplete="off" placeholder="API Key" className="rounded-2xl px-3 py-2.5 text-[0.8rem]" aria-label="API Key" />
                <button type="button" onClick={send} disabled={sending} className="mcdl-button-primary mcdl-hover-primary-action mcdl-hover-press px-5 py-2.5 text-[0.8rem] font-semibold disabled:opacity-50">{sending ? ts("请求中") : ts("发送请求")}</button>
              </div>
              {(activeSection.id === "chat" || activeSection.id === "start") && endpoint.method === "POST" && (
                <label className="mt-3 flex items-center gap-2 text-[0.8rem]"><input type="checkbox" checked={stream} onChange={(event) => setStream(event.target.checked)} />stream</label>
              )}
              {endpoint.method === "POST" && <textarea value={bodyText} onChange={(event) => setBodyText(event.target.value)} className="mt-3 min-h-36 w-full rounded-[1.1rem] p-3 text-[0.75rem] leading-6" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }} aria-label={ts("请求体 JSON")} spellCheck={false} />}
              {result && <pre className="mcdl-stage mt-3 max-h-96 overflow-auto p-4 text-[0.75rem] leading-6 text-[#f5efeb]" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{result}</pre>}
            </section>

            <section className="mcdl-editorial-card p-5">
              <h2 className="text-[1.15rem]">{ts("错误")}</h2>
              <div className="mt-4 grid gap-3 md:grid-cols-2">
                {ERRORS.map((item) => (
                  <article key={item.status} className="rounded-[1.1rem] bg-[var(--mcdl-color-surface-subtle)] p-4">
                    <div className="flex items-baseline gap-3">
                      <span className="text-[1.4rem] text-[var(--mcdl-color-negative)]" style={{ fontFamily: "var(--mcdl-font-numeric)" }}>{item.status}</span>
                      <code className="text-[0.75rem] text-[var(--mcdl-text-body)]" style={{ fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace" }}>{item.code}</code>
                    </div>
                    <p className="mt-2 text-[0.8rem] leading-6 text-[var(--mcdl-text-body)]">{ts(item.help)}</p>
                  </article>
                ))}
              </div>
              <Link href="/app/recharge" className="mt-4 inline-flex text-[0.8rem] text-[var(--mcdl-color-accent)]">{ts("余额")}</Link>
            </section>
            {activeSection.id === "start" && <ReferralShareButton variant="card" />}
          </div>
        )}
      </main>
    </div>
  );
}

function Conventions() {
  const { ts } = useI18n();
  const items = [
    ["鉴权", "除另有说明外，使用 Authorization: Bearer <API_KEY>。Key 在设置中创建，前缀为 sk-，完整值只返回一次。"],
    ["模型", "model 与 Gemini 路径参数都使用模型 ID。展示名称不能作为请求值。GET /v1/models 的 data[].id 与此处一致。"],
    ["文本响应", "OpenAI 兼容接口在 stream=false 时返回 content，不返回 choices。官方 SDK 的非流式解析不适用，流式响应为 SSE。"],
    ["异步任务", "图像、视频和音频创建接口返回 task_no。轮询 GET /v1/tasks/{task_no}，status=succeeded 后读取 output。"],
  ];
  return (
    <div className="grid gap-3 sm:grid-cols-2">
      {items.map(([title, detail]) => (
        <article key={title} className="mcdl-editorial-card p-4">
          <h2 className="text-[1rem]">{ts(title)}</h2>
          <p className="mt-2 text-[0.8rem] leading-6 text-[var(--mcdl-text-body)]">{ts(detail)}</p>
        </article>
      ))}
    </div>
  );
}

function DocsSkeleton() {
  const { ts } = useI18n();
  return (
    <div className="mx-auto w-full max-w-[76rem] px-4 py-8 sm:px-6">
      <p className="sr-only" role="status">{ts("正在加载文档")}</p>
      <div aria-hidden="true">
      <div className="h-4 w-24 rounded-full bg-[var(--mcdl-color-surface-subtle)]" />
      <div className="mt-4 h-12 w-2/3 rounded-2xl bg-[var(--mcdl-color-surface-subtle)]" />
      <div className="mt-6 grid gap-3 sm:grid-cols-2">
        <div className="h-36 rounded-[1.75rem] bg-[var(--mcdl-color-surface-card)]" />
        <div className="h-36 rounded-[1.75rem] bg-[var(--mcdl-color-surface-card)]" />
      </div>
      </div>
    </div>
  );
}

function defaultBaseUrl() {
  const configured = (API_URL || "").trim().replace(/\/+$/, "");
  if (configured && !/your-yihangai-domain/i.test(configured)) return configured;
  if (typeof window !== "undefined" && window.location.origin) return window.location.origin.replace(/\/+$/, "");
  return "http://localhost:8080";
}

function sectionMatches(id: SectionId, label: string, query: string, models: DocModel[]) {
  if (!query) return true;
  if (label.toLowerCase().includes(query)) return true;
  if (id === "start" && ["base url", "鉴权", "bearer", "概述"].some((word) => query.includes(word))) return true;
  if ((id === "task" || id === "events") && ["task", "任务", "事件"].some((word) => query.includes(word))) return true;
  if (id === "models" && "模型 列表 id".includes(query)) return true;
  return models.some((model) => model.section === id && [model.name, model.modelId, model.description].join(" ").toLowerCase().includes(query));
}
