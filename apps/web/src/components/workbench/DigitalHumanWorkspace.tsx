"use client";

import { FormEvent, useEffect, useRef, useState } from "react";
import { useI18n } from "@/i18n/I18nProvider";
import { api } from "@/lib/api";

const RELATIONS = ["父亲", "母亲", "伴侣", "朋友", "兄弟", "姐妹", "祖父", "祖母", "助理", "其他"];
const RTC_SDK = "https://g.alicdn.com/apsara-media-box/imp-web-rtc/7.1.9/aliyun-rtc-sdk.js";

type RoleSummary = {
  public_id: string;
  name: string;
  relation: string;
  user_title: string;
  avatar_url: string;
  knowledge_title: string;
};

type ChatMessage = { id?: number; speaker: string; content: string; created_at?: string };
type KnowledgeDoc = { id: number; title: string; content: string };
type RtcInfo = { app_id?: string; channel_id?: string; user_id?: string; token?: string; token_expire_at?: string };
type LiveSession = { public_id: string; call_mode: string; status: string; cost?: number; rtc?: RtcInfo };
type RoleDetail = RoleSummary & {
  memory: string;
  style: string;
  persona: string;
  voice: string;
  messages?: ChatMessage[];
  knowledge?: KnowledgeDoc[];
  active_session?: LiveSession | null;
};

type Draft = {
  name: string;
  relation: string;
  user_title: string;
  memory: string;
  style: string;
  persona: string;
  voice: string;
  avatar_url: string;
  knowledge_title: string;
};

const emptyDraft = (): Draft => ({
  name: "",
  relation: "朋友",
  user_title: "",
  memory: "",
  style: "",
  persona: "",
  voice: "Tina",
  avatar_url: "",
  knowledge_title: "",
});

function draftFromRole(role: RoleDetail): Draft {
  return {
    name: role.name || "",
    relation: role.relation || "朋友",
    user_title: role.user_title || "",
    memory: role.memory || "",
    style: role.style || "",
    persona: role.persona || "",
    voice: role.voice || "Tina",
    avatar_url: role.avatar_url || "",
    knowledge_title: role.knowledge_title || "",
  };
}

async function loadRtcEngine() {
  const host = window as Window & { AliRtcEngine?: { getInstance: () => RtcEngine } };
  if (host.AliRtcEngine) return host.AliRtcEngine;
  await new Promise<void>((resolve, reject) => {
    const found = document.querySelector<HTMLScriptElement>("script[data-ali-rtc='1']");
    if (found) {
      found.addEventListener("load", () => resolve(), { once: true });
      found.addEventListener("error", () => reject(new Error("语音组件加载失败")), { once: true });
      return;
    }
    const script = document.createElement("script");
    script.src = RTC_SDK;
    script.async = true;
    script.dataset.aliRtc = "1";
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("语音组件加载失败"));
    document.body.appendChild(script);
  });
  if (!host.AliRtcEngine) throw new Error("语音组件没有就绪");
  return host.AliRtcEngine;
}

type RtcEngine = {
  setDefaultSubscribeAllRemoteAudioStreams?: (enabled: boolean) => Promise<void> | void;
  setDefaultSubscribeAllRemoteVideoStreams?: (enabled: boolean) => Promise<void> | void;
  subscribeAllRemoteVideoStreams?: (enabled: boolean) => Promise<void> | void;
  subscribeRemoteVideoStream?: (userId: string, subscribe: boolean, track?: number) => Promise<void> | void;
  setChannelProfile?: (profile: string) => Promise<void> | void;
  joinChannel: (...args: unknown[]) => Promise<void>;
  publishLocalAudioStream?: (enabled: boolean) => Promise<void> | void;
  publishLocalVideoStream?: (enabled: boolean) => Promise<void> | void;
  setLocalViewConfig?: (element: HTMLVideoElement, streamType: number) => Promise<void> | void;
  setRemoteViewConfig?: (element: HTMLVideoElement | null, userId: string, streamType: number) => void;
  on?: (event: string, handler: (...args: unknown[]) => void) => void;
  leaveChannel?: () => Promise<void> | void;
};

function rtcSubscribed(value: unknown) {
  return value === 3 || value === "3" || value === "subscribed";
}

export function DigitalHumanWorkspace() {
  const { ts } = useI18n();
  const [roles, setRoles] = useState<RoleSummary[]>([]);
  const [roleId, setRoleId] = useState("");
  const [draft, setDraft] = useState<Draft>(emptyDraft);
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [knowledge, setKnowledge] = useState<KnowledgeDoc[]>([]);
  const [session, setSession] = useState<LiveSession | null>(null);
  const [query, setQuery] = useState("");
  const [text, setText] = useState("");
  const [docTitle, setDocTitle] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [prices, setPrices] = useState({ voice: 100, video: 3500 });
  const [creating, setCreating] = useState(false);
  const [videoStage, setVideoStage] = useState<"idle" | "connecting" | "live">("idle");
  const [micOn, setMicOn] = useState(true);
  const [cameraOn, setCameraOn] = useState(true);
  const [elapsedSec, setElapsedSec] = useState(0);
  const [remoteReady, setRemoteReady] = useState(false);
  const sessionRef = useRef<LiveSession | null>(null);
  const engineRef = useRef<RtcEngine | null>(null);
  const listRef = useRef<HTMLDivElement | null>(null);
  const remoteVideoRef = useRef<HTMLVideoElement | null>(null);
  const localVideoRef = useRef<HTMLVideoElement | null>(null);
  const videoJoinRef = useRef<RtcInfo | null>(null);
  const joinedVideoRef = useRef("");
  const previewStreamRef = useRef<MediaStream | null>(null);

  const stopLocalPreview = () => {
    previewStreamRef.current?.getTracks().forEach((track) => track.stop());
    previewStreamRef.current = null;
    if (localVideoRef.current?.srcObject) localVideoRef.current.srcObject = null;
  };

  useEffect(() => {
    sessionRef.current = session;
  }, [session]);

  useEffect(() => {
    if (videoStage === "idle") {
      stopLocalPreview();
      return;
    }
    if (previewStreamRef.current || !navigator.mediaDevices?.getUserMedia) return;
    let cancelled = false;
    navigator.mediaDevices.getUserMedia({ video: { facingMode: "user" }, audio: false }).then((media) => {
      if (cancelled || engineRef.current) {
        media.getTracks().forEach((track) => track.stop());
        return;
      }
      previewStreamRef.current = media;
      const view = localVideoRef.current;
      if (!view) return;
      view.srcObject = media;
      void view.play().catch(() => undefined);
    }).catch(() => undefined);
    return () => {
      cancelled = true;
    };
  }, [videoStage]);

  useEffect(() => {
    api<{ voice_price_per_minute: number; video_price_per_minute: number }>("/api/digital-human/prices")
      .then((item) => setPrices({ voice: item.voice_price_per_minute, video: item.video_price_per_minute }))
      .catch(() => undefined);
    api<RoleSummary[]>("/api/digital-human/roles")
      .then((items) => {
        setRoles(items);
        if (items[0]) setRoleId(items[0].public_id);
      })
      .catch((err: Error) => setNotice(err.message));
    return () => {
      const current = sessionRef.current;
      void engineRef.current?.leaveChannel?.();
      if (current?.public_id) {
        void api(`/api/digital-human/sessions/${current.public_id}/end`, { method: "POST" }).catch(() => undefined);
      }
    };
  }, []);

  useEffect(() => {
    if (!roleId || creating) return;
    let stop = false;
    api<RoleDetail>(`/api/digital-human/roles/${roleId}`)
      .then((role) => {
        if (stop) return;
        setDraft(draftFromRole(role));
        setMessages(role.messages || []);
        setKnowledge(role.knowledge || []);
        setSession(role.active_session || null);
      })
      .catch((err: Error) => setNotice(err.message));
    return () => {
      stop = true;
    };
  }, [creating, roleId]);

  useEffect(() => {
    if (!session?.public_id || !roleId) return;
    const timer = window.setInterval(() => {
      api<RoleDetail>(`/api/digital-human/roles/${roleId}`)
        .then((role) => {
          setMessages(role.messages || []);
          setSession(role.active_session || null);
        })
        .catch(() => undefined);
    }, 2000);
    return () => window.clearInterval(timer);
  }, [roleId, session?.public_id]);

  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [messages]);

  const visibleRoles = roles.filter((role) => {
    const q = query.trim();
    return !q || role.name.includes(q) || role.relation.includes(q);
  });

  const patch = (key: keyof Draft, value: string) => setDraft((prev) => ({ ...prev, [key]: value }));

  const saveRole = async () => {
    setBusy("保存中");
    setNotice("");
    try {
      const saved = await api<RoleDetail>(!roleId || creating ? "/api/digital-human/roles" : `/api/digital-human/roles/${roleId}`, {
        method: !roleId || creating ? "POST" : "PUT",
        body: JSON.stringify(draft),
      });
      setCreating(false);
      setRoleId(saved.public_id);
      setRoles(await api<RoleSummary[]>("/api/digital-human/roles"));
      setNotice(ts("角色已保存"));
    } catch (err) {
      setNotice(err instanceof Error ? ts(err.message) : ts("保存失败"));
    } finally {
      setBusy("");
    }
  };

  const removeRole = async () => {
    if (!roleId || !window.confirm(ts("删除这个角色？对话和知识库会一起删除。"))) return;
    setBusy("删除中");
    try {
      await api(`/api/digital-human/roles/${roleId}`, { method: "DELETE" });
      const items = await api<RoleSummary[]>("/api/digital-human/roles");
      setRoles(items);
      setCreating(false);
      setRoleId(items[0]?.public_id || "");
      if (!items[0]) {
        setDraft(emptyDraft());
        setMessages([]);
        setKnowledge([]);
      }
    } catch (err) {
      setNotice(err instanceof Error ? ts(err.message) : ts("删除失败"));
    } finally {
      setBusy("");
    }
  };

  const bindRemoteVideo = (engine: RtcEngine, userId: string, track: number) => {
    const remote = remoteVideoRef.current;
    if (!remote || !userId || userId === "undefined") return;
    engine.setRemoteViewConfig?.(remote, userId, track);
    remote.dataset.boundUser = userId;
    void remote.play().catch(() => undefined);
  };

  const subscribeRemote = async (engine: RtcEngine, userId: string) => {
    if (!userId || userId === "undefined") return;
    try {
      await engine.subscribeRemoteVideoStream?.(userId, true, 1);
    } catch { /* 相机流可能还没推上来 */ }
    try {
      await engine.subscribeRemoteVideoStream?.(userId, true, 2);
    } catch { /* 屏幕流可能还没推上来 */ }
    bindRemoteVideo(engine, userId, 1);
    bindRemoteVideo(engine, userId, 2);
  };

  const joinMedia = async (rtc: RtcInfo | undefined, withVideo: boolean) => {
    if (!rtc?.token || !rtc.user_id) throw new Error("没有拿到通话房间凭证");
    const sdk = await loadRtcEngine();
    const engine = sdk.getInstance();
    engineRef.current = engine;
    await engine.setChannelProfile?.("communication");
    await engine.setDefaultSubscribeAllRemoteAudioStreams?.(true);
    if (withVideo) {
      await engine.setDefaultSubscribeAllRemoteVideoStreams?.(true);
      await engine.subscribeAllRemoteVideoStreams?.(true);
    }
    const onVideo = (userId: unknown, _oldState: unknown, newState: unknown) => {
      if (!withVideo || userId == null) return;
      if (rtcSubscribed(newState) || newState === 2 || newState === "2") bindRemoteVideo(engine, String(userId), 1);
    };
    const onScreen = (userId: unknown, _oldState: unknown, newState: unknown) => {
      if (!withVideo || userId == null) return;
      if (rtcSubscribed(newState) || newState === 2 || newState === "2") bindRemoteVideo(engine, String(userId), 2);
    };
    engine.on?.("videoSubscribeStateChanged", onVideo);
    engine.on?.("screenShareSubscribeStateChanged", onScreen);
    engine.on?.("remoteUserOnLineNotify", (userId) => {
      if (withVideo) void subscribeRemote(engine, String(userId || ""));
    });
    if (withVideo && localVideoRef.current) {
      stopLocalPreview();
      await engine.setLocalViewConfig?.(localVideoRef.current, 1);
    }
    const expire = Number(rtc.token_expire_at);
    try {
      await engine.joinChannel({
        appId: rtc.app_id,
        channelId: rtc.channel_id,
        userId: rtc.user_id,
        token: rtc.token,
        timestamp: Number.isFinite(expire) ? expire : undefined,
      }, rtc.user_id);
    } catch {
      await engine.joinChannel(rtc.token, rtc.user_id);
    }
    await engine.publishLocalAudioStream?.(true);
    if (withVideo) {
      await engine.publishLocalVideoStream?.(true);
      window.setTimeout(() => {
        const listed = (engine as RtcEngine & { getOnlineRemoteUsers?: () => string[] }).getOnlineRemoteUsers?.() || [];
        listed.forEach((userId) => void subscribeRemote(engine, String(userId)));
      }, 1500);
    }
    setMicOn(true);
    setCameraOn(withVideo);
  };

  const startSession = async (mode: "text" | "audio" | "video") => {
    if (!roleId) throw new Error("请先保存角色");
    if (session?.public_id && session.call_mode !== mode) {
      await api(`/api/digital-human/sessions/${session.public_id}/end`, { method: "POST" });
      await engineRef.current?.leaveChannel?.();
      engineRef.current = null;
    } else if (session?.public_id) {
      return session;
    }
    const started = await api<LiveSession>(`/api/digital-human/roles/${roleId}/sessions`, {
      method: "POST",
      body: JSON.stringify({ mode }),
    });
    setSession(started);
    if (mode === "audio") {
      try {
        await joinMedia(started.rtc, false);
      } catch (err) {
        await api(`/api/digital-human/sessions/${started.public_id}/end`, { method: "POST" }).catch(() => undefined);
        setSession(null);
        throw err;
      }
    }
    if (mode === "video") videoJoinRef.current = started.rtc || null;
    return started;
  };

  useEffect(() => {
    const rtc = videoJoinRef.current;
    const id = session?.public_id || "";
    if (videoStage !== "connecting" || !rtc || !id || joinedVideoRef.current === id) return;
    if (!remoteVideoRef.current || !localVideoRef.current) return;
    joinedVideoRef.current = id;
    videoJoinRef.current = null;
    joinMedia(rtc, true)
      .then(() => setVideoStage("live"))
      .catch(async (err: unknown) => {
        joinedVideoRef.current = "";
        const current = sessionRef.current;
        await engineRef.current?.leaveChannel?.();
        engineRef.current = null;
        if (current?.public_id) {
          await api(`/api/digital-human/sessions/${current.public_id}/end`, { method: "POST" }).catch(() => undefined);
        }
        setSession(null);
        setVideoStage("idle");
        setNotice(err instanceof Error ? ts(err.message) : ts("视频连接失败"));
      });
  }, [session?.public_id, videoStage]);

  const hangup = async () => {
    const current = sessionRef.current;
    setVideoStage("idle");
    videoJoinRef.current = null;
    joinedVideoRef.current = "";
    if (!current?.public_id) {
      await engineRef.current?.leaveChannel?.();
      engineRef.current = null;
      return;
    }
    setBusy("结束中");
    try {
      await engineRef.current?.leaveChannel?.();
      engineRef.current = null;
      const ended = await api<LiveSession>(`/api/digital-human/sessions/${current.public_id}/end`, { method: "POST" });
      setSession(null);
      setNotice(ended.cost ? ts("通话已结束，本次消耗 {cost} 算力").replace("{cost}", String(ended.cost)) : ts("通话已结束"));
    } catch (err) {
      setNotice(err instanceof Error ? ts(err.message) : ts("结束失败"));
    } finally {
      setBusy("");
    }
  };

  const sendText = async (event?: FormEvent) => {
    event?.preventDefault();
    const content = text.trim();
    if (!content || busy) return;
    setBusy("发送中");
    setNotice("");
    setText("");
    try {
      const live = session?.public_id ? session : await startSession("text");
      const result = await api<{ user: ChatMessage; assistant: ChatMessage | null }>(`/api/digital-human/sessions/${live.public_id}/messages`, {
        method: "POST",
        body: JSON.stringify({ content }),
      });
      setMessages((prev) => [...prev, result.user, ...(result.assistant ? [result.assistant] : [])]);
      if (!result.assistant) setNotice(ts("文字已送出。如果这是语音通话，直接听对方说话即可。"));
    } catch (err) {
      setText(content);
      setNotice(err instanceof Error ? ts(err.message) : ts("发送失败"));
    } finally {
      setBusy("");
    }
  };

  const startVoice = async () => {
    setBusy("连接中");
    setNotice("");
    try {
      await startSession("audio");
      setNotice(ts("语音已接通，可以直接说话"));
    } catch (err) {
      setNotice(err instanceof Error ? ts(err.message) : ts("语音连接失败"));
    } finally {
      setBusy("");
    }
  };

  const addKnowledge = async (content: string, title: string) => {
    if (!roleId) {
      setNotice(ts("请先保存角色，再添加知识库"));
      return;
    }
    setBusy("添加资料");
    try {
      const doc = await api<KnowledgeDoc>(`/api/digital-human/roles/${roleId}/knowledge`, {
        method: "POST",
        body: JSON.stringify({ title, content }),
      });
      setKnowledge((prev) => [...prev, doc]);
      setDocTitle("");
    } catch (err) {
      setNotice(err instanceof Error ? ts(err.message) : ts("资料添加失败"));
    } finally {
      setBusy("");
    }
  };

  const onFile = async (file: File | undefined) => {
    if (!file) return;
    if (!/\.(txt|md|json)$/i.test(file.name) && !file.type.startsWith("text/")) {
      setNotice(ts("这一期只接收 TXT、MD、JSON 文本"));
      return;
    }
    const content = await file.text();
    await addKnowledge(content, docTitle.trim() || file.name.replace(/\.[^.]+$/, ""));
  };

  const startVideo = async () => {
    setBusy("连接中");
    setNotice("");
    setVideoStage("connecting");
    try {
      await startSession("video");
    } catch (err) {
      setVideoStage("idle");
      setNotice(err instanceof Error ? ts(err.message) : ts("视频连接失败"));
    } finally {
      setBusy("");
    }
  };

  const toggleMic = async () => {
    const next = !micOn;
    await engineRef.current?.publishLocalAudioStream?.(next);
    setMicOn(next);
  };

  const toggleCamera = async () => {
    const next = !cameraOn;
    if (engineRef.current?.publishLocalVideoStream) {
      await engineRef.current.publishLocalVideoStream(next);
    } else {
      previewStreamRef.current?.getVideoTracks().forEach((track) => {
        track.enabled = next;
      });
    }
    setCameraOn(next);
  };

  useEffect(() => {
    if (videoStage !== "live") {
      setElapsedSec(0);
      setRemoteReady(false);
      return;
    }
    const started = Date.now();
    const timer = window.setInterval(() => setElapsedSec(Math.floor((Date.now() - started) / 1000)), 1000);
    return () => window.clearInterval(timer);
  }, [videoStage]);

  const status = videoStage !== "idle" ? "视频通话中" : session?.call_mode === "audio" ? "语音通话中" : session ? "控制链路已就绪" : "已结束";
  const elapsedLabel = `${String(Math.floor(elapsedSec / 60)).padStart(2, "0")}:${String(elapsedSec % 60).padStart(2, "0")}`;
  const estimatedCost = Math.round((elapsedSec / 60) * prices.video * 100) / 100;

  return (
    <div className="flex h-full min-h-0 flex-col bg-[#F6F7FB] text-gray-900 dark:bg-gray-950 dark:text-gray-100">
      <header className="flex h-[73px] shrink-0 items-center gap-3 border-b border-gray-100 bg-white px-4 dark:border-white/10 dark:bg-gray-900">
        <div className="flex items-center gap-2">
          <h1 className="text-base font-semibold">{ts("实时数字人对话")}</h1>
          <span className={`text-xs ${status === "已结束" ? "text-red-500" : "text-gray-500"}`}>{ts(status)}</span>
        </div>
      </header>
      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[240px_minmax(0,1fr)_320px]">
        <aside className="flex min-h-0 flex-col border-b border-gray-200 bg-white dark:border-white/10 dark:bg-gray-900 lg:border-b-0 lg:border-r">
          <div className="flex items-center justify-between px-3 py-3">
            <span className="text-sm font-medium">{ts("角色列表")}</span>
            <button
              type="button"
              className="rounded-md bg-cyan-600 px-2 py-1 text-xs text-white"
              onClick={() => {
                setCreating(true);
                setRoleId("");
                setSession(null);
                setMessages([]);
                setKnowledge([]);
                setDraft(emptyDraft());
              }}
            >
              {ts("新建角色")}
            </button>
          </div>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={ts("搜索角色")} className="mx-3 mb-2 rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
            {visibleRoles.map((role) => (
              <button
                key={role.public_id}
                type="button"
                onClick={() => {
                  if (sessionRef.current?.public_id) {
                    void engineRef.current?.leaveChannel?.();
                    void api(`/api/digital-human/sessions/${sessionRef.current.public_id}/end`, { method: "POST" }).catch(() => undefined);
                    engineRef.current = null;
                    sessionRef.current = null;
                    setSession(null);
                    setVideoStage("idle");
                    videoJoinRef.current = null;
                    joinedVideoRef.current = "";
                  }
                  setCreating(false);
                  setRoleId(role.public_id);
                }}
                className={`mb-1 flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm ${role.public_id === roleId && !creating ? "bg-cyan-50 text-cyan-800 dark:bg-cyan-950/40" : "hover:bg-gray-50 dark:hover:bg-white/5"}`}
              >
                <span className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-100 text-xs dark:bg-white/10">
                  {role.avatar_url ? <img src={role.avatar_url} alt="" className="h-full w-full object-cover" /> : role.name.slice(0, 1) || ts("人")}
                </span>
                <span className="min-w-0">
                  <span className="block truncate font-medium">{role.name || ts("未命名")}</span>
                  <span className="block truncate text-[11px] text-gray-400">{role.relation ? ts(role.relation) : ts("未设置关系")}</span>
                </span>
              </button>
            ))}
            {visibleRoles.length === 0 && <div className="px-2 py-6 text-center text-xs text-gray-400">{ts("还没有角色")}</div>}
          </div>
        </aside>

        <section className="relative flex min-h-0 flex-col">
          <div className="border-b border-gray-200 px-4 py-2 text-[11px] text-gray-500 dark:border-white/10">
            {ts("文字按会话消耗计费，语音")} {prices.voice} {ts("算力/分钟，视频")} {prices.video} {ts("算力/分钟")}
          </div>
          <div ref={listRef} className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-4">
            {messages.map((message, index) => (
              <div key={`${message.id || index}-${message.created_at || index}`} className={`flex ${message.speaker === "user" ? "justify-end" : "justify-start"}`}>
                <div className={`max-w-[80%] rounded-2xl px-3 py-2 text-sm leading-6 ${message.speaker === "user" ? "bg-cyan-600 text-white" : "bg-white text-gray-800 dark:bg-gray-900 dark:text-gray-100"}`}>
                  {message.content}
                </div>
              </div>
            ))}
            {messages.length === 0 && <div className="pt-16 text-center text-sm text-gray-400">{ts("像微信一样和 Ta 说点什么")}</div>}
          </div>
          {notice && <div className="px-4 pb-2 text-xs text-amber-700">{notice}</div>}
          <form onSubmit={sendText} className="flex items-end gap-2 border-t border-gray-200 bg-white p-3 dark:border-white/10 dark:bg-gray-900">
            <textarea
              value={text}
              onChange={(event) => setText(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter" && !event.shiftKey) {
                  event.preventDefault();
                  void sendText();
                }
              }}
              rows={2}
              placeholder={ts("像微信一样和 Ta 说点什么...")}
              className="min-w-0 flex-1 resize-none rounded-xl border border-gray-200 px-3 py-2 text-sm outline-none dark:border-white/10 dark:bg-gray-950"
            />
            <div className="flex flex-col gap-2">
              <button type="button" disabled={!!busy || videoStage !== "idle"} onClick={() => void (session?.call_mode === "audio" ? hangup() : startVoice())} className="rounded-lg bg-gray-900 px-3 py-1.5 text-xs text-white disabled:opacity-50 dark:bg-white dark:text-gray-900">
                {session?.call_mode === "audio" ? ts("挂断") : ts("语音通话")}
              </button>
              <button type="button" disabled={!!busy || videoStage !== "idle"} onClick={() => void startVideo()} className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs disabled:opacity-50 dark:border-white/10">
                {videoStage === "connecting" ? ts("接通中") : ts("视频通话")}
              </button>
              <button type="submit" disabled={!!busy || !text.trim()} className="rounded-lg bg-cyan-600 px-3 py-1.5 text-xs text-white disabled:opacity-50">{ts("发送")}</button>
            </div>
          </form>
          {videoStage !== "idle" && (
            <div className="absolute inset-0 z-20 flex flex-col bg-black text-white">
              <div className="flex items-center justify-between px-4 py-3 text-sm">
                <span>{draft.name || ts("数字人")}</span>
                <span className="text-xs text-white/80">
                  {videoStage === "live" ? `${ts("已连接")} ${elapsedLabel} · ${ts("预计")} ${estimatedCost} ${ts("算力")}` : ts("正在接通")}
                </span>
              </div>
              <div className="flex min-h-0 flex-1 items-center justify-center px-4">
                <div className="relative h-full max-h-full overflow-hidden rounded-[28px] bg-zinc-950" style={{ aspectRatio: "9 / 16", width: "auto", maxWidth: "100%" }}>
                  {draft.avatar_url && (
                    <img src={draft.avatar_url} alt="" className={`absolute inset-0 h-full w-full object-cover ${remoteReady ? "opacity-0" : "opacity-100"}`} />
                  )}
                  <video ref={remoteVideoRef} autoPlay playsInline onPlaying={() => setRemoteReady(true)} className={`absolute inset-0 h-full w-full bg-transparent object-contain ${remoteReady ? "opacity-100" : "opacity-0"}`} />
                  {!remoteReady && (
                    <div className="pointer-events-none absolute inset-0 flex items-center justify-center px-6">
                      <span className="text-2xl font-medium tracking-wide text-white" style={{ textShadow: "0 2px 16px rgba(0,0,0,0.9), 0 1px 2px rgba(0,0,0,0.8)" }}>{ts("正在呼叫...")}</span>
                    </div>
                  )}
                  <div className="absolute right-3 top-3 z-10 h-28 w-[4.5rem] overflow-hidden rounded-2xl border border-white/40 bg-zinc-900 shadow-lg">
                    <video ref={localVideoRef} autoPlay playsInline muted className={`h-full w-full object-cover ${cameraOn ? "-scale-x-100" : "hidden"}`} />
                    {!cameraOn && <div className="flex h-full items-center justify-center px-2 text-center text-[11px] text-white/70">{ts("摄像头已关闭")}</div>}
                  </div>
                </div>
              </div>
              <div className="flex items-center justify-center gap-8 px-4 py-5">
                <button type="button" onClick={() => void toggleMic()} className="flex h-14 w-16 items-center justify-center rounded-full bg-white/15 px-1 text-center text-[11px] leading-tight">
                  {micOn ? ts("闭麦") : ts("开麦")}
                </button>
                <button type="button" onClick={() => void hangup()} className="flex h-16 w-16 items-center justify-center rounded-full bg-red-500 text-sm font-medium">
                  {ts("挂断")}
                </button>
                <button type="button" onClick={() => void toggleCamera()} className="flex h-14 w-16 items-center justify-center rounded-full bg-white/15 px-1 text-center text-[11px] leading-tight">
                  {cameraOn ? ts("关闭摄像头") : ts("打开摄像头")}
                </button>
              </div>
            </div>
          )}
        </section>

        <aside className="flex min-h-0 flex-col border-t border-gray-200 bg-white dark:border-white/10 dark:bg-gray-900 lg:border-l lg:border-t-0">
          <div className="min-h-0 flex-1 overflow-y-auto p-4">
          <div className="mb-3 text-sm font-medium">{creating ? ts("新建角色") : ts("编辑当前角色")}</div>
          <label className="mb-2 block text-xs text-gray-500">{ts("名字")}
            <input value={draft.name} onChange={(event) => patch("name", event.target.value)} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <div className="mb-2 text-xs text-gray-500">{ts("关系")}</div>
          <div className="mb-3 flex flex-wrap gap-1">
            {RELATIONS.map((item) => (
              <button key={item} type="button" onClick={() => patch("relation", item)} className={`rounded-full px-2 py-1 text-[11px] ${draft.relation === item ? "bg-cyan-600 text-white" : "bg-gray-100 text-gray-600 dark:bg-white/10"}`}>{ts(item)}</button>
            ))}
          </div>
          <label className="mb-2 block text-xs text-gray-500">{ts("你对用户的称呼")}
            <input value={draft.user_title} onChange={(event) => patch("user_title", event.target.value)} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <label className="mb-2 block text-xs text-gray-500">{ts("人设撰写")}
            <textarea value={draft.persona} onChange={(event) => patch("persona", event.target.value)} rows={4} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <label className="mb-2 block text-xs text-gray-500">{ts("核心记忆")}
            <textarea value={draft.memory} onChange={(event) => patch("memory", event.target.value)} rows={3} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <label className="mb-2 block text-xs text-gray-500">{ts("说话风格")}
            <textarea value={draft.style} onChange={(event) => patch("style", event.target.value)} rows={2} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <label className="mb-2 block text-xs text-gray-500">{ts("音色")}
            <input value={draft.voice} onChange={(event) => patch("voice", event.target.value)} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <p className="mb-3 text-[11px] leading-5 text-gray-400">{ts("预设音色 Tina。克隆音色即将开放。")}</p>
          <label className="mb-2 block text-xs text-gray-500">{ts("头像 URL")}
            <input value={draft.avatar_url} onChange={(event) => patch("avatar_url", event.target.value)} placeholder={ts("https:// 公网图片地址")} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <label className="mb-3 block text-xs text-gray-500">{ts("知识库标题")}
            <input value={draft.knowledge_title} onChange={(event) => patch("knowledge_title", event.target.value)} className="mt-1 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          </label>
          <div className="border-t border-gray-100 pt-3 dark:border-white/10">
            <div className="mb-2 text-sm font-medium">{ts("知识库")}</div>
            <p className="mb-2 text-[11px] leading-5 text-gray-400">{ts("上传 TXT、MD 或 JSON。通话中数字人只会检索这个角色的资料。后台需要填写 Vidu 能访问的公网地址。")}</p>
            <input value={docTitle} onChange={(event) => setDocTitle(event.target.value)} placeholder={ts("资料标题，可留空用文件名")} className="mb-2 w-full rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
            <input type="file" accept=".txt,.md,.json,text/plain" onChange={(event) => void onFile(event.target.files?.[0])} className="mb-2 block w-full text-xs" />
            <ul className="space-y-1">
              {knowledge.map((doc) => (
                <li key={doc.id} className="flex items-center justify-between gap-2 rounded-md bg-gray-50 px-2 py-1.5 text-xs dark:bg-white/5">
                  <span className="truncate">{doc.title}</span>
                  <button
                    type="button"
                    className="text-red-500"
                    onClick={() => void api(`/api/digital-human/roles/${roleId}/knowledge/${doc.id}`, { method: "DELETE" }).then(() => setKnowledge((prev) => prev.filter((item) => item.id !== doc.id))).catch((err: Error) => setNotice(err.message))}
                  >
                    {ts("移除")}
                  </button>
                </li>
              ))}
            </ul>
          </div>
          </div>
          <div className="flex shrink-0 gap-2 border-t border-gray-200 p-3 dark:border-white/10">
            <button type="button" disabled={!!busy} onClick={() => void saveRole()} className="min-w-0 flex-1 rounded-lg bg-cyan-600 px-3 py-2 text-sm text-white disabled:opacity-50">{busy === "保存中" ? ts("保存中") : ts("保存角色")}</button>
            {!creating && roleId && <button type="button" onClick={() => void removeRole()} className="rounded-lg border border-red-200 px-3 py-2 text-xs text-red-600">{ts("删除")}</button>}
            {session && <button type="button" onClick={() => void hangup()} className="rounded-lg border border-gray-200 px-3 py-2 text-xs">{ts("结束通话")}</button>}
          </div>
        </aside>
      </div>
    </div>
  );
}
