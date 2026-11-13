"use client";

import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import {
  Aperture,
  CameraOff,
  Check,
  ChevronRight,
  Contrast,
  Download,
  Film,
  Info,
  Layers,
  Loader2,
  Mic,
  MicOff,
  PhoneOff,
  Plus,
  Radio,
  RefreshCw,
  ShieldCheck,
  Sparkles,
  Upload,
  VenetianMask,
  X,
} from "lucide-react";
import { useI18n } from "@/i18n/I18nProvider";
import {
  addCharacterStudioTake,
  createCharacterStudioCharacter,
  deleteCharacterStudioCharacter,
  endCharacterStudioSession,
  fetchCharacterStudioConfig,
  getActiveCharacterStudioSession,
  getCharacterStudioSession,
  heartbeatCharacterStudioSession,
  listCharacterStudioCharacters,
  startCharacterStudioSession,
  switchCharacterStudioCharacter,
  updateCharacterStudioCharacter,
  uploadCharacterStudioImage,
  uploadCharacterStudioTake,
  type CharacterStudioCharacter,
  type CharacterStudioConfig,
  type CharacterStudioSession,
  type CharacterStudioTake,
} from "@/lib/characterStudio";
import { uploadAsset } from "@/lib/api";
import { CHARACTER_STUDIO_DEFAULT_CATEGORY_LABELS, CHARACTER_STUDIO_DEFAULT_ROLES, type CharacterStudioDefaultRole } from "@/lib/characterStudioDefaults";
import { createRenderStage, preloadRenderStage, type RenderStage } from "@/lib/realtime/renderStage";

type Phase = "boot" | "idle" | "starting" | "live" | "wrapping" | "wrapped" | "failed";
type CameraState = "idle" | "requesting" | "ready" | "error";
type RecordingState = "idle" | "recording" | "uploading";

type DisplayTake = {
  key: string;
  publicId?: string;
  url: string;
  thumb?: string;
  durationMs: number;
  sizeBytes: number;
  characterName: string;
  status: "uploading" | "saved" | "local" | "failed";
};

type SwitchLogEntry = { key: string; name: string; at: number };

type CharacterDraft = {
  publicId?: string;
  name: string;
  imageURL: string;
  rights: boolean;
};

const EMPTY_DRAFT: CharacterDraft = { name: "", imageURL: "", rights: false };
const RESOLUTION_LABEL: Record<string, string> = { "540p": "流畅", "720p": "高清", "1080p": "超清" };
const RESOLUTION_DETAIL: Record<string, string> = { "540p": "540p", "720p": "720p", "1080p": "1080p" };

let pendingPageEndTimer: ReturnType<typeof setTimeout> | null = null;

function clearPendingPageEnd() {
  if (pendingPageEndTimer) {
    clearTimeout(pendingPageEndTimer);
    pendingPageEndTimer = null;
  }
}

function schedulePageEnd(publicId: string) {
  clearPendingPageEnd();
  pendingPageEndTimer = setTimeout(() => {
    pendingPageEndTimer = null;
    void endCharacterStudioSession(publicId).catch(() => undefined);
  }, 1200);
}

function formatClock(totalSeconds: number) {
  const seconds = Math.max(0, Math.floor(totalSeconds || 0));
  return `${String(Math.floor(seconds / 60)).padStart(2, "0")}:${String(seconds % 60).padStart(2, "0")}`;
}

function formatCost(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0.00";
  return value.toFixed(2);
}

function shortSessionId(publicId: string) {
  const raw = publicId.replace(/^cs_/, "").replace(/-/g, "").slice(0, 8);
  return raw ? raw.toUpperCase() : "—";
}

function isRehearseId(publicId?: string) {
  return Boolean(publicId?.startsWith("rehearse_"));
}

function errorText(error: unknown, fallback: string) {
  return error instanceof Error && error.message ? error.message : fallback;
}

function takeDurationLabel(ms: number) {
  return `${(ms / 1000).toFixed(1)}s`;
}

function pickRecorderMime() {
  if (typeof window === "undefined" || typeof window.MediaRecorder === "undefined") return "";
  const candidates = [
    "video/webm;codecs=vp9,opus",
    "video/webm;codecs=vp8,opus",
    "video/webm",
    "video/mp4",
  ];
  return candidates.find((type) => {
    try {
      return window.MediaRecorder.isTypeSupported(type);
    } catch {
      return false;
    }
  }) || "";
}

function videoCaptureStream(video: HTMLVideoElement): MediaStream {
  const element = video as HTMLVideoElement & { captureStream?: () => MediaStream; mozCaptureStream?: () => MediaStream };
  if (typeof element.captureStream === "function") return element.captureStream();
  if (typeof element.mozCaptureStream === "function") return element.mozCaptureStream();
  throw new Error("当前浏览器不支持直接录制画面");
}

function takeThumbnail(video: HTMLVideoElement | null): string {
  if (!video || !video.videoWidth || !video.videoHeight) return "";
  const width = 320;
  const height = Math.max(1, Math.round((video.videoHeight / video.videoWidth) * width));
  const canvas = document.createElement("canvas");
  canvas.width = width;
  canvas.height = height;
  const context = canvas.getContext("2d");
  if (!context) return "";
  context.drawImage(video, 0, 0, width, height);
  return canvas.toDataURL("image/jpeg", 0.72);
}

function mergeTakes(previous: DisplayTake[], serverTakes?: CharacterStudioTake[]): DisplayTake[] {
  if (!serverTakes?.length) return previous;
  const known = new Set(previous.map((item) => item.publicId).filter(Boolean) as string[]);
  const added = serverTakes
    .filter((item) => !known.has(item.public_id))
    .map<DisplayTake>((item) => ({
      key: item.public_id,
      publicId: item.public_id,
      url: item.url,
      durationMs: item.duration_ms,
      sizeBytes: item.size_bytes,
      characterName: "",
      status: "saved",
    }));
  return added.length ? [...previous, ...added] : previous;
}

type StatusTone = "idle" | "working" | "live" | "danger" | "done";

function StatusPill({ label, tone }: { label: string; tone: StatusTone }) {
  const toneClass = tone === "live"
    ? "border-[color:var(--mcdl-color-positive)]/30 bg-[color:var(--mcdl-color-positive)]/10 text-[color:var(--mcdl-color-positive)]"
    : tone === "danger"
      ? "border-[color:var(--mcdl-color-negative)]/30 bg-[color:var(--mcdl-color-negative)]/10 text-[color:var(--mcdl-color-negative)]"
      : tone === "working"
        ? "border-[color:var(--mcdl-accent-border)] bg-[color:var(--mcdl-accent-soft)] text-[color:var(--mcdl-color-accent)]"
        : tone === "done"
          ? "border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface-subtle)] text-[color:var(--mcdl-text-body)]"
          : "border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface-subtle)] text-[color:var(--mcdl-text-muted)]";
  return (
    <span className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-[0.6875rem] font-medium leading-none ${toneClass}`}>
      <span className={`h-1.5 w-1.5 rounded-full ${tone === "live" ? "bg-[color:var(--mcdl-color-positive)]" : tone === "working" ? "bg-[color:var(--mcdl-color-accent)]" : tone === "danger" ? "bg-[color:var(--mcdl-color-negative)]" : "bg-[color:var(--mcdl-text-muted)]"}`} />
      {label}
    </span>
  );
}

function Metric({ label, value, hint, numeric = true }: { label: string; value: string; hint?: string; numeric?: boolean }) {
  return (
    <div className="rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3 py-2.5">
      <div className="text-[0.5625rem] font-medium uppercase tracking-[0.12em] text-[color:var(--mcdl-text-muted)]">{label}</div>
      <div className={`mt-0.5 truncate text-[0.875rem] font-semibold text-[color:var(--mcdl-text-title)] ${numeric ? "tabular-nums [font-family:var(--mcdl-font-numeric)]" : ""}`}>{value}</div>
      {hint ? <div className="truncate text-[0.5625rem] text-[color:var(--mcdl-text-muted)]">{hint}</div> : null}
    </div>
  );
}

function PipelineRow({ label, active, done }: { label: string; active: boolean; done: boolean }) {
  return (
    <div className="flex items-center gap-2.5 py-1.5">
      <span className={`relative flex h-4 w-4 shrink-0 items-center justify-center rounded-full border ${done ? "border-[color:var(--mcdl-color-positive)] bg-[color:var(--mcdl-color-positive)]/15" : active ? "border-[color:var(--mcdl-accent-border)] bg-[color:var(--mcdl-accent-soft)]" : "border-[color:var(--mcdl-border-ghost)]"}`}>
        {done ? <Check size={10} className="text-[color:var(--mcdl-color-positive)]" /> : <span className={`h-1 w-1 rounded-full ${active ? "character-studio-live-dot bg-[color:var(--mcdl-color-accent)]" : "bg-[color:var(--mcdl-text-muted)]"}`} />}
      </span>
      <span className={`text-[0.75rem] ${done || active ? "text-[color:var(--mcdl-text-body)]" : "text-[color:var(--mcdl-text-muted)]"}`}>{label}</span>
      <span className="ml-auto text-[0.625rem] text-[color:var(--mcdl-text-muted)]">{done ? "OK" : active ? "RUN" : "—"}</span>
    </div>
  );
}

export function CharacterStudioWorkspace() {
  const { ts } = useI18n();
  const [mounted, setMounted] = useState(false);
  const [rehearse, setRehearse] = useState(false);
  const [phase, setPhase] = useState<Phase>("boot");
  const [config, setConfig] = useState<CharacterStudioConfig | null>(null);
  const [characters, setCharacters] = useState<CharacterStudioCharacter[]>([]);
  const [selectedId, setSelectedId] = useState("");
  const [resolution, setResolution] = useState("720p");
  const [session, setSession] = useState<CharacterStudioSession | null>(null);
  const [takes, setTakes] = useState<DisplayTake[]>([]);
  const [switchLog, setSwitchLog] = useState<SwitchLogEntry[]>([]);
  const [switchingId, setSwitchingId] = useState("");
  const [cameraState, setCameraState] = useState<CameraState>("idle");
  const [cameraError, setCameraError] = useState("");
  const [remoteReady, setRemoteReady] = useState(false);
  const [joined, setJoined] = useState(false);
  const [firstFrameMs, setFirstFrameMs] = useState<number | null>(null);
  const [mirror, setMirror] = useState(true);
  const [compare, setCompare] = useState(false);
  const [compareValue, setCompareValue] = useState(52);
  const [notice, setNotice] = useState("");
  const [elapsed, setElapsed] = useState(0);
  const [recordMic, setRecordMic] = useState(true);
  const [recordingState, setRecordingState] = useState<RecordingState>("idle");
  const [recordSeconds, setRecordSeconds] = useState(0);
  const [countdown, setCountdown] = useState<number | null>(null);
  const [editorOpen, setEditorOpen] = useState(false);
  const [draft, setDraft] = useState<CharacterDraft>(EMPTY_DRAFT);
  const [editorError, setEditorError] = useState("");
  const [editorSaving, setEditorSaving] = useState(false);
  const [editorUploading, setEditorUploading] = useState(false);
  const [editorDrag, setEditorDrag] = useState(false);
  const [imageMode, setImageMode] = useState<"upload" | "url">("upload");
  const [publicImageURL, setPublicImageURL] = useState("");
  const [imagePreviewSrc, setImagePreviewSrc] = useState("");
  const [imagePreviewError, setImagePreviewError] = useState("");
  const [dossierOpen, setDossierOpen] = useState(false);
  const [signalOpen, setSignalOpen] = useState(false);
  const [confirmEndOpen, setConfirmEndOpen] = useState(false);
  const [restoreSession, setRestoreSession] = useState<CharacterStudioSession | null>(null);
  const [restoreAction, setRestoreAction] = useState<"" | "resume" | "end">("");
  const [stageAspect, setStageAspect] = useState<"16 / 9" | "9 / 16">("16 / 9");
  const [stageError, setStageError] = useState("");
  const [rackTab, setRackTab] = useState<"default" | "mine">("default");
  const [defaultRoleLoading, setDefaultRoleLoading] = useState("");

  const imageFileRef = useRef<HTMLInputElement | null>(null);
  const localVideoRef = useRef<HTMLVideoElement | null>(null);
  const remoteVideoRef = useRef<HTMLVideoElement | null>(null);
  const stageRef = useRef<RenderStage | null>(null);
  const previewStreamRef = useRef<MediaStream | null>(null);
  const mockRef = useRef<{ setCharacter: (url?: string) => void } | null>(null);
  const joinStartRef = useRef(0);
  const joinedSessionRef = useRef("");
  const liveStartedAtRef = useRef(0);
  const sessionRef = useRef<CharacterStudioSession | null>(null);
  const phaseRef = useRef<Phase>("boot");
  const endingRef = useRef(false);
  const localObjectURLsRef = useRef<string[]>([]);
  const recorderRef = useRef<MediaRecorder | null>(null);
  const recorderChunksRef = useRef<BlobPart[]>([]);
  const recorderMicRef = useRef<MediaStream | null>(null);
  const stopRecordingRef = useRef<() => void>(() => undefined);
  const recorderStopTimerRef = useRef<number | null>(null);
  const recorderTickRef = useRef<number | null>(null);
  const recorderStartedAtRef = useRef(0);
  const rehearseTimerRef = useRef<number | null>(null);
  const runTokenRef = useRef(0);
  const noticeTimerRef = useRef<number | null>(null);
  const lastErrorRef = useRef("");

  const selectedCharacter = useMemo(
    () => characters.find((item) => item.public_id === selectedId) || null,
    [characters, selectedId],
  );
  const currentCharacter = session?.character || selectedCharacter;
  const price = session?.sale_price_per_minute ?? config?.sale_price_per_minute ?? 2.49;
  const maxSeconds = session?.max_seconds ?? (config?.max_minutes ?? 10) * 60;
  const estimatedCost = Math.round((elapsed / 60) * price * 100) / 100;
  const busy = phase === "boot" || phase === "wrapping";
  const canStart = Boolean(selectedCharacter) && !busy && phase !== "live";
  const resolutionOptions = config?.resolutions?.length ? config.resolutions : ["540p", "720p", "1080p"];

  useEffect(() => {
    setMounted(true);
  }, []);

  useEffect(() => {
    sessionRef.current = session;
  }, [session]);

  useEffect(() => {
    phaseRef.current = phase;
  }, [phase]);

  const showNotice = useCallback((message: string) => {
    setNotice(ts(message));
    if (noticeTimerRef.current) window.clearTimeout(noticeTimerRef.current);
    noticeTimerRef.current = window.setTimeout(() => setNotice(""), 6000);
  }, [ts]);

  const stopPreviewTracks = useCallback(() => {
    previewStreamRef.current?.getTracks().forEach((track) => track.stop());
    previewStreamRef.current = null;
    if (localVideoRef.current?.srcObject) localVideoRef.current.srcObject = null;
  }, []);

  const stopStage = useCallback(async () => {
    if (mockRef.current) mockRef.current = null;
    const stage = stageRef.current;
    stageRef.current = null;
    joinedSessionRef.current = "";
    setJoined(false);
    if (stage) {
      try {
        await stage.leave();
      } catch {
        /* 通道可能已经被远端关闭 */
      }
    }
  }, []);

  const mergeServerTakes = useCallback((next?: CharacterStudioTake[]) => {
    setTakes((previous) => mergeTakes(previous, next));
  }, []);

  // 启动：加载配置、角色卡与未关闭的会话。
  useEffect(() => {
    clearPendingPageEnd();
    void preloadRenderStage();
    let cancelled = false;
    const rehearsal = process.env.NODE_ENV !== "production" && new URLSearchParams(window.location.search).get("rehearse") === "1";
    setRehearse(rehearsal);
    (async () => {
      try {
        const [nextConfig, nextCharacters] = await Promise.all([
          fetchCharacterStudioConfig(),
          listCharacterStudioCharacters(),
        ]);
        if (cancelled) return;
        setConfig(nextConfig);
        setCharacters(nextCharacters);
        setResolution(nextConfig.default_resolution || "720p");
        if (nextCharacters[0]) setSelectedId(nextCharacters[0].public_id);
        if (!rehearsal) {
          const active = await getActiveCharacterStudioSession();
          if (!cancelled) setRestoreSession(active);
        }
        if (!cancelled) setPhase("idle");
      } catch (error) {
        if (cancelled) return;
        setPhase("failed");
        showNotice(errorText(error, "角色架加载失败"));
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [showNotice]);

  // 离开页面：尽量在服务端结束会话；组件内跳转时用短延迟避免 StrictMode 误结束。
  useEffect(() => {
    const onPageHide = () => {
      const current = sessionRef.current;
      if (!current || isRehearseId(current.public_id)) return;
      if (phaseRef.current !== "live" && phaseRef.current !== "starting") return;
      const url = `${process.env.NEXT_PUBLIC_API_URL || ""}/api/character-studio/sessions/${encodeURIComponent(current.public_id)}/end`;
      try {
        fetch(url, { method: "POST", credentials: "include", keepalive: true, headers: { "Content-Type": "application/json" }, body: "{}" }).catch(() => undefined);
      } catch {
        /* 页面卸载时无法保证请求送达，服务端心跳兜底 */
      }
    };
    const onBeforeUnload = (event: BeforeUnloadEvent) => {
      if (phaseRef.current !== "live") return;
      event.preventDefault();
      event.returnValue = "";
    };
    window.addEventListener("pagehide", onPageHide);
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => {
      window.removeEventListener("pagehide", onPageHide);
      window.removeEventListener("beforeunload", onBeforeUnload);
      const current = sessionRef.current;
      if (current && !isRehearseId(current.public_id) && (phaseRef.current === "live" || phaseRef.current === "starting")) {
        schedulePageEnd(current.public_id);
      }
    };
  }, []);

  useEffect(() => {
    if (!noticeTimerRef.current) return;
    if (notice) return;
    window.clearTimeout(noticeTimerRef.current);
    noticeTimerRef.current = null;
  }, [notice]);

  useEffect(() => {
    if (session?.error && session.error !== lastErrorRef.current) {
      lastErrorRef.current = session.error;
      showNotice(session.error);
    }
    if (!session?.error) lastErrorRef.current = "";
  }, [session?.error, showNotice]);

  // 轮询会话状态。
  useEffect(() => {
    if (!session?.public_id || isRehearseId(session.public_id)) return;
    if (!["starting", "live", "reconnecting", "wrapping"].includes(session.status)) return;
    let cancelled = false;
    let busyPolling = false;
    const interval = window.setInterval(async () => {
      if (cancelled || busyPolling) return;
      busyPolling = true;
      try {
        const next = await getCharacterStudioSession(session.public_id);
        if (cancelled) return;
        mergeServerTakes(next.takes);
        setSession(next);
        if (next.status === "live" && phaseRef.current === "starting") {
          liveStartedAtRef.current = Date.now();
          setPhase("live");
        }
        if (next.status === "ended" || next.status === "failed") {
          setPhase(next.status === "failed" ? "failed" : "wrapped");
          setCountdown(null);
          stopRecordingRef.current();
          void stopStage();
        }
      } catch {
        /* 轮询失败不打断画面，下一次继续 */
      } finally {
        busyPolling = false;
      }
    }, session.status === "starting" ? 900 : 2500);
    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [session?.public_id, session?.status, mergeServerTakes, stopStage]);

  // 心跳：服务端用心跳判断页面是否还在。
  useEffect(() => {
    if (phase !== "live" || !session?.public_id || isRehearseId(session.public_id)) return;
    const beat = () => {
      void heartbeatCharacterStudioSession(session.public_id).catch(() => undefined);
    };
    beat();
    const interval = window.setInterval(beat, 10000);
    return () => window.clearInterval(interval);
  }, [phase, session?.public_id]);

  // 实时计时与预估费用。
  useEffect(() => {
    if (phase !== "live") return;
    const tick = () => {
      const start = session?.started_at ? Date.parse(session.started_at) : liveStartedAtRef.current;
      if (!start || Number.isNaN(start)) {
        setElapsed(0);
        return;
      }
      const next = Math.max(0, Math.floor((Date.now() - start) / 1000));
      setElapsed(maxSeconds > 0 ? Math.min(next, maxSeconds) : next);
    };
    tick();
    const interval = window.setInterval(tick, 1000);
    return () => window.clearInterval(interval);
  }, [phase, session?.started_at, maxSeconds]);

  const openCamera = useCallback(async () => {
    void preloadRenderStage();
    if (cameraState === "ready" && previewStreamRef.current) return true;
    if (!navigator.mediaDevices?.getUserMedia) {
      setCameraState("error");
      setCameraError(ts("当前浏览器不支持摄像头，请使用 Chrome 或 Edge"));
      return false;
    }
    setCameraState("requesting");
    setCameraError("");
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        video: { facingMode: "user", width: { ideal: 1280 }, height: { ideal: 720 } },
        audio: false,
      });
      previewStreamRef.current = stream;
      const video = localVideoRef.current;
      if (video) {
        video.srcObject = stream;
        await video.play().catch(() => undefined);
      }
      setCameraState("ready");
      return true;
    } catch {
      setCameraState("error");
      setCameraError(ts("没有拿到摄像头权限。请允许浏览器使用摄像头，或检查设备是否被其他程序占用。"));
      return false;
    }
  }, [cameraState, ts]);

  const closeCameraPreview = useCallback(() => {
    stopPreviewTracks();
    setCameraState("idle");
    setCameraError("");
  }, [stopPreviewTracks]);

  const joinStage = useCallback(async (target: CharacterStudioSession) => {
    const channel = target.channel;
    const outputChannel = target.output_channel;
    if (!channel || !outputChannel) return;
    const localVideo = localVideoRef.current;
    const remoteVideo = remoteVideoRef.current;
    if (!localVideo || !remoteVideo) return;
    joinedSessionRef.current = target.public_id;
    joinStartRef.current = performance.now();
    setFirstFrameMs(null);
    setRemoteReady(false);
    setJoined(false);
    setStageError("");

    const baseOptions = {
      channel,
      outputChannel,
      localVideo,
      remoteVideo,
      onFirstFrame: () => {
        setFirstFrameMs(Math.round(performance.now() - joinStartRef.current));
        setRemoteReady(true);
        setStageError("");
      },
      onRemoteOnline: () => setRemoteReady(true),
      onRemoteOffline: () => {
        setRemoteReady(false);
        showNotice(ts("合成画面暂时中断，正在等待恢复"));
      },
    };

    if (isRehearseId(target.public_id)) {
      try {
        const mockModule = await import("@/lib/realtime/renderStageMock");
        mockRef.current = { setCharacter: mockModule.setMockCharacter };
        mockModule.setMockCharacter(currentCharacter?.image_url);
        const stage = await mockModule.createMockRenderStage();
        stageRef.current = stage;
        await stage.join(baseOptions);
        setJoined(true);
      } catch (error) {
        setStageError(errorText(error, ts("实时通道连接失败，请检查网络后重试")));
      }
      return;
    }

    // 先释放排练预览，给摄像头一个短暂的释放窗口；运行时已由 openCamera 预热。
    stopPreviewTracks();
    await new Promise((resolve) => window.setTimeout(resolve, 250));

    let lastError: unknown = null;
    for (let attempt = 0; attempt < 2; attempt += 1) {
      if (endingRef.current || phaseRef.current !== "live" || sessionRef.current?.public_id !== target.public_id) return;
      try {
        const stage = await createRenderStage();
        stageRef.current = stage;
        await stage.join({
          ...baseOptions,
          onClosed: () => {
            if (endingRef.current || phaseRef.current !== "live") return;
            setRemoteReady(false);
            setStageError(ts("实时通道已断开，请重新接入"));
            showNotice(ts("实时通道已断开，请重新接入"));
            const active = stageRef.current;
            stageRef.current = null;
            void active?.leave().catch(() => undefined);
          },
        });
        setJoined(true);
        setStageError("");
        return;
      } catch (error) {
        lastError = error;
        await stageRef.current?.leave().catch(() => undefined);
        stageRef.current = null;
        if (attempt === 0) {
          await new Promise((resolve) => window.setTimeout(resolve, 900));
          stopPreviewTracks();
        }
      }
    }
    setStageError(errorText(lastError, ts("实时通道连接失败，请检查网络后重试")));
    showNotice(ts("实时通道连接失败，请检查网络后重试"));
  }, [currentCharacter?.image_url, showNotice, stopPreviewTracks, ts]);

  const retryStage = useCallback(async () => {
    const current = sessionRef.current;
    if (!current || endingRef.current || phaseRef.current !== "live") return;
    setStageError("");
    setRemoteReady(false);
    setFirstFrameMs(null);
    setJoined(false);
    joinedSessionRef.current = "";
    await stopStage();
    await joinStage(current);
  }, [joinStage, stopStage]);

  // 会话进入 live 后加入实时通道。
  useEffect(() => {
    if (phase !== "live" || !session?.public_id || !session.channel || !session.output_channel) return;
    if (joinedSessionRef.current === session.public_id) return;
    void joinStage(session);
  }, [phase, session?.public_id, session?.channel, session?.output_channel, joinStage, session]);

  const endLiveSession = useCallback(async (reason: "user" | "join_failed" | "connection_lost") => {
    const current = sessionRef.current;
    if (!current || endingRef.current) return;
    endingRef.current = true;
    if (reason !== "user") showNotice(reason === "connection_lost" ? current.error || ts("实时通道已断开，会话已结束") : ts("这次会话没有成功开播"));
    setPhase("wrapping");
    setCountdown(null);
    stopRecordingRef.current();
    await stopStage();
    stopPreviewTracks();
    setCameraState("idle");
    if (isRehearseId(current.public_id)) {
      const startedAt = liveStartedAtRef.current || Date.now();
      const billed = Math.max(0, Math.floor((Date.now() - startedAt) / 1000));
      const ended: CharacterStudioSession = {
        ...current,
        status: "ended",
        billed_seconds: billed,
        elapsed_seconds: billed,
        cost: Math.round((billed / 60) * (current.sale_price_per_minute || 2.49) * 1e6) / 1e6,
        end_reason: reason === "user" ? ts("你结束了本次会话") : current.error,
      };
      setSession(ended);
      setPhase("wrapped");
      endingRef.current = false;
      return;
    }
    const failedJoin = reason === "join_failed";
    try {
      const ended = await endCharacterStudioSession(current.public_id);
      mergeServerTakes(ended.takes);
      if (failedJoin) ended.end_reason = ts("这次没有成功开播");
      if (reason === "connection_lost") ended.end_reason = ts("实时通道已断开，会话已结束");
      setSession(ended);
      setElapsed(ended.billed_seconds || ended.elapsed_seconds || 0);
      setPhase(failedJoin || ended.status === "failed" ? "failed" : "wrapped");
    } catch (error) {
      showNotice(errorText(error, ts("结束会话失败，请稍后重试")));
      setPhase("wrapped");
    } finally {
      endingRef.current = false;
    }
  }, [mergeServerTakes, showNotice, stopPreviewTracks, stopStage, ts]);

  // 结束页面资源。
  useEffect(() => {
    const objectURLs = localObjectURLsRef.current;
    return () => {
      stopPreviewTracks();
      void stageRef.current?.leave().catch(() => undefined);
      if (rehearseTimerRef.current) window.clearTimeout(rehearseTimerRef.current);
      if (noticeTimerRef.current) window.clearTimeout(noticeTimerRef.current);
      objectURLs.forEach((url) => URL.revokeObjectURL(url));
    };
  }, [stopPreviewTracks]);

  const startSession = async () => {
    if (!selectedCharacter) {
      showNotice(ts("先选择一个角色，再开始换角"));
      return;
    }
    const cameraReady = await openCamera();
    if (!cameraReady) return;
    setPhase("starting");
    setNotice("");
    setStageError("");
    setSwitchLog([]);
    liveStartedAtRef.current = Date.now();
    const token = ++runTokenRef.current;
    if (rehearse) {
      const startedAt = new Date().toISOString();
      const mockSession: CharacterStudioSession = {
        public_id: `rehearse_${Date.now()}`,
        status: "starting",
        resolution,
        character: selectedCharacter,
        elapsed_seconds: 0,
        max_seconds: (config?.max_minutes ?? 10) * 60,
        sale_price_per_minute: price,
        switch_count: 0,
        billed_seconds: 0,
        cost: 0,
        created_at: startedAt,
      };
      setSession(mockSession);
      if (rehearseTimerRef.current) window.clearTimeout(rehearseTimerRef.current);
      rehearseTimerRef.current = window.setTimeout(() => {
        if (token !== runTokenRef.current) return;
        const liveSession: CharacterStudioSession = {
          ...mockSession,
          status: "live",
          started_at: new Date().toISOString(),
          channel: { app_id: "rehearsal", channel_id: "rehearsal", user_id: "rehearsal", token: "rehearsal", token_expire_at: "0" },
          output_channel: "rehearsal",
          can_resume: true,
        };
        liveStartedAtRef.current = Date.now();
        setSession(liveSession);
        setPhase("live");
      }, 1400);
      return;
    }
    try {
      const started = await startCharacterStudioSession(selectedCharacter.public_id, resolution);
      if (token !== runTokenRef.current) return;
      setSession(started);
      mergeServerTakes(started.takes);
    } catch (error) {
      setPhase("idle");
      showNotice(errorText(error, ts("开播失败，请稍后再试")));
    }
  };

  const switchTo = async (character: CharacterStudioCharacter) => {
    setSelectedId(character.public_id);
    if (phase !== "live" || !session) return;
    if (session.character?.public_id === character.public_id) return;
    if (switchingId) return;
    setSwitchingId(character.public_id);
    const characterName = character.name;
    if (isRehearseId(session.public_id)) {
      mockRef.current?.setCharacter(character.image_url);
      setSession((previous) => previous ? { ...previous, character, switch_count: previous.switch_count + 1, error: undefined } : previous);
      setSwitchLog((previous) => [...previous, { key: `${character.public_id}-${Date.now()}`, name: characterName, at: Date.now() }]);
      window.setTimeout(() => setSwitchingId(""), 3600);
      return;
    }
    try {
      const result = await switchCharacterStudioCharacter(session.public_id, character.public_id);
      setSession((previous) => previous ? { ...previous, character: result.character, switch_count: previous.switch_count + 1, error: undefined } : previous);
      setSwitchLog((previous) => [...previous, { key: `${character.public_id}-${Date.now()}`, name: characterName, at: Date.now() }]);
      window.setTimeout(() => setSwitchingId(""), 3600);
    } catch (error) {
      setSwitchingId("");
      showNotice(errorText(error, ts("角色切换没有生效，请再试一次")));
    }
  };

  const cancelStarting = async () => {
    if (!session?.public_id) return;
    if (isRehearseId(session.public_id)) {
      if (rehearseTimerRef.current) window.clearTimeout(rehearseTimerRef.current);
      setSession(null);
      setPhase("idle");
      return;
    }
    setPhase("wrapping");
    try {
      const ended = await endCharacterStudioSession(session.public_id);
      setSession(ended);
      setPhase("wrapped");
    } catch (error) {
      showNotice(errorText(error, ts("取消失败，请稍后重试")));
      setPhase("failed");
    }
  };

  const confirmEnd = async () => {
    setConfirmEndOpen(false);
    await endLiveSession("user");
  };

  const resetStudio = () => {
    clearPendingPageEnd();
    setSession(null);
    setTakes([]);
    setSwitchLog([]);
    setElapsed(0);
    setFirstFrameMs(null);
    setRemoteReady(false);
    setCompare(false);
    setPhase("idle");
    setNotice("");
    setCountdown(null);
    setStageError("");
    setSwitchingId("");
    localObjectURLsRef.current.forEach((url) => URL.revokeObjectURL(url));
    localObjectURLsRef.current.length = 0;
    stopPreviewTracks();
    setCameraState("idle");
  };

  const resumeSession = async () => {
    if (!restoreSession) return;
    setRestoreAction("resume");
    try {
      const next = await getCharacterStudioSession(restoreSession.public_id);
      mergeServerTakes(next.takes);
      setSession(next);
      setSelectedId(next.character?.public_id || selectedId);
      lastErrorRef.current = "";
      if (next.status === "live" || next.status === "reconnecting") {
        liveStartedAtRef.current = next.started_at ? Date.parse(next.started_at) : Date.now();
        const cameraReady = await openCamera();
        if (!cameraReady) {
          setRestoreSession(null);
          setPhase("idle");
          return;
        }
        setPhase("live");
      } else if (next.status === "starting") {
        setPhase("starting");
      } else {
        setPhase(next.status === "failed" ? "failed" : "wrapped");
      }
      setRestoreSession(null);
    } catch (error) {
      showNotice(errorText(error, ts("恢复会话失败")));
    } finally {
      setRestoreAction("");
    }
  };

  const endRestoredSession = async () => {
    if (!restoreSession) return;
    setRestoreAction("end");
    try {
      const ended = await endCharacterStudioSession(restoreSession.public_id);
      mergeServerTakes(ended.takes);
      setSession(ended);
      setPhase(ended.status === "failed" ? "failed" : "wrapped");
      setRestoreSession(null);
    } catch (error) {
      showNotice(errorText(error, ts("结束会话失败")));
    } finally {
      setRestoreAction("");
    }
  };

  // ---------- 角色卡编辑 ----------

  const openCreateCharacter = () => {
    setDraft(EMPTY_DRAFT);
    setImageMode("upload");
    setPublicImageURL("");
    setImagePreviewSrc("");
    setImagePreviewError("");
    setEditorError("");
    setEditorOpen(true);
  };

  const openEditCharacter = (character: CharacterStudioCharacter) => {
    const isPublic = /^https?:\/\//i.test(character.image_url);
    setDraft({ publicId: character.public_id, name: character.name, imageURL: character.image_url, rights: true });
    setImageMode(isPublic ? "url" : "upload");
    setPublicImageURL(isPublic ? character.image_url : "");
    setImagePreviewSrc(isPublic ? character.image_url : "");
    setImagePreviewError("");
    setEditorError("");
    setEditorOpen(true);
  };

  useEffect(() => {
    if (!editorOpen || imageMode !== "url") return;
    const raw = publicImageURL.trim();
    if (!raw) {
      setImagePreviewSrc("");
      setImagePreviewError("");
      return;
    }
    if (!/^https?:\/\//i.test(raw)) {
      setImagePreviewSrc("");
      setImagePreviewError(ts("请输入 http(s) 开头的公网地址"));
      return;
    }
    setImagePreviewError("");
    const timer = window.setTimeout(() => setImagePreviewSrc(raw), 350);
    return () => window.clearTimeout(timer);
  }, [editorOpen, imageMode, publicImageURL, ts]);

  const uploadCharacterFile = async (file?: File) => {
    if (!file) return;
    if (!/^image\/(png|jpe?g|webp)$/i.test(file.type)) {
      setEditorError(ts("只支持 PNG、JPG、WebP 图片"));
      return;
    }
    if (file.size > 8 * 1024 * 1024) {
      setEditorError(ts("图片不能超过 8MB，请压缩后重试"));
      return;
    }
    setEditorUploading(true);
    setEditorError("");
    try {
      const url = await uploadCharacterStudioImage(file);
      setDraft((previous) => ({ ...previous, imageURL: url }));
    } catch (error) {
      setEditorError(errorText(error, ts("图片上传失败，请重试")));
    } finally {
      setEditorUploading(false);
    }
  };

  const saveCharacter = async (event?: FormEvent) => {
    event?.preventDefault();
    if (editorSaving || editorUploading) return;
    if (!draft.name.trim()) {
      setEditorError(ts("请给这个角色起个名字"));
      return;
    }
    const imageURL = imageMode === "url" ? publicImageURL.trim() : draft.imageURL;
    if (!imageURL) {
      setEditorError(ts(imageMode === "url" ? "请填写公网图片地址" : "请上传一张角色参考图"));
      return;
    }
    if (imageMode === "url" && !/^https?:\/\//i.test(imageURL)) {
      setEditorError(ts("请输入 http(s) 开头的公网地址"));
      return;
    }
    if (!draft.publicId && !draft.rights) {
      setEditorError(ts("请先确认你拥有该形象的使用授权"));
      return;
    }
    setEditorSaving(true);
    setEditorError("");
    try {
      const saved = draft.publicId
        ? await updateCharacterStudioCharacter(draft.publicId, { name: draft.name.trim(), image_url: imageURL, rights_confirmed: true })
        : await createCharacterStudioCharacter({ name: draft.name.trim(), image_url: imageURL, rights_confirmed: true });
      setCharacters((previous) => {
        const without = previous.filter((item) => item.public_id !== saved.public_id);
        return [saved, ...without];
      });
      setSelectedId(saved.public_id);
      setEditorOpen(false);
      showNotice(ts("角色卡已保存"));
    } catch (error) {
      setEditorError(errorText(error, ts("保存角色卡失败")));
    } finally {
      setEditorSaving(false);
    }
  };

  const removeCharacter = async () => {
    if (!draft.publicId) return;
    if (!window.confirm(ts("删除这张角色卡？已经抓取的片段不会被删除。"))) return;
    setEditorSaving(true);
    setEditorError("");
    try {
      await deleteCharacterStudioCharacter(draft.publicId);
      const next = characters.filter((item) => item.public_id !== draft.publicId);
      setCharacters(next);
      if (selectedId === draft.publicId) setSelectedId(next[0]?.public_id || "");
      setEditorOpen(false);
      showNotice(ts("角色卡已删除"));
    } catch (error) {
      setEditorError(errorText(error, ts("删除角色卡失败")));
    } finally {
      setEditorSaving(false);
    }
  };

  // ---------- 片段录制 ----------

  const stopRecordingInternal = useCallback(() => {
    if (recorderStopTimerRef.current) {
      window.clearTimeout(recorderStopTimerRef.current);
      recorderStopTimerRef.current = null;
    }
    if (recorderTickRef.current) {
      window.clearInterval(recorderTickRef.current);
      recorderTickRef.current = null;
    }
    const recorder = recorderRef.current;
    if (recorder && recorder.state !== "inactive") {
      try {
        recorder.stop();
      } catch {
        /* 已经停止 */
      }
    } else {
      setRecordingState("idle");
    }
    recorderRef.current = null;
    recorderMicRef.current?.getTracks().forEach((track) => track.stop());
    recorderMicRef.current = null;
  }, []);

  useEffect(() => {
    stopRecordingRef.current = stopRecordingInternal;
  }, [stopRecordingInternal]);

  const finalizeRecording = useCallback(async (blob: Blob, mimeType: string, durationMs: number, characterName: string, thumb: string, sessionId: string) => {
    const extension = mimeType.includes("mp4") ? "mp4" : "webm";
    const file = new File([blob], `character-studio-${Date.now()}.${extension}`, { type: blob.type || mimeType || "video/webm" });
    const url = URL.createObjectURL(blob);
    localObjectURLsRef.current.push(url);
    const key = `local-${Date.now()}`;
    const entry: DisplayTake = {
      key,
      url,
      thumb,
      durationMs,
      sizeBytes: blob.size,
      characterName,
      status: rehearse ? "local" : "uploading",
    };
    setTakes((previous) => [...previous, entry]);
    setRecordSeconds(0);
    setRecordingState("idle");
    if (rehearse) {
      showNotice(ts("本地演练片段已生成，可以直接预览或下载"));
      return;
    }
    setRecordingState("uploading");
    try {
      const asset = await uploadCharacterStudioTake(file);
      const saved = await addCharacterStudioTake(sessionId, {
        url: asset.url,
        asset_public_id: asset.public_id,
        duration_ms: durationMs,
        size_bytes: blob.size,
      });
      setTakes((previous) => previous.map((item) => item.key === key
        ? { ...item, key: saved.public_id, publicId: saved.public_id, url: asset.url, status: "saved" }
        : item));
      showNotice(ts("片段已存入我的作品"));
    } catch (error) {
      setTakes((previous) => previous.map((item) => item.key === key ? { ...item, status: "failed" } : item));
      showNotice(errorText(error, ts("片段已生成，但同步到作品库失败")));
    } finally {
      setRecordingState("idle");
    }
  }, [rehearse, showNotice, ts]);

  const startRecording = useCallback(async () => {
    if (phase !== "live" || !session?.public_id || recordingState !== "idle") return;
    const remote = remoteVideoRef.current;
    if (!remote || !remoteReady) {
      showNotice(ts("等合成画面出现后再抓取片段"));
      return;
    }
    if (typeof window.MediaRecorder === "undefined") {
      showNotice(ts("当前浏览器不支持录制，请使用 Chrome 或 Edge"));
      return;
    }
    let stream: MediaStream;
    try {
      const captured = videoCaptureStream(remote);
      stream = new MediaStream(captured.getVideoTracks());
    } catch {
      showNotice(ts("当前浏览器不支持录制合成画面"));
      return;
    }
    if (recordMic) {
      let timedOut = false;
      const micPromise = navigator.mediaDevices.getUserMedia({ audio: true }).then((mic) => {
        if (timedOut) {
          mic.getTracks().forEach((track) => track.stop());
          return null;
        }
        return mic;
      }).catch(() => null);
      const microphone = await Promise.race([
        micPromise,
        new Promise<MediaStream | null>((resolve) => {
          window.setTimeout(() => {
            timedOut = true;
            resolve(null);
          }, 1500);
        }),
      ]);
      if (microphone) {
        recorderMicRef.current = microphone;
        microphone.getAudioTracks().forEach((track) => stream.addTrack(track));
      } else {
        showNotice(ts("麦克风没有响应，这段片段将只保留画面"));
      }
    }
    const mimeType = pickRecorderMime();
    let recorder: MediaRecorder;
    try {
      recorder = new MediaRecorder(stream, mimeType ? { mimeType, videoBitsPerSecond: 2_000_000 } : undefined);
    } catch {
      stream.getTracks().forEach((track) => track.stop());
      showNotice(ts("录制初始化失败，请刷新页面后重试"));
      return;
    }
    const characterName = currentCharacter?.name || "";
    const thumb = takeThumbnail(remote);
    const sessionId = session.public_id;
    recorderChunksRef.current = [];
    recorderRef.current = recorder;
    recorderStartedAtRef.current = Date.now();
    recorder.ondataavailable = (event) => {
      if (event.data && event.data.size > 0) recorderChunksRef.current.push(event.data);
    };
    recorder.onstop = () => {
      const duration = Math.max(0, Date.now() - recorderStartedAtRef.current);
      const type = recorder.mimeType || mimeType || "video/webm";
      const blob = new Blob(recorderChunksRef.current, { type });
      recorderChunksRef.current = [];
      recorderMicRef.current?.getTracks().forEach((track) => track.stop());
      recorderMicRef.current = null;
      if (blob.size > 0) {
        void finalizeRecording(blob, type, duration, characterName, thumb, sessionId);
      } else {
        setRecordingState("idle");
      }
    };
    try {
      recorder.start(800);
      setRecordingState("recording");
      setRecordSeconds(0);
      recorderTickRef.current = window.setInterval(() => {
        setRecordSeconds(Math.min(60, Math.floor((Date.now() - recorderStartedAtRef.current) / 1000)));
      }, 250);
      recorderStopTimerRef.current = window.setTimeout(() => stopRecordingInternal(), 60000);
    } catch {
      showNotice(ts("录制启动失败，请重试"));
      setRecordingState("idle");
    }
  }, [currentCharacter?.name, finalizeRecording, phase, recordMic, recordingState, remoteReady, session?.public_id, showNotice, stopRecordingInternal, ts]);

  useEffect(() => {
    if (countdown === null) return;
    if (countdown <= 0) {
      setCountdown(null);
      void startRecording();
      return;
    }
    const timer = window.setTimeout(() => setCountdown((value) => (value === null ? null : value - 1)), 720);
    return () => window.clearTimeout(timer);
  }, [countdown, startRecording]);

  const beginTake = () => {
    if (recordingState !== "idle" || countdown !== null || !remoteReady) return;
    setCountdown(3);
  };

  const stopRecording = () => {
    if (recorderStopTimerRef.current) {
      window.clearTimeout(recorderStopTimerRef.current);
      recorderStopTimerRef.current = null;
    }
    stopRecordingInternal();
  };

  const handleLocalVideoMeta = () => {
    const video = localVideoRef.current;
    if (!video || !video.videoWidth || !video.videoHeight) return;
    setStageAspect(video.videoHeight > video.videoWidth ? "9 / 16" : "16 / 9");
  };

  // 快捷键：1-9 换角、R 抓取、Esc 结束会话确认。
  useEffect(() => {
    if (phase !== "live") return;
    const onKey = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (target && (target.tagName === "INPUT" || target.tagName === "TEXTAREA" || target.isContentEditable)) return;
      if (editorOpen || dossierOpen || signalOpen || confirmEndOpen) return;
      if (event.key === "Escape") {
        setConfirmEndOpen(true);
        return;
      }
      if (event.key.toLowerCase() === "r" && recordingState === "idle") {
        beginTake();
        return;
      }
      const index = Number(event.key) - 1;
      if (Number.isInteger(index) && index >= 0 && index < 9 && characters[index]) {
        void switchTo(characters[index]);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  // Esc 关闭浮层。
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      if (editorOpen) setEditorOpen(false);
      else if (dossierOpen) setDossierOpen(false);
      else if (signalOpen) setSignalOpen(false);
      else if (confirmEndOpen) setConfirmEndOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [confirmEndOpen, dossierOpen, editorOpen, signalOpen]);

  // 本地片段对象 URL 回收。
  useEffect(() => {
    return () => {
      if (recorderStopTimerRef.current) window.clearTimeout(recorderStopTimerRef.current);
      if (recorderTickRef.current) window.clearInterval(recorderTickRef.current);
      if (rehearseTimerRef.current) window.clearTimeout(rehearseTimerRef.current);
    };
  }, []);

  const statusLabel = phase === "boot"
    ? ts("正在准备")
    : phase === "starting"
      ? ts("接入中")
      : phase === "live"
        ? ts("直播中")
        : phase === "wrapping"
          ? ts("正在结算")
          : phase === "wrapped"
            ? ts("已收工")
            : phase === "failed"
              ? ts("未开播")
              : ts("待开播");
  const statusTone: StatusTone = phase === "live" ? "live" : phase === "starting" || phase === "wrapping" ? "working" : phase === "failed" ? "danger" : phase === "wrapped" ? "done" : "idle";

  const startupSteps = [
    { label: ts("建立会话"), done: Boolean(session?.public_id) },
    { label: ts("渲染节点就绪"), done: phase === "live" || Boolean(remoteReady) },
    { label: ts("接入实时通道"), done: joined || Boolean(remoteReady) },
    { label: ts("首帧画面"), done: Boolean(remoteReady) },
  ];

  const renderSignalPanel = (compact = false) => (
    <div className={`flex min-h-0 flex-col ${compact ? "" : "h-full"}`}>
      <div className="flex items-center justify-between px-1">
        <div>
          <div className="mcdl-eyebrow">{ts("信号")}</div>
          <div className="mt-1 text-[0.9375rem] font-semibold">{phase === "live" ? ts("实时运行中") : phase === "starting" ? ts("正在接入") : ts("等待开播")}</div>
        </div>
        <StatusPill label={statusLabel} tone={statusTone} />
      </div>
      <div className="mt-4 grid grid-cols-2 gap-2">
        <Metric label={ts("通道")} value={phase === "live" ? (joined ? ts("已连接") : ts("接入中")) : ts("未连接")} hint={rehearse && phase === "live" ? ts("本地演练") : undefined} numeric={false} />
        <Metric label={ts("输出")} value={session?.resolution || resolution} hint={ts("合成画面")} />
        <Metric label={ts("首帧")} value={firstFrameMs ? `${(firstFrameMs / 1000).toFixed(1)}s` : "—"} hint={ts("客户端实测")} />
        <Metric label={ts("已运行")} value={formatClock(elapsed)} hint={`${ts("上限")} ${formatClock(maxSeconds)}`} />
        <Metric label={ts("换角")} value={String(session?.switch_count ?? 0)} hint={ts("本次")} />
        <Metric label={ts("预计")} value={`${formatCost(estimatedCost)} ${ts("算力")}`} hint={`${price.toFixed(2)} / ${ts("分钟")}`} />
      </div>

      <div className="mt-3 rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3 py-2.5">
        <div className="flex items-center justify-between">
          <span className="text-[0.625rem] font-semibold uppercase tracking-[0.12em] text-[color:var(--mcdl-text-muted)]">{ts("管线")}</span>
          <span className="text-[0.5625rem] text-[color:var(--mcdl-text-muted)]">{ts("运行中")}</span>
        </div>
        <div className="mt-2">
          <PipelineRow label={ts("采集")} active={phase !== "idle" && phase !== "boot"} done={Boolean(remoteReady)} />
          <PipelineRow label={ts("身份锚定")} active={phase === "live"} done={Boolean(remoteReady)} />
          <PipelineRow label={ts("流式重绘")} active={phase === "live"} done={Boolean(remoteReady)} />
          <PipelineRow label={ts("时域稳定")} active={phase === "live"} done={Boolean(remoteReady)} />
          <PipelineRow label={ts("合成输出")} active={phase === "live"} done={Boolean(remoteReady)} />
        </div>
      </div>

      <div className="mt-3 shrink-0 overflow-hidden rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3 py-2.5">
        <div className="flex items-center justify-between">
          <span className="text-[0.6875rem] font-semibold uppercase tracking-[0.12em] text-[color:var(--mcdl-text-muted)]">{ts("切换记录")}</span>
          {session?.public_id ? <span className="font-mono text-[0.625rem] text-[color:var(--mcdl-text-muted)]">#{shortSessionId(session.public_id)}</span> : null}
        </div>
        <div className="mt-1.5 max-h-24 space-y-1 overflow-y-auto pr-1">
          {switchLog.length === 0 ? (
            <p className="py-3 text-[0.6875rem] leading-5 text-[color:var(--mcdl-text-muted)]">{ts("开播后点击角色卡即可换角，这里会留下记录。")}</p>
          ) : switchLog.slice().reverse().map((entry) => (
            <div key={entry.key} className="flex items-center gap-2 text-[0.6875rem]">
              <span className="h-1 w-1 rounded-full bg-[color:var(--mcdl-color-accent)]" />
              <span className="min-w-0 flex-1 truncate text-[color:var(--mcdl-text-body)]">{entry.name}</span>
              <span className="tabular-nums text-[color:var(--mcdl-text-muted)]">{new Date(entry.at).toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })}</span>
            </div>
          ))}
        </div>
      </div>

    </div>
  );

  const renderSetupPanel = (compact = false) => {
    const isStarting = phase === "starting";
    const reloadNeeded = !config && phase === "failed";
    const canStartHere = Boolean(selectedCharacter) && Boolean(config) && phase !== "starting" && phase !== "wrapping" && phase !== "live";
    const primaryLabel = reloadNeeded
      ? ts("重新加载")
      : phase === "wrapped" || phase === "failed"
        ? ts("再来一条")
        : isStarting
          ? ts("接入中…")
          : ts("开始换角");
    const primaryAction = () => {
      if (reloadNeeded) {
        window.location.reload();
        return;
      }
      if (phase === "wrapped" || phase === "failed") {
        resetStudio();
        return;
      }
      if (isStarting) return;
      void startSession();
    };
    return (
      <div className={`flex min-h-0 flex-col ${compact ? "" : "h-full"}`}>
        <div className="flex items-center justify-between gap-2 px-1">
          <div>
            <div className="mcdl-eyebrow">{ts("开机检查")}</div>
            <div className="mt-1 text-[0.9375rem] font-semibold">{isStarting ? ts("正在接入渲染节点") : phase === "wrapped" ? ts("本次已收工") : ts("准备好再开播")}</div>
          </div>
          <StatusPill label={statusLabel} tone={statusTone} />
        </div>

        <div className="mt-4 space-y-2.5">
          <div className="flex items-center gap-3 rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] p-2.5">
            <div className="h-14 w-11 shrink-0 overflow-hidden rounded-xl bg-[color:var(--mcdl-color-surface-card)]">
              {selectedCharacter?.image_url ? (
                <img src={selectedCharacter.image_url} alt="" className="h-full w-full object-cover" />
              ) : (
                <div className="flex h-full items-center justify-center text-[color:var(--mcdl-text-muted)]"><VenetianMask size={16} /></div>
              )}
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-[0.5625rem] font-semibold uppercase tracking-[0.14em] text-[color:var(--mcdl-text-muted)]">{ts("当前角色")}</div>
              <div className="truncate text-[0.8125rem] font-semibold">{selectedCharacter?.name || ts("还没有选择角色")}</div>
              <div className="truncate text-[0.625rem] text-[color:var(--mcdl-text-muted)]">{selectedCharacter ? ts("参考形象已就绪") : ts("从左侧角色架选一张")}</div>
            </div>
            {selectedCharacter ? (
              <Check size={15} className="shrink-0 text-[color:var(--mcdl-color-positive)]" />
            ) : (
              <button type="button" onClick={openCreateCharacter} className="shrink-0 rounded-full border border-[color:var(--mcdl-border-ghost)] px-2.5 py-1 text-[0.625rem] text-[color:var(--mcdl-text-body)] transition hover:border-[color:var(--mcdl-hover-border)]">{ts("新建")}</button>
            )}
          </div>

          <div className="flex items-center justify-between gap-3 rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3.5 py-2.5">
            <div className="min-w-0">
              <div className="text-[0.5625rem] font-semibold uppercase tracking-[0.14em] text-[color:var(--mcdl-text-muted)]">{ts("摄像头")}</div>
              <div className="mt-0.5 truncate text-[0.75rem] font-medium">
                {cameraState === "ready" ? ts("预览已就绪") : cameraState === "requesting" ? ts("正在请求权限") : cameraState === "error" ? ts("权限未打开") : ts("尚未接入")}
              </div>
            </div>
            <button
              type="button"
              onClick={() => { if (cameraState === "ready") closeCameraPreview(); else void openCamera(); }}
              disabled={cameraState === "requesting" || isStarting}
              className="inline-flex shrink-0 items-center gap-1 rounded-full border border-[color:var(--mcdl-border-ghost)] px-3 py-1.5 text-[0.625rem] text-[color:var(--mcdl-text-body)] transition hover:border-[color:var(--mcdl-hover-border)] hover:bg-[color:var(--mcdl-hover-surface)] disabled:opacity-50"
            >
              {cameraState === "ready" ? <CameraOff size={12} /> : null}
              {cameraState === "ready" ? ts("关闭预览") : cameraState === "error" ? ts("重试") : ts("开启预览")}
            </button>
          </div>
          {cameraError ? <p className="rounded-xl bg-[color:var(--mcdl-color-negative)]/10 px-3 py-2 text-[0.625rem] leading-5 text-[color:var(--mcdl-color-negative)]">{ts(cameraError)}</p> : null}

          <div className="rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3.5 py-2.5">
            <div className="flex items-center justify-between">
              <span className="text-[0.5625rem] font-semibold uppercase tracking-[0.14em] text-[color:var(--mcdl-text-muted)]">{ts("输出清晰度")}</span>
              <span className="text-[0.5625rem] text-[color:var(--mcdl-text-muted)]">{ts("合成画面")}</span>
            </div>
            <div className="mt-2 grid grid-cols-3 gap-1.5">
              {resolutionOptions.map((item) => {
                const active = resolution === item;
                return (
                  <button
                    key={item}
                    type="button"
                    disabled={phase !== "idle"}
                    onClick={() => setResolution(item)}
                    className={`rounded-xl border px-1.5 py-2 text-center transition disabled:opacity-60 ${active ? "border-[color:var(--mcdl-accent-border)] bg-[color:var(--mcdl-accent-soft)] text-[color:var(--mcdl-color-accent)]" : "border-[color:var(--mcdl-border-ghost)] text-[color:var(--mcdl-text-body)] hover:border-[color:var(--mcdl-hover-border)]"}`}
                  >
                    <span className="block text-[0.75rem] font-semibold">{ts(RESOLUTION_LABEL[item] || item)}</span>
                    <span className={`block text-[0.5625rem] ${active ? "text-[color:var(--mcdl-color-accent)]/75" : "text-[color:var(--mcdl-text-muted)]"}`}>{RESOLUTION_DETAIL[item] || item}</span>
                  </button>
                );
              })}
            </div>
          </div>

          <button
            type="button"
            onClick={() => setRecordMic((value) => !value)}
            className="flex w-full items-center justify-between rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3.5 py-2.5 text-left"
          >
            <span className="flex items-center gap-2 text-[0.75rem] text-[color:var(--mcdl-text-body)]">
              {recordMic ? <Mic size={14} className="text-[color:var(--mcdl-color-accent)]" /> : <MicOff size={14} className="text-[color:var(--mcdl-text-muted)]" />}
              {ts("片段收录原声")}
            </span>
            <span className={`rounded-full px-2 py-0.5 text-[0.5625rem] font-medium ${recordMic ? "bg-[color:var(--mcdl-color-positive)]/12 text-[color:var(--mcdl-color-positive)]" : "bg-[color:var(--mcdl-color-surface-card)] text-[color:var(--mcdl-text-muted)]"}`}>{recordMic ? ts("开") : ts("关")}</span>
          </button>

          <div className="rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3.5 py-3">
            <div className="flex items-baseline justify-between gap-2">
              <span className="text-[0.5625rem] font-semibold uppercase tracking-[0.14em] text-[color:var(--mcdl-text-muted)]">{ts("计费")}</span>
              <span className="tabular-nums text-[1.0625rem] font-semibold [font-family:var(--mcdl-font-numeric)]">{price.toFixed(2)}<span className="ml-1 text-[0.5625rem] font-normal text-[color:var(--mcdl-text-muted)]">{ts("算力/分钟")}</span></span>
            </div>
            <p className="mt-1.5 text-[0.625rem] leading-5 text-[color:var(--mcdl-text-muted)]">
              {ts("开播时冻结 1 分钟额度，结束后按实际秒数结算。单场最长 {n}。").replace("{n}", formatClock(maxSeconds))}
            </p>
          </div>

          <button
            type="button"
            onClick={primaryAction}
            disabled={!reloadNeeded && !canStartHere}
            className="mcdl-button-primary mcdl-hover-primary-action inline-flex w-full items-center justify-center gap-2 px-4 py-2.5 text-[0.8125rem] font-semibold disabled:cursor-not-allowed disabled:opacity-50"
          >
            {isStarting ? <Loader2 size={14} className="animate-spin" /> : <Sparkles size={14} />}
            {primaryLabel}
          </button>
          {isStarting ? (
            <button type="button" onClick={() => void cancelStarting()} className="w-full rounded-full border border-[color:var(--mcdl-border-ghost)] px-3 py-2 text-[0.6875rem] text-[color:var(--mcdl-text-body)] transition hover:bg-[color:var(--mcdl-hover-surface)]">
              {ts("取消这次开播")}
            </button>
          ) : null}
          <p className="pt-1 text-center text-[0.625rem] leading-5 text-[color:var(--mcdl-text-muted)]">
            {ts("开播后按 1–9 换角，R 抓取片段；点画面右下角或按 Esc 结束会话。")}
          </p>
        </div>
      </div>
    );
  };

  const renderCastCard = (character: CharacterStudioCharacter, index: number) => {
    const active = currentCharacter?.public_id === character.public_id;
    const switching = switchingId === character.public_id;
    return (
      <div key={character.public_id} className="group relative w-[6.4rem] shrink-0">
        <button
          type="button"
          onClick={() => void switchTo(character)}
          disabled={phase === "starting" || phase === "wrapping"}
          className={`character-studio-card relative block w-full overflow-hidden rounded-[1.15rem] border text-left transition disabled:cursor-not-allowed ${active ? "border-[color:var(--mcdl-accent-border)] shadow-[var(--mcdl-shadow-soft)]" : "border-[color:var(--mcdl-border-ghost)] hover:border-[color:var(--mcdl-hover-border)]"}`}
          aria-label={phase === "live" ? ts("切换到这个角色") : ts("选择这个角色")}
        >
          <div className="relative aspect-[3/4] w-full overflow-hidden bg-[color:var(--mcdl-color-surface-subtle)]">
            {character.image_url ? (
              <img src={character.image_url} alt={character.name} className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]" />
            ) : (
              <div className="flex h-full items-center justify-center"><VenetianMask size={26} className="text-[color:var(--mcdl-text-muted)]" /></div>
            )}
            <div className="absolute inset-0 bg-gradient-to-t from-[#120d0a]/85 via-transparent to-transparent" />
            {active ? <span className="absolute left-2 top-2 rounded-full bg-[color:var(--mcdl-color-accent)] px-2 py-0.5 text-[0.5625rem] font-semibold text-[color:var(--mcdl-color-on-accent)]">{phase === "live" ? ts("上场") : ts("已选")}</span> : null}
            {switching ? <span className="absolute inset-0 flex items-center justify-center bg-black/45 text-[0.625rem] font-medium text-white">{ts("重绘中…")}</span> : null}
            {phase === "live" && index < 9 ? <span className="absolute right-2 top-2 rounded bg-black/45 px-1.5 py-0.5 font-mono text-[0.5625rem] text-white/80">{index + 1}</span> : null}
            <div className="absolute inset-x-0 bottom-0 px-2.5 pb-2.5 pt-6">
              <div className="truncate text-[0.75rem] font-semibold text-white">{character.name}</div>
              <div className="truncate text-[0.5625rem] text-white/65">{active ? (phase === "live" ? ts("正在扮演") : ts("已选中")) : ts("角色卡")}</div>
            </div>
          </div>
        </button>
        <button
          type="button"
          onClick={() => openEditCharacter(character)}
          className="mcdl-hover-quiet absolute -right-1.5 -top-1.5 grid h-6 w-6 place-items-center rounded-full border border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface-card)] text-[color:var(--mcdl-text-body)] opacity-0 shadow-sm transition group-hover:opacity-100 focus:opacity-100"
          aria-label={ts("编辑角色卡")}
        >
          <Sparkles size={11} />
        </button>
      </div>
    );
  };

  const defaultRoleBase = (config?.default_role_base_url || "").trim().replace(/\/+$/, "");
  const defaultRoleSource = (role: CharacterStudioDefaultRole) => (
    /^https?:\/\//i.test(defaultRoleBase) ? `${defaultRoleBase}/${role.fileName}` : role.src
  );

  const applyDefaultRole = async (role: CharacterStudioDefaultRole) => {
    if (defaultRoleLoading || phase === "starting" || phase === "wrapping") return;
    const existing = characters.find((item) => item.image_url.includes(role.fileName));
    if (existing) {
      void switchTo(existing);
      showNotice(ts("已选用默认角色"));
      return;
    }
    const publicSource = defaultRoleSource(role);
    const usePublicSource = /^https?:\/\//i.test(publicSource) && publicSource !== role.src;
    setDefaultRoleLoading(role.key);
    try {
      if (usePublicSource) {
        const created = await createCharacterStudioCharacter({ name: role.name, image_url: publicSource, rights_confirmed: true });
        setCharacters((previous) => [created, ...previous.filter((item) => item.public_id !== created.public_id)]);
        void switchTo(created);
        showNotice(ts("默认角色已加入并选中"));
        return;
      }
      const response = await fetch(role.src);
      if (!response.ok) throw new Error(ts("默认角色图片加载失败"));
      const blob = await response.blob();
      const file = new File([blob], role.fileName, { type: blob.type || "image/jpeg" });
      const asset = await uploadAsset(file, { kind: "image", name: role.fileName });
      const created = await createCharacterStudioCharacter({ name: role.name, image_url: asset.url, rights_confirmed: true });
      setCharacters((previous) => [created, ...previous.filter((item) => item.public_id !== created.public_id)]);
      void switchTo(created);
      showNotice(ts("默认角色已加入并选中"));
    } catch (error) {
      showNotice(errorText(error, ts("默认角色加载失败，请重试")));
    } finally {
      setDefaultRoleLoading("");
    }
  };

  const renderDefaultRoleCard = (role: CharacterStudioDefaultRole) => {
    const categoryLabel = CHARACTER_STUDIO_DEFAULT_CATEGORY_LABELS[role.category];
    const active = Boolean(selectedCharacter?.image_url.includes(role.fileName));
    const loading = defaultRoleLoading === role.key;
    return (
      <button
        key={role.key}
        type="button"
        onClick={() => void applyDefaultRole(role)}
        disabled={Boolean(defaultRoleLoading) || phase === "starting" || phase === "wrapping"}
        aria-label={`${ts("使用默认角色")} ${role.name} · ${ts(categoryLabel)}`}
        className={`group relative h-[8.5rem] w-[6.4rem] shrink-0 overflow-hidden rounded-[1.15rem] border text-left transition disabled:cursor-not-allowed ${active ? "border-[color:var(--mcdl-accent-border)] shadow-[var(--mcdl-shadow-soft)] ring-1 ring-[color:var(--mcdl-accent-soft)]" : "border-[color:var(--mcdl-border-ghost)] hover:border-[color:var(--mcdl-hover-border)]"}`}
      >
        <img src={defaultRoleSource(role)} alt={role.name} loading="lazy" className="h-full w-full object-cover transition duration-500 group-hover:scale-[1.03]" />
        <span className="absolute left-1.5 top-1.5 rounded-full bg-black/45 px-1.5 py-0.5 text-[0.5rem] text-white/90 backdrop-blur">{ts(categoryLabel)}</span>
        {active && !loading ? <span className="absolute right-1.5 top-1.5 grid h-5 w-5 place-items-center rounded-full bg-[color:var(--mcdl-color-accent)] text-[color:var(--mcdl-color-on-accent)]"><Check size={11} /></span> : null}
        {loading ? <span className="absolute inset-0 grid place-items-center bg-black/45"><Loader2 size={18} className="animate-spin text-white" /></span> : null}
        <span className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-[#120d0a]/85 via-transparent to-transparent px-2.5 pb-2.5 pt-8">
          <span className="block truncate text-[0.75rem] font-semibold text-white">{role.name}</span>
          <span className="block truncate text-[0.5625rem] text-white/65">{active ? ts("已选中") : ts("点击试用")}</span>
        </span>
      </button>
    );
  };

  return (
    <div className="mcdl-workspace character-studio-workspace flex h-full min-h-0 flex-col text-[color:var(--mcdl-text-title)]">
      <header className="flex h-[73px] shrink-0 items-center gap-3 border-b border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface)] px-4 lg:pr-48">
        <div className="flex min-w-0 items-center gap-3">
          <div className="grid h-9 w-9 shrink-0 place-items-center rounded-2xl bg-[color:var(--mcdl-accent-soft)] text-[color:var(--mcdl-color-accent)]">
            <VenetianMask size={18} />
          </div>
          <div className="min-w-0">
            <div className="hidden items-center gap-1.5 text-[0.5625rem] font-semibold uppercase tracking-[0.18em] text-[color:var(--mcdl-text-muted)] sm:flex">
              <span className="h-1 w-1 rounded-full bg-[color:var(--mcdl-color-accent)]" />
              OPEN REALTIME PIPELINE
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="mcdl-display truncate text-[1.0625rem] leading-tight sm:text-[1.25rem]">{ts("角色替换台")}</h1>
              <StatusPill label={statusLabel} tone={statusTone} />
              {rehearse ? <span className="hidden rounded-full border border-[color:var(--mcdl-border-ghost)] px-2 py-0.5 text-[0.5625rem] text-[color:var(--mcdl-text-muted)] sm:inline">{ts("本地演练")}</span> : null}
            </div>
          </div>
        </div>
        <div className="ml-auto flex shrink-0 items-center gap-2">
          <span className="hidden text-[0.6875rem] tabular-nums text-[color:var(--mcdl-text-body)] xl:inline">{price.toFixed(2)} {ts("算力/分钟")}</span>
          <button type="button" onClick={() => setDossierOpen(true)} className="mcdl-button-secondary mcdl-hover-control inline-flex items-center gap-1.5 px-3 py-1.5 text-[0.6875rem]">
            <Info size={13} />
            <span className="hidden sm:inline">{ts("技术档案")}</span>
          </button>
          {phase === "live" ? (
            <button type="button" onClick={() => setConfirmEndOpen(true)} aria-label={ts("结束当前会话")} className="inline-flex items-center gap-1.5 rounded-full border border-[color:var(--mcdl-color-negative)]/30 bg-[color:var(--mcdl-color-negative)]/8 px-3.5 py-1.5 text-[0.6875rem] font-semibold text-[color:var(--mcdl-color-negative)] transition hover:bg-[color:var(--mcdl-color-negative)]/14">
              <PhoneOff size={13} />
              {ts("结束会话")}
            </button>
          ) : null}
        </div>
      </header>

      <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
        <section className="shrink-0 border-b border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface)] px-4 pb-3 pt-3 sm:px-6">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex min-w-0 flex-wrap items-center gap-2.5">
              <div className="mcdl-eyebrow">{ts("角色架")}</div>
              <div className="flex rounded-full bg-[color:var(--mcdl-color-surface-subtle)] p-0.5 text-[0.625rem]">
                <button type="button" onClick={() => setRackTab("default")} className={`rounded-full px-2.5 py-1 transition ${rackTab === "default" ? "bg-[color:var(--mcdl-color-surface-card)] font-semibold text-[color:var(--mcdl-text-title)] shadow-sm" : "text-[color:var(--mcdl-text-muted)]"}`}>
                  {ts("默认角色")} <span className="text-[0.5625rem] opacity-70">{CHARACTER_STUDIO_DEFAULT_ROLES.length}</span>
                </button>
                <button type="button" onClick={() => setRackTab("mine")} className={`rounded-full px-2.5 py-1 transition ${rackTab === "mine" ? "bg-[color:var(--mcdl-color-surface-card)] font-semibold text-[color:var(--mcdl-text-title)] shadow-sm" : "text-[color:var(--mcdl-text-muted)]"}`}>
                  {ts("我的角色")} <span className="text-[0.5625rem] opacity-70">{characters.length}</span>
                </button>
              </div>
              <span className="hidden text-[0.625rem] text-[color:var(--mcdl-text-muted)] sm:inline">{rackTab === "default" ? ts("平台提供，点一下即可加入并选中") : ts("选一张身份卡，开播后可随时换角")}</span>
              <span className="hidden items-center gap-1 rounded-full bg-[color:var(--mcdl-color-surface-subtle)] px-2 py-0.5 text-[0.5625rem] text-[color:var(--mcdl-text-muted)] lg:inline-flex">
                <ShieldCheck size={11} className="text-[color:var(--mcdl-color-positive)]" />
                {ts("只使用本人或已获授权的形象")}
              </span>
            </div>
            <button type="button" onClick={openCreateCharacter} className="inline-flex shrink-0 items-center gap-1.5 rounded-full bg-[color:var(--mcdl-color-accent)] px-3 py-1.5 text-[0.625rem] font-semibold text-[color:var(--mcdl-color-on-accent)] transition hover:brightness-105">
              <Plus size={13} />
              {ts("新建角色")}
            </button>
          </div>
          <div className="mt-2.5 flex gap-2.5 overflow-x-auto pb-1">
            {rackTab === "default" ? (
              CHARACTER_STUDIO_DEFAULT_ROLES.map(renderDefaultRoleCard)
            ) : (
              <>
                {characters.map((character, index) => renderCastCard(character, index))}
                <button
                  type="button"
                  onClick={openCreateCharacter}
                  className="flex h-[8.5rem] w-[6.4rem] shrink-0 flex-col items-center justify-center gap-2 rounded-[1.15rem] border border-dashed border-[color:var(--mcdl-border-ghost)] text-[color:var(--mcdl-text-muted)] transition hover:border-[color:var(--mcdl-hover-border)] hover:bg-[color:var(--mcdl-hover-surface)]"
                >
                  <Plus size={17} />
                  <span className="text-[0.625rem]">{ts("新建角色")}</span>
                </button>
              </>
            )}
          </div>
          {rackTab === "default" ? (
            <p className="mt-1.5 text-[0.5625rem] leading-4 text-[color:var(--mcdl-text-muted)]">{ts("默认角色由平台提供；动物/卡通效果受主体识别影响。")}</p>
          ) : null}
        </section>

        <div className="grid min-h-0 flex-1 grid-cols-1 xl:grid-cols-[minmax(0,1fr)_20rem]">
        <section className="flex min-h-0 flex-col px-4 py-4 sm:px-6 sm:py-5">
          {phase !== "wrapped" && phase !== "failed" ? (
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2 text-[0.6875rem] text-[color:var(--mcdl-text-muted)]">
                <Radio size={13} className={phase === "live" ? "text-[color:var(--mcdl-color-positive)]" : ""} />
                <span>{phase === "live" ? ts("合成监看台") : phase === "starting" ? ts("正在接入渲染节点") : ts("排练监看台")}</span>
                {currentCharacter ? <span className="truncate">· {currentCharacter.name}</span> : null}
                {rehearse ? <span className="rounded-full border border-[color:var(--mcdl-border-ghost)] px-1.5 py-0.5 text-[0.5625rem] text-[color:var(--mcdl-text-muted)] sm:hidden">{ts("本地演练")}</span> : null}
              </div>
              <div className="flex items-center gap-2">
                <button type="button" onClick={() => setSignalOpen(true)} className="mcdl-button-secondary mcdl-hover-control inline-flex items-center gap-1.5 px-3 py-1.5 text-[0.6875rem] xl:hidden">
                  <Layers size={13} />
                  {phase === "live" || phase === "wrapping" ? ts("信号") : ts("设置")}
                </button>
              </div>
            </div>
          ) : null}

          {phase !== "wrapped" && phase !== "failed" ? (
            <>
          <div className="relative mt-3 flex min-h-[16rem] flex-none items-start justify-center">
            <div
              className={`character-studio-stage relative w-full max-w-[62rem] overflow-hidden rounded-[var(--mcdl-radius-stage)] ${phase === "starting" ? "character-studio-scanning" : ""} ${phase === "idle" || phase === "starting" ? "min-h-[19rem] sm:min-h-0" : ""}`}
              style={{ aspectRatio: stageAspect, maxHeight: "100%" }}
            >
              <video
                ref={remoteVideoRef}
                autoPlay
                playsInline
                muted
                aria-label={ts("合成画面")}
                className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-500 ${remoteReady ? "opacity-100" : "opacity-0"}`}
              />

              <div
                className={`absolute overflow-hidden rounded-2xl transition-all duration-500 ${
                  phase === "live"
                    ? compare
                      ? "inset-0 z-20 rounded-none"
                      : "right-3 top-3 z-20 h-[5rem] w-[3.6rem] border border-white/30 shadow-[0_0.75rem_2rem_rgba(0,0,0,0.35)] sm:h-[6.5rem] sm:w-[4.6rem]"
                    : "inset-0 z-0 rounded-none"
                }`}
                style={compare && phase === "live" ? { clipPath: `inset(0 ${100 - compareValue}% 0 0)` } : undefined}
              >
                <video
                  ref={localVideoRef}
                  autoPlay
                  playsInline
                  muted
                  aria-label={phase === "live" ? ts("原始画面") : ts("摄像头预览")}
                  onLoadedMetadata={handleLocalVideoMeta}
                  className={`h-full w-full object-cover ${cameraState === "ready" ? "opacity-100" : "opacity-0"} ${mirror && !compare ? "-scale-x-100" : ""}`}
                />
                {compare && phase === "live" ? <div className="absolute left-3 top-3 rounded-full bg-black/50 px-2 py-0.5 text-[0.5625rem] text-white/85">{ts("原始")}</div> : null}
              </div>

              {compare && phase === "live" ? (
                <>
                  <div className="pointer-events-none absolute inset-y-0 z-30 w-px bg-white/75 shadow-[0_0_1.25rem_rgba(0,0,0,0.5)]" style={{ left: `${compareValue}%` }} />
                  <span className="pointer-events-none absolute right-3 top-3 z-30 rounded-full bg-black/50 px-2 py-0.5 text-[0.5625rem] text-white/85">{ts("合成")}</span>
                  <input
                    type="range"
                    min={4}
                    max={96}
                    value={compareValue}
                    onChange={(event) => setCompareValue(Number(event.target.value))}
                    aria-label={ts("原始与合成画面拖拽对比")}
                    className="character-studio-compare absolute inset-x-0 bottom-3 z-40 mx-auto w-[72%] cursor-ew-resize"
                  />
                </>
              ) : null}

              {!remoteReady || phase !== "live" ? (
                <div className="pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_35%,rgba(255,135,17,0.14),transparent_55%)]" />
              ) : null}

              {phase === "idle" && cameraState !== "ready" ? (
                <div className="absolute inset-0 z-10 flex flex-col items-center justify-center px-6 text-center">
                  <div className="grid h-16 w-16 place-items-center rounded-[1.4rem] border border-white/12 bg-white/[0.06] text-white/85">
                    <Aperture size={26} />
                  </div>
                  <h2 className="character-studio-stage-title mcdl-display mt-5 max-w-[20rem] text-[1.2rem] leading-tight sm:max-w-md sm:text-[1.55rem]">{ts("换一个身份，保留你的每一个动作")}</h2>
                  <p className="mt-2 max-w-md text-[0.75rem] leading-6 text-white/60">
                    {ts("接入摄像头，选一张角色卡，合成画面会在这里实时出现。原始画面会一直保留，方便你随时对照。")}
                  </p>
                  <button type="button" onClick={() => void openCamera()} className="mcdl-button-primary mcdl-hover-primary-action mt-5 inline-flex items-center gap-2 px-5 py-2.5 text-[0.75rem] font-semibold">
                    <Aperture size={14} />
                    {cameraState === "requesting" ? ts("正在请求摄像头") : ts("接入摄像头预览")}
                  </button>
                  {cameraError ? <p className="mt-3 max-w-sm text-[0.6875rem] leading-5 text-[color:var(--mcdl-color-negative)]">{ts(cameraError)}</p> : null}
                </div>
              ) : null}

              {phase === "idle" && cameraState === "ready" ? (
                <div className="absolute inset-0 z-20 bg-gradient-to-t from-black/55 via-transparent to-black/10">
                  <div className="absolute left-3 top-3 flex items-center gap-1.5 rounded-full bg-black/40 px-2.5 py-1 text-[0.5625rem] text-white/80 backdrop-blur">
                    <span className="h-1.5 w-1.5 rounded-full bg-[color:var(--mcdl-color-accent)]" />
                    {ts("排练 · 未接入渲染")}
                  </div>
                  <div className="absolute inset-x-0 bottom-0 p-4">
                    <div className="flex flex-wrap items-center justify-between gap-3">
                      <div>
                        <div className="text-[0.75rem] font-semibold text-white">{currentCharacter ? `${ts("准备扮演")} ${currentCharacter.name}` : ts("先选一张角色卡")}</div>
                        <div className="mt-0.5 text-[0.625rem] text-white/60">{ts("开播后，这里会切换成合成画面")}</div>
                      </div>
                      <div className="flex flex-wrap items-center gap-2">
                        <button type="button" onClick={closeCameraPreview} className="inline-flex items-center gap-1.5 rounded-full bg-white/12 px-3.5 py-2.5 text-[0.6875rem] text-white/80 transition hover:bg-white/20">
                          <CameraOff size={13} />
                          {ts("关闭预览")}
                        </button>
                        <button type="button" onClick={() => void startSession()} disabled={!canStart} className="mcdl-button-primary mcdl-hover-primary-action inline-flex items-center gap-2 px-5 py-2.5 text-[0.75rem] font-semibold disabled:opacity-50">
                          <Sparkles size={14} />
                          {ts("开始换角")}
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              ) : null}

              {phase === "starting" ? (
                <div className="absolute inset-0 z-40 flex items-center justify-center bg-[#0d0b0a]/76 px-6 backdrop-blur-sm">
                  <div className="w-full max-w-sm rounded-[1.35rem] border border-white/10 bg-white/[0.06] p-5 text-white">
                    <div className="flex items-center gap-2 text-[0.6875rem] font-semibold uppercase tracking-[0.16em] text-white/60">
                      <Loader2 size={13} className="animate-spin" />
                      {ts("启动序列")}
                    </div>
                    <h3 className="mt-2 text-[1.05rem] font-semibold">{ts("正在接入渲染节点")}</h3>
                    <div className="mt-4 space-y-2">
                      {startupSteps.map((step) => (
                        <div key={step.label} className="flex items-center gap-2.5 text-[0.75rem]">
                          <span className={`grid h-4 w-4 place-items-center rounded-full border ${step.done ? "border-[color:var(--mcdl-color-positive)] bg-[color:var(--mcdl-color-positive)]/15 text-[color:var(--mcdl-color-positive)]" : "border-white/20 text-white/30"}`}>
                            {step.done ? <Check size={10} /> : <span className="h-1 w-1 rounded-full bg-white/40" />}
                          </span>
                          <span className={step.done ? "text-white/85" : "text-white/50"}>{step.label}</span>
                        </div>
                      ))}
                    </div>
                    <button type="button" onClick={() => void cancelStarting()} className="mt-5 w-full rounded-full border border-white/15 px-3 py-2 text-[0.6875rem] text-white/70 transition hover:bg-white/10">
                      {ts("取消这次开播")}
                    </button>
                  </div>
                </div>
              ) : null}

              {phase === "live" && !remoteReady ? (
                <div className="pointer-events-none absolute inset-0 z-30 flex items-center justify-center bg-[#0d0b0a]/55">
                  <div className="rounded-full bg-black/45 px-4 py-2 text-[0.75rem] text-white/80 backdrop-blur">
                    {ts("实时通道已接入，等待首帧画面")}
                  </div>
                </div>
              ) : null}

              {phase === "wrapping" ? (
                <div className="absolute inset-0 z-50 flex items-center justify-center bg-[#0d0b0a]/65 backdrop-blur-sm">
                  <div className="flex items-center gap-2 rounded-full bg-black/50 px-4 py-2 text-[0.75rem] text-white/85">
                    <Loader2 size={14} className="animate-spin" />
                    {ts("正在结算本次会话…")}
                  </div>
                </div>
              ) : null}

              {switchingId ? (
                <div className="pointer-events-none absolute inset-x-0 top-3 z-40 flex justify-center">
                  <div className="flex items-center gap-2 rounded-full bg-black/55 px-3 py-1.5 text-[0.6875rem] text-white/85 backdrop-blur">
                    <Loader2 size={12} className="animate-spin" />
                    {ts("正在应用新角色，画面会在数秒内更新")}
                  </div>
                </div>
              ) : null}

              {phase === "live" && stageError ? (
                <div className="absolute inset-0 z-50 flex items-center justify-center bg-[#0d0b0a]/75 px-4 backdrop-blur-[3px]">
                  <div className="w-full max-w-sm rounded-[1.35rem] border border-white/10 bg-white/[0.06] p-5 text-white">
                    <div className="flex items-center gap-1.5 text-[0.625rem] font-semibold uppercase tracking-[0.16em] text-white/55">
                      <Radio size={13} />
                      {ts("实时通道")}
                    </div>
                    <div className="mt-2 text-[1.05rem] font-semibold">{ts("连接没有建立")}</div>
                    <p className="mt-2 text-[0.75rem] leading-6 text-white/60">{stageError}</p>
                    <div className="mt-4 flex flex-wrap gap-2">
                      <button type="button" onClick={() => void retryStage()} className="mcdl-button-primary mcdl-hover-primary-action inline-flex flex-1 items-center justify-center px-4 py-2.5 text-[0.75rem] font-semibold">
                        {ts("重新接入")}
                      </button>
                      <button type="button" onClick={() => setConfirmEndOpen(true)} className="inline-flex items-center justify-center rounded-full border border-white/15 px-4 py-2.5 text-[0.75rem] text-white/70 transition hover:bg-white/10">
                        {ts("结束会话")}
                      </button>
                    </div>
                  </div>
                </div>
              ) : null}

              {phase === "live" && countdown !== null ? (
                <div className="absolute inset-0 z-50 flex items-center justify-center bg-black/45">
                  <div className="text-center">
                    <div className="character-studio-countdown text-[4.5rem] font-semibold leading-none text-white [font-family:var(--mcdl-font-display)]">{countdown > 0 ? countdown : "●"}</div>
                    <div className="mt-2 text-[0.6875rem] text-white/70">{ts("准备表演")}</div>
                  </div>
                </div>
              ) : null}

              {phase === "live" ? (
                <div className="absolute inset-x-0 bottom-0 z-30 bg-gradient-to-t from-black/70 via-black/20 to-transparent px-3 pb-3 pt-10 sm:px-4">
                  <div className="flex flex-wrap items-center gap-2">
                    {recordingState === "recording" ? (
                      <button type="button" onClick={stopRecording} className="inline-flex items-center gap-2 rounded-full bg-[color:var(--mcdl-color-negative)] px-4 py-2 text-[0.6875rem] font-semibold text-white">
                        <span className="character-studio-live-dot h-2 w-2 rounded-full bg-white" />
                        {ts("停止")} {formatClock(recordSeconds)}
                      </button>
                    ) : (
                      <button type="button" onClick={beginTake} disabled={recordingState !== "idle" || !remoteReady} className="inline-flex items-center gap-2 rounded-full bg-white/14 px-4 py-2 text-[0.6875rem] font-medium text-white transition hover:bg-white/20 disabled:opacity-40">
                        {recordingState === "uploading" ? <Loader2 size={13} className="animate-spin" /> : <span className="grid h-3 w-3 place-items-center rounded-full border border-white/80"><span className="h-1.5 w-1.5 rounded-full bg-[color:var(--mcdl-color-negative)]" /></span>}
                        {recordingState === "uploading" ? ts("保存片段中") : ts("抓取片段")}
                      </button>
                    )}
                    <button type="button" aria-label={ts("对比")} onClick={() => setCompare((value) => !value)} className={`inline-flex items-center gap-1.5 rounded-full px-3 py-2 text-[0.6875rem] transition ${compare ? "bg-white text-[#1b1715]" : "bg-white/12 text-white/80 hover:bg-white/20"}`}>
                      <Contrast size={14} />
                      <span className="hidden sm:inline">{ts("对比")}</span>
                    </button>
                    <button type="button" aria-label={ts("镜像")} onClick={() => setMirror((value) => !value)} className="inline-flex items-center gap-1.5 rounded-full bg-white/12 px-3 py-2 text-[0.6875rem] text-white/80 transition hover:bg-white/20">
                      <RefreshCw size={14} />
                      <span className="hidden sm:inline">{ts("镜像")}</span>
                    </button>
                    <button type="button" aria-label={recordMic ? ts("收录原声") : ts("仅画面")} onClick={() => setRecordMic((value) => !value)} className="inline-flex items-center gap-1.5 rounded-full bg-white/12 px-3 py-2 text-[0.6875rem] text-white/80 transition hover:bg-white/20">
                      {recordMic ? <Mic size={14} /> : <MicOff size={14} />}
                      <span className="hidden sm:inline">{recordMic ? ts("收录原声") : ts("仅画面")}</span>
                    </button>
                    <button type="button" aria-label={ts("切换摄像头")} onClick={() => void stageRef.current?.switchCamera()} className="ml-auto inline-flex items-center gap-1.5 rounded-full bg-white/12 px-3 py-2 text-[0.6875rem] text-white/80 transition hover:bg-white/20">
                      <RefreshCw size={14} />
                      <span className="hidden sm:inline">{ts("切换摄像头")}</span>
                    </button>
                    <button type="button" onClick={() => setConfirmEndOpen(true)} aria-label={ts("结束当前会话")} className="inline-flex items-center gap-1.5 rounded-full bg-[color:var(--mcdl-color-negative)] px-3.5 py-2 text-[0.6875rem] font-semibold text-white transition hover:brightness-105">
                      <PhoneOff size={13} />
                      {ts("结束会话")}
                    </button>
                  </div>
                </div>
              ) : null}
            </div>
          </div>

          {phase === "idle" || phase === "starting" ? (
            <div className="mt-4">
              <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-5">
                {[
                  { title: ts("采集"), body: ts("低延迟上行") },
                  { title: ts("身份锚定"), body: ts("参考形象约束") },
                  { title: ts("流式重绘"), body: ts("逐帧保留动作") },
                  { title: ts("时域稳定"), body: ts("抑制闪烁漂移") },
                  { title: ts("合成输出"), body: ts("实时流回传") },
                ].map((item, index) => (
                  <div key={item.title} className="rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3 py-2.5">
                    <div className="flex items-center gap-1.5">
                      <span className="font-mono text-[0.5625rem] text-[color:var(--mcdl-color-accent)]">0{index + 1}</span>
                      <span className="text-[0.6875rem] font-semibold text-[color:var(--mcdl-text-body)]">{item.title}</span>
                    </div>
                    <div className="mt-0.5 text-[0.5625rem] leading-4 text-[color:var(--mcdl-text-muted)]">{item.body}</div>
                  </div>
                ))}
              </div>
              <div className="mt-2.5 flex flex-wrap items-center justify-between gap-2 rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3.5 py-2.5">
                <p className="text-[0.625rem] leading-5 text-[color:var(--mcdl-text-muted)]">
                  {ts("开放视频模型内核 · 自研实时工程链路 · 项目代码开源")}
                </p>
                <button type="button" onClick={() => setDossierOpen(true)} className="inline-flex items-center gap-1 text-[0.625rem] font-medium text-[color:var(--mcdl-color-accent)]">
                  {ts("了解实现")} <ChevronRight size={11} />
                </button>
              </div>
            </div>
          ) : null}
            </>
          ) : null}

          {phase === "live" ? (
            <div className="mt-3 space-y-2">
              <div className="grid grid-cols-3 gap-2 xl:hidden">
                <Metric label={ts("已运行")} value={formatClock(elapsed)} />
                <Metric label={ts("算力")} value={formatCost(estimatedCost)} />
                <Metric label={ts("换角")} value={String(session?.switch_count ?? 0)} />
              </div>
              {takes.length === 0 ? (
                <div className="flex flex-wrap items-center justify-between gap-2 rounded-2xl border border-dashed border-[color:var(--mcdl-border-ghost)] px-3.5 py-2.5">
                  <span className="text-[0.6875rem] text-[color:var(--mcdl-text-body)]">{ts("镜头带")} · {ts("还没有片段")}</span>
                  <span className="text-[0.625rem] text-[color:var(--mcdl-text-muted)]">{ts("按 REC 抓取 · 每段最长 60 秒")}</span>
                </div>
              ) : null}
            </div>
          ) : null}

          {phase === "wrapped" || phase === "failed" ? (
            <div className="mt-4 rounded-[var(--mcdl-radius-card)] bg-[color:var(--mcdl-color-surface-card)] p-5 shadow-[var(--mcdl-shadow-soft)] sm:p-6">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <div className="mcdl-eyebrow">{phase === "wrapped" ? ts("本次收工") : ts("开播未完成")}</div>
                  <h2 className="mcdl-display mt-2 text-[1.35rem] leading-tight">{phase === "wrapped" ? ts("这条管线跑完了") : ts("这次没有成功开播")}</h2>
                  <p className="mt-1.5 max-w-lg text-[0.75rem] leading-6 text-[color:var(--mcdl-text-body)]">
                    {ts(session?.end_reason || (phase === "wrapped" ? "片段已保存，可以下载或去我的作品查看。" : session?.error || "请稍后重试，或检查角色卡与摄像头。"))}
                  </p>
                </div>
                <div className="flex gap-2">
                  <button type="button" onClick={resetStudio} className="mcdl-button-secondary mcdl-hover-control px-4 py-2 text-[0.75rem]">{ts("再来一条")}</button>
                  <a href="/app/works" className="mcdl-button-primary mcdl-hover-primary-action inline-flex items-center px-4 py-2 text-[0.75rem] font-semibold">{ts("查看作品")}</a>
                </div>
              </div>
              <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
                <Metric label={ts("时长")} value={formatClock(session?.billed_seconds || session?.elapsed_seconds || elapsed)} />
                <Metric label={ts("算力")} value={formatCost(session?.cost || 0)} />
                <Metric label={ts("换角")} value={String(session?.switch_count ?? 0)} />
                <Metric label={ts("片段")} value={String(takes.length)} />
              </div>
              {takes.length > 0 ? (
                <div className="mt-4 grid grid-cols-2 gap-2.5 sm:grid-cols-3">
                  {takes.map((take) => (
                    <div key={take.key} className="overflow-hidden rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)]">
                      <div className="relative aspect-video bg-black/20">
                        {take.thumb ? <img src={take.thumb} alt="" className="h-full w-full object-cover" /> : <div className="flex h-full items-center justify-center text-[color:var(--mcdl-text-muted)]"><Film size={18} /></div>}
                        <span className="absolute bottom-1.5 left-1.5 rounded bg-black/55 px-1.5 py-0.5 font-mono text-[0.5625rem] text-white">{takeDurationLabel(take.durationMs)}</span>
                      </div>
                      <div className="flex items-center justify-between gap-2 px-2.5 py-2">
                        <span className="truncate text-[0.625rem] text-[color:var(--mcdl-text-body)]">{take.status === "saved" ? ts("已存入作品") : take.status === "failed" ? ts("仅本地") : take.status === "local" ? ts("演练片段") : ts("保存中")}</span>
                        <a href={take.url} download={`character-studio-${take.key}`} className="text-[color:var(--mcdl-color-accent)]" aria-label={ts("下载片段")}><Download size={13} /></a>
                      </div>
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          ) : null}

          {takes.length > 0 && phase !== "wrapped" && phase !== "failed" ? (
            <div className="mt-4">
              <div className="flex items-center justify-between">
                <span className="mcdl-eyebrow">{ts("镜头带")}</span>
                <span className="text-[0.625rem] text-[color:var(--mcdl-text-muted)]">{takes.length} {ts("段")} · {ts("每段最长 60 秒")}</span>
              </div>
              <div className="mt-2 flex gap-2 overflow-x-auto pb-1">
                {takes.map((take) => (
                  <div key={take.key} className="w-[7.5rem] shrink-0 overflow-hidden rounded-xl border border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface-subtle)]">
                    <div className="relative aspect-video bg-black/20">
                      {take.thumb ? <img src={take.thumb} alt="" className="h-full w-full object-cover" /> : <div className="flex h-full items-center justify-center text-[color:var(--mcdl-text-muted)]"><Film size={14} /></div>}
                      <span className="absolute bottom-1 right-1 rounded bg-black/55 px-1 py-0.5 font-mono text-[0.5rem] text-white">{takeDurationLabel(take.durationMs)}</span>
                    </div>
                    <div className="flex items-center justify-between px-2 py-1.5">
                      <span className="text-[0.5625rem] text-[color:var(--mcdl-text-muted)]">{take.status === "saved" ? ts("已存入作品") : take.status === "failed" ? ts("仅本地") : take.status === "local" ? ts("演练") : ts("保存中")}</span>
                      <a href={take.url} download={`character-studio-${take.key}`} aria-label={ts("下载片段")} className="text-[color:var(--mcdl-text-body)]"><Download size={12} /></a>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          ) : null}
        </section>

        <aside className="hidden min-h-0 overflow-y-auto border-l border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface)] px-4 py-4 xl:flex xl:flex-col">
          {phase === "live" || phase === "wrapping" ? renderSignalPanel() : renderSetupPanel()}
        </aside>
        </div>
      </div>

      {mounted && signalOpen ? createPortal(
        <div className="fixed inset-0 z-50 flex items-end justify-end p-0 sm:items-stretch sm:p-4">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-[3px]" onClick={() => setSignalOpen(false)} />
          <div role="dialog" aria-modal="true" aria-label={ts("信号")} className="mcdl-dialog-surface relative flex max-h-[85dvh] w-full flex-col overflow-y-auto rounded-t-[1.5rem] p-5 sm:ml-auto sm:max-h-full sm:w-[22rem] sm:rounded-[1.5rem]">
            <div className="mb-3 flex items-center justify-between">
              <span className="text-[0.8125rem] font-semibold">{phase === "live" || phase === "wrapping" ? ts("信号面板") : ts("开机检查")}</span>
              <button type="button" onClick={() => setSignalOpen(false)} aria-label={ts("关闭")} className="mcdl-icon-button mcdl-hover-quiet grid h-8 w-8 place-items-center rounded-full"><X size={14} /></button>
            </div>
            {phase === "live" || phase === "wrapping" ? renderSignalPanel(true) : renderSetupPanel(true)}
          </div>
        </div>,
        document.body,
      ) : null}

      {mounted && confirmEndOpen ? createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/55 backdrop-blur-[4px]" onClick={() => setConfirmEndOpen(false)} />
          <div role="dialog" aria-modal="true" aria-labelledby="character-studio-end-title" className="mcdl-dialog-surface relative w-full max-w-md p-6 sm:p-7">
            <div className="mcdl-eyebrow">{ts("结束会话")}</div>
            <h2 id="character-studio-end-title" className="mcdl-display mt-2 text-[1.35rem]">{ts("结束这次会话？")}</h2>
            <p className="mt-2 text-[0.75rem] leading-6 text-[color:var(--mcdl-text-body)]">{ts("结束后会按实际开播秒数结算，角色卡和已抓取的片段都会保留。")}</p>
            <div className="mt-4 grid grid-cols-3 gap-2">
              <Metric label={ts("已运行")} value={formatClock(elapsed)} />
              <Metric label={ts("预计")} value={formatCost(estimatedCost)} />
              <Metric label={ts("片段")} value={String(takes.length)} />
            </div>
            <div className="mt-6 flex justify-end gap-2">
              <button type="button" onClick={() => setConfirmEndOpen(false)} className="mcdl-button-secondary mcdl-hover-control px-4 py-2 text-[0.75rem]">{ts("继续换角")}</button>
              <button type="button" onClick={() => void confirmEnd()} className="mcdl-button-primary mcdl-hover-primary-action px-4 py-2 text-[0.75rem] font-semibold">{ts("结束并结算")}</button>
            </div>
          </div>
        </div>,
        document.body,
      ) : null}

      {mounted && restoreSession ? createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4">
          <div className="absolute inset-0 bg-black/55 backdrop-blur-[4px]" />
          <div role="dialog" aria-modal="true" aria-labelledby="character-studio-restore-title" className="mcdl-dialog-surface relative w-full max-w-md p-6 sm:p-7">
            <div className="mcdl-eyebrow">{ts("未关闭的会话")}</div>
            <h2 id="character-studio-restore-title" className="mcdl-display mt-2 text-[1.35rem]">{ts("发现一场仍在运行的会话")}</h2>
            <p className="mt-2 text-[0.75rem] leading-6 text-[color:var(--mcdl-text-body)]">
              {ts("上次离开页面时没有结束会话。你可以恢复监看，或直接结束并结算。")}
            </p>
            <div className="mt-4 grid grid-cols-2 gap-2">
              <Metric label={ts("已运行")} value={formatClock(restoreSession.elapsed_seconds)} />
              <Metric label={ts("预计")} value={formatCost((restoreSession.elapsed_seconds / 60) * (restoreSession.sale_price_per_minute || price))} />
            </div>
            <div className="mt-6 flex justify-end gap-2">
              <button type="button" onClick={() => void endRestoredSession()} disabled={restoreAction !== ""} className="mcdl-button-secondary mcdl-hover-control px-4 py-2 text-[0.75rem] disabled:opacity-50">{restoreAction === "end" ? ts("结算中…") : ts("结束并结算")}</button>
              <button type="button" onClick={() => void resumeSession()} disabled={restoreAction !== ""} className="mcdl-button-primary mcdl-hover-primary-action px-4 py-2 text-[0.75rem] font-semibold disabled:opacity-50">{restoreAction === "resume" ? ts("恢复中") : ts("恢复监看")}</button>
            </div>
          </div>
        </div>,
        document.body,
      ) : null}

      {mounted && dossierOpen ? createPortal(
        <div className="fixed inset-0 z-50 flex justify-end">
          <div className="absolute inset-0 bg-black/50 backdrop-blur-[3px]" onClick={() => setDossierOpen(false)} />
          <aside role="dialog" aria-modal="true" aria-labelledby="character-studio-dossier-title" className="mcdl-dialog-surface relative flex h-full w-full flex-col overflow-y-auto rounded-none p-6 sm:max-w-[30rem] sm:rounded-l-[1.75rem] sm:p-8">
            <div className="flex items-start justify-between gap-3">
              <div>
                <div className="mcdl-eyebrow">OPEN REALTIME PIPELINE</div>
                <h2 id="character-studio-dossier-title" className="mcdl-display mt-2 text-[1.5rem] leading-tight">{ts("这条管线怎么做到的")}</h2>
              </div>
              <button type="button" onClick={() => setDossierOpen(false)} aria-label={ts("关闭")} className="mcdl-icon-button mcdl-hover-quiet grid h-8 w-8 shrink-0 place-items-center rounded-full"><X size={14} /></button>
            </div>
            <p className="mt-3 text-[0.75rem] leading-6 text-[color:var(--mcdl-text-body)]">
              {ts("角色替换不是滤镜。它需要在每一帧里重新理解主体、保持身份，再把动作和镜头节奏原样留下来。")}
            </p>
            <div className="mt-5 space-y-2">
              {[
                { title: ts("采集"), body: ts("浏览器采集你的画面，经低延迟媒体通道上行。") },
                { title: ts("身份锚定"), body: ts("参考形象被编码为身份特征，约束每一帧的主体一致性。") },
                { title: ts("流式重绘"), body: ts("开放视频模型逐帧重绘，保留动作、表情与镜头节奏。") },
                { title: ts("时域稳定"), body: ts("自研的帧间一致性策略抑制闪烁与漂移。") },
                { title: ts("合成输出"), body: ts("编码为实时视频流回传，浏览器直接播放。") },
              ].map((item, index) => (
                <div key={item.title} className="flex gap-3 rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3.5 py-3">
                  <span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-[color:var(--mcdl-accent-soft)] font-mono text-[0.625rem] text-[color:var(--mcdl-color-accent)]">{index + 1}</span>
                  <div>
                    <div className="text-[0.75rem] font-semibold text-[color:var(--mcdl-text-title)]">{item.title}</div>
                    <p className="mt-0.5 text-[0.6875rem] leading-5 text-[color:var(--mcdl-text-muted)]">{item.body}</p>
                  </div>
                </div>
              ))}
            </div>
            <p className="mt-4 text-[0.625rem] leading-5 text-[color:var(--mcdl-text-muted)]">
              {ts("素材只用于你发起的会话；本站不保存原始视频流。请只使用本人或已获授权的形象。")}
            </p>
          </aside>
        </div>,
        document.body,
      ) : null}

      {mounted && editorOpen ? createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6">
          <div className="absolute inset-0 bg-black/55 backdrop-blur-[5px]" onClick={() => setEditorOpen(false)} />
          <form onSubmit={(event) => void saveCharacter(event)} role="dialog" aria-modal="true" aria-labelledby="character-studio-editor-title" className="mcdl-dialog-surface relative flex max-h-[calc(100dvh-2rem)] w-full max-w-2xl flex-col overflow-hidden rounded-[var(--mcdl-radius-card)]">
            <header className="shrink-0 px-6 pb-4 pt-6 sm:px-8 sm:pt-7">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <div className="mcdl-eyebrow">{ts("角色卡")}</div>
                  <h2 id="character-studio-editor-title" className="mcdl-display mt-2 text-[1.35rem]">{draft.publicId ? ts("编辑角色卡") : ts("新建角色卡")}</h2>
                  <p className="mt-1.5 text-[0.75rem] leading-6 text-[color:var(--mcdl-text-muted)]">{ts("参考图决定合成画面的身份。建议单人、正面、五官清晰、背景干净。")}</p>
                </div>
                <button type="button" onClick={() => setEditorOpen(false)} aria-label={ts("关闭")} className="mcdl-icon-button mcdl-hover-quiet grid h-8 w-8 shrink-0 place-items-center rounded-full"><X size={14} /></button>
              </div>
            </header>
            <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-5 sm:px-8">
              <div className="grid gap-5 sm:grid-cols-[10.5rem_minmax(0,1fr)]">
                <div>
                  <div className="mb-2 flex rounded-full bg-[color:var(--mcdl-color-surface-subtle)] p-0.5 text-[0.625rem]">
                    <button type="button" onClick={() => setImageMode("upload")} className={`flex-1 rounded-full px-2 py-1.5 font-medium transition ${imageMode === "upload" ? "bg-[color:var(--mcdl-color-surface-card)] text-[color:var(--mcdl-text-title)] shadow-sm" : "text-[color:var(--mcdl-text-muted)]"}`}>{ts("上传图片")}</button>
                    <button type="button" onClick={() => setImageMode("url")} className={`flex-1 rounded-full px-2 py-1.5 font-medium transition ${imageMode === "url" ? "bg-[color:var(--mcdl-color-surface-card)] text-[color:var(--mcdl-text-title)] shadow-sm" : "text-[color:var(--mcdl-text-muted)]"}`}>{ts("公网链接")}</button>
                  </div>
                  <div
                    role={imageMode === "upload" ? "button" : undefined}
                    tabIndex={imageMode === "upload" ? 0 : undefined}
                    onClick={() => { if (imageMode === "upload") imageFileRef.current?.click(); }}
                    onKeyDown={(event) => {
                      if (imageMode === "upload" && (event.key === "Enter" || event.key === " ")) {
                        event.preventDefault();
                        imageFileRef.current?.click();
                      }
                    }}
                    onDragOver={imageMode === "upload" ? (event) => { event.preventDefault(); setEditorDrag(true); } : undefined}
                    onDragLeave={imageMode === "upload" ? () => setEditorDrag(false) : undefined}
                    onDrop={imageMode === "upload" ? (event) => {
                      event.preventDefault();
                      setEditorDrag(false);
                      void uploadCharacterFile(event.dataTransfer.files?.[0]);
                    } : undefined}
                    className={`group relative block aspect-[3/4] w-full overflow-hidden rounded-[1.2rem] border ${imageMode === "upload" ? "cursor-pointer" : ""} ${editorDrag ? "border-[color:var(--mcdl-hover-border)] bg-[color:var(--mcdl-hover-surface)]" : "border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface-subtle)]"}`}
                  >
                    {imageMode === "url" && imagePreviewSrc ? (
                      <img
                        src={imagePreviewSrc}
                        alt=""
                        referrerPolicy="no-referrer"
                        className="h-full w-full object-cover"
                        onLoad={() => setImagePreviewError("")}
                        onError={() => {
                          setImagePreviewSrc("");
                          setImagePreviewError(ts("图片加载失败，请检查链接是否公网可访问"));
                        }}
                      />
                    ) : imageMode === "upload" && draft.imageURL ? (
                      <img src={draft.imageURL} alt="" className="h-full w-full object-cover" />
                    ) : (
                      <div className="flex h-full flex-col items-center justify-center gap-2 px-3 text-center text-[color:var(--mcdl-text-muted)]">
                        {editorUploading ? <Loader2 size={22} className="animate-spin" /> : imageMode === "url" ? <Info size={22} /> : <Upload size={22} />}
                        <span className="text-[0.6875rem]">{editorUploading ? ts("上传中…") : imageMode === "url" ? ts("输入公网图片地址后实时预览") : ts("点击或拖入参考图")}</span>
                        <span className="text-[0.5625rem] leading-4">{imageMode === "url" ? ts("支持 http(s) 图片地址") : ts("PNG / JPG / WebP，≤ 8MB")}</span>
                      </div>
                    )}
                    {imageMode === "upload" && draft.imageURL ? (
                      <span className="absolute inset-x-0 bottom-0 bg-black/45 px-2 py-1 text-center text-[0.5625rem] text-white/85 opacity-0 transition group-hover:opacity-100">{ts("更换参考图")}</span>
                    ) : null}
                    {imageMode === "url" && imagePreviewError ? (
                      <span className="absolute inset-x-0 bottom-0 bg-[color:var(--mcdl-color-negative)]/88 px-2 py-1.5 text-center text-[0.5625rem] leading-4 text-white">{ts("预览加载失败")}</span>
                    ) : null}
                  </div>
                  <input
                    ref={imageFileRef}
                    type="file"
                    accept="image/png,image/jpeg,image/webp"
                    className="hidden"
                    onChange={(event) => {
                      void uploadCharacterFile(event.target.files?.[0]);
                      event.currentTarget.value = "";
                    }}
                  />
                  {imageMode === "upload" ? (
                    <>
                      <button type="button" onClick={() => imageFileRef.current?.click()} className="mt-2 inline-flex w-full items-center justify-center gap-1.5 rounded-full border border-[color:var(--mcdl-border-ghost)] px-3 py-2 text-[0.625rem] text-[color:var(--mcdl-text-body)] transition hover:border-[color:var(--mcdl-hover-border)] hover:bg-[color:var(--mcdl-hover-surface)]">
                        <Upload size={12} />
                        {ts("选择本地图片")}
                      </button>
                      <p className="mt-2 text-[0.5625rem] leading-4 text-[color:var(--mcdl-text-muted)]">{ts("参考图会上传到你的素材库，仅用于角色替换。")}</p>
                    </>
                  ) : (
                    <p className="mt-2 text-[0.5625rem] leading-4 text-[color:var(--mcdl-text-muted)]">{ts("公网链接不会经过素材库，渲染服务会直接拉取。")}</p>
                  )}
                </div>
                <div className="space-y-4">
                  {imageMode === "url" ? (
                    <label className="block">
                      <span className="text-[0.75rem] font-medium text-[color:var(--mcdl-text-body)]">{ts("公网图片地址")}</span>
                      <input
                        type="url"
                        value={publicImageURL}
                        onChange={(event) => setPublicImageURL(event.target.value)}
                        placeholder="https://img.example.com/character.png"
                        className="mt-1.5 w-full rounded-xl border border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-field-surface)] px-3.5 py-2.5 text-[0.8125rem] outline-none transition focus:border-[color:var(--mcdl-accent-border)] focus:ring-2 focus:ring-[color:var(--mcdl-accent-soft)]"
                      />
                      <p className={`mt-1 text-[0.5625rem] leading-4 ${imagePreviewError ? "text-[color:var(--mcdl-color-negative)]" : "text-[color:var(--mcdl-text-muted)]"}`}>
                        {imagePreviewError || ts("必须是公网可访问的绝对地址，渲染服务会直接拉取这张图。")}
                      </p>
                    </label>
                  ) : null}
                  <label className="block">
                    <span className="text-[0.75rem] font-medium text-[color:var(--mcdl-text-body)]">{ts("角色名字")}</span>
                    <input
                      autoFocus
                      value={draft.name}
                      onChange={(event) => setDraft((previous) => ({ ...previous, name: event.target.value }))}
                      maxLength={24}
                      placeholder={ts("例如：午夜侦探、赛博搭档")}
                      className="mt-1.5 w-full rounded-xl border border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-field-surface)] px-3.5 py-2.5 text-[0.8125rem] outline-none transition focus:border-[color:var(--mcdl-accent-border)] focus:ring-2 focus:ring-[color:var(--mcdl-accent-soft)]"
                    />
                  </label>
                  <div className="rounded-2xl bg-[color:var(--mcdl-color-surface-subtle)] px-3.5 py-3">
                    <div className="flex items-center gap-1.5 text-[0.6875rem] font-semibold text-[color:var(--mcdl-text-body)]"><ShieldCheck size={13} className="text-[color:var(--mcdl-color-positive)]" />{ts("使用授权")}</div>
                    <p className="mt-1 text-[0.625rem] leading-5 text-[color:var(--mcdl-text-muted)]">{ts("请仅使用本人或已获得明确授权的形象。禁止用于冒充他人、欺诈或误导性内容。")}</p>
                    {!draft.publicId ? (
                      <label className="mt-2 flex cursor-pointer items-start gap-2 text-[0.625rem] leading-5 text-[color:var(--mcdl-text-body)]">
                        <input type="checkbox" checked={draft.rights} onChange={(event) => setDraft((previous) => ({ ...previous, rights: event.target.checked }))} className="mt-0.5 h-3.5 w-3.5 accent-[color:var(--mcdl-color-accent)]" />
                        {ts("我确认拥有该形象的使用授权")}
                      </label>
                    ) : (
                      <div className="mt-2 inline-flex items-center gap-1 text-[0.625rem] text-[color:var(--mcdl-color-positive)]"><Check size={12} />{ts("已确认授权")}</div>
                    )}
                  </div>
                  {editorError ? <p className="rounded-xl bg-[color:var(--mcdl-color-negative)]/10 px-3.5 py-2.5 text-[0.6875rem] leading-5 text-[color:var(--mcdl-color-negative)]">{ts(editorError)}</p> : null}
                </div>
              </div>
            </div>
            <footer className="flex shrink-0 items-center justify-end gap-2 bg-[color:var(--mcdl-color-surface-subtle)] px-6 py-4 sm:px-8">
              {draft.publicId ? (
                <button type="button" onClick={() => void removeCharacter()} disabled={editorSaving} className="mr-auto px-2 py-2 text-[0.6875rem] text-[color:var(--mcdl-color-negative)] disabled:opacity-50">{ts("删除角色卡")}</button>
              ) : null}
              <button type="button" onClick={() => setEditorOpen(false)} className="mcdl-button-secondary mcdl-hover-control px-4 py-2 text-[0.75rem]">{ts("取消")}</button>
              <button type="submit" disabled={editorSaving || editorUploading} className="mcdl-button-primary mcdl-hover-primary-action inline-flex items-center gap-2 px-4 py-2 text-[0.75rem] font-semibold disabled:opacity-50">
                {editorSaving ? <Loader2 size={13} className="animate-spin" /> : null}
                {draft.publicId ? ts("保存修改") : ts("保存角色卡")}
              </button>
            </footer>
          </form>
        </div>,
        document.body,
      ) : null}

      {notice ? (
        <div role="status" aria-live="polite" className="fixed bottom-5 left-1/2 z-[60] w-[calc(100%-2rem)] max-w-md -translate-x-1/2 rounded-2xl border border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface-card)] px-4 py-3 text-center text-[0.75rem] leading-5 text-[color:var(--mcdl-text-body)] shadow-[var(--mcdl-shadow-strong)]">
          {notice}
        </div>
      ) : null}
    </div>
  );
}
