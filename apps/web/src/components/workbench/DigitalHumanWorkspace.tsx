"use client";

import { FormEvent, ReactNode, useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useI18n } from "@/i18n/I18nProvider";
import { api, uploadFile } from "@/lib/api";
import { DIGITAL_HUMAN_DEFAULT_ROLES, type DigitalHumanDefaultRole } from "@/lib/digitalHumanDefaults";

const RELATIONS = ["父亲", "母亲", "伴侣", "朋友", "兄弟", "姐妹", "祖父", "祖母", "助理", "其他"];
const RTC_SDK = "https://g.alicdn.com/apsara-media-box/imp-web-rtc/7.1.9/aliyun-rtc-sdk.js";

// 音色取 Vidu 官方「音色列表」中 provider=qwen_omni 的实时版音色。
const VOICE_GROUPS: { label: string; items: { id: string; name: string; desc: string }[] }[] = [
  {
    label: "女声",
    items: [
      { id: "Tina", name: "甜甜 Tina", desc: "奶茶小姐姐，甜甜的、暖暖的" },
      { id: "Cindy", name: "林欣宜 Cindy", desc: "台湾说话嗲嗲的小姐姐" },
      { id: "Liora Mira", name: "清欢 Liora Mira", desc: "温柔治愈" },
      { id: "Serena", name: "苏瑶 Serena", desc: "温柔小姐姐" },
      { id: "Mia", name: "舒然 Mia", desc: "慢生活美学，日常治愈" },
      { id: "Maia", name: "四月 Maia", desc: "知性与温柔" },
      { id: "Sunnybobi", name: "知芝 Sunnybobi", desc: "大大咧咧的社恐邻家姑娘" },
      { id: "Momo", name: "茉兔 Momo", desc: "撒娇搞怪，逗你开心" },
      { id: "Katerina", name: "卡捷琳娜 Katerina", desc: "御姐音色，韵律回味十足" },
      { id: "Jennifer", name: "詹妮弗 Jennifer", desc: "电影质感美语女声" },
    ],
  },
  {
    label: "男声",
    items: [
      { id: "Raymond", name: "林川野 Raymond", desc: "声音清亮，宅男感" },
      { id: "Ethan", name: "晨煦 Ethan", desc: "阳光、温暖、有朝气" },
      { id: "Theo Calm", name: "予安 Theo Calm", desc: "在静默处传递理解" },
      { id: "Harvey", name: "厚 Harvey", desc: "低沉温和，岁月沉淀" },
      { id: "Evan", name: "江晨 Evan", desc: "男大学生，年下奶狗" },
      { id: "Ryan", name: "甜茶 Ryan", desc: "节奏拉满，戏感炸裂" },
      { id: "Joyner", name: "阿逗 Joyner", desc: "搞笑夸张，接地气" },
    ],
  },
  {
    label: "方言 / 特色",
    items: [
      { id: "Sunny", name: "晴儿 Sunny", desc: "甜到心里的川妹子" },
      { id: "Dylan", name: "晓东 Dylan", desc: "北京胡同里长大的少年" },
      { id: "Eric", name: "程川 Eric", desc: "四川成都男子" },
      { id: "Peter", name: "李彼得 Peter", desc: "天津相声，专业捧哏" },
      { id: "Rocky", name: "阿强 Rocky", desc: "广东话幽默陪聊" },
      { id: "Kiki", name: "阿清 Kiki", desc: "甜美的港妹闺蜜" },
      { id: "Gold", name: "金爷 Gold", desc: "西海岸 Rapper" },
    ],
  },
];

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
      found.addEventListener("error", () => reject(new Error("音视频组件加载失败")), { once: true });
      return;
    }
    const script = document.createElement("script");
    script.src = RTC_SDK;
    script.async = true;
    script.dataset.aliRtc = "1";
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("音视频组件加载失败"));
    document.body.appendChild(script);
  });
  if (!host.AliRtcEngine) throw new Error("音视频组件没有就绪");
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

type RoleFieldsProps = {
  draft: Draft;
  patch: (key: keyof Draft, value: string) => void;
  uploadingAvatar: boolean;
  onUploadAvatar: (file: File | undefined) => void;
};

// 三个长文本字段各给几个现成示例：点一下填入，再点一下取消，也可以直接手写。
const FIELD_PRESETS: Record<"persona" | "memory" | "style", { label: string; value: string }[]> = {
  persona: [
    { label: "温柔倾听", value: "先接住情绪，再慢慢给建议；不急着下结论，也不说教。" },
    { label: "元气搭子", value: "说话有精神，爱用轻松的方式逗人开心；聊到正事会立刻认真起来。" },
    { label: "专业顾问", value: "先给结论，再讲理由，最后给一两个能马上做的动作；不确定就直说不确定。" },
    { label: "邻家损友", value: "会开玩笑、会拆台，但对方真难受时立刻收起玩笑，站在对方这边。" },
  ],
  memory: [
    { label: "备考压力", value: "对方最近在准备考试，压力大，容易熬夜。" },
    { label: "养了只猫", value: "对方养了一只猫，叫年糕，最喜欢聊猫。" },
    { label: "小店生意", value: "对方在开一家小店，最近在纠结要不要扩店。" },
    { label: "爱跑步", value: "对方喜欢跑步，每周三次，最近在准备半马。" },
  ],
  style: [
    { label: "短句慢语", value: "短句，语速偏慢，常用「嗯」「其实吧」开头。" },
    { label: "轻松幽默", value: "轻松幽默，爱打比方，偶尔自嘲。" },
    { label: "简洁理性", value: "简洁理性，先说结论，少用形容词。" },
    { label: "温柔细碎", value: "温柔细碎，喜欢重复对方的话来确认感受。" },
  ],
};

const CHIP_BASE = "rounded-full border px-3 py-1 text-[0.6875rem] font-medium transition duration-200";
const CHIP_ON = "border-[color:var(--mcdl-accent-border)] bg-[color:var(--mcdl-accent-soft)] text-[color:var(--mcdl-color-accent)]";
const CHIP_OFF = "border-[color:var(--mcdl-border-ghost)] text-[color:var(--mcdl-text-body)] hover:border-[color:var(--mcdl-hover-border)] hover:bg-[color:var(--mcdl-hover-surface)]";

const FIELD_CLASS =
  "w-full rounded-xl border border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-field-surface)] px-3.5 py-2.5 text-sm text-[color:var(--mcdl-text-title)] outline-none transition placeholder:text-[color:var(--mcdl-text-muted)] focus:border-[color:var(--mcdl-accent-border)] focus:ring-2 focus:ring-[color:var(--mcdl-accent-soft)]";

function Field({ label, badge, hint, children }: { label: string; badge?: ReactNode; hint?: string; children: ReactNode }) {
  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-[0.75rem] font-medium text-[color:var(--mcdl-text-body)]">{label}</span>
        {badge}
      </div>
      {hint ? <p className="mcdl-muted-copy mt-1.5 text-[0.6875rem] leading-5">{hint}</p> : null}
      <div className="mt-2.5">{children}</div>
    </div>
  );
}

function PresetChips({
  field,
  value,
  onPick,
}: {
  field: "persona" | "memory" | "style";
  value: string;
  onPick: (text: string) => void;
}) {
  const { ts } = useI18n();
  return (
    <div className="flex flex-wrap gap-2">
      {FIELD_PRESETS[field].map((preset) => {
        const active = value.trim() === preset.value;
        return (
          <button
            key={preset.label}
            type="button"
            aria-pressed={active}
            onClick={() => onPick(active ? "" : preset.value)}
            className={`${CHIP_BASE} ${active ? CHIP_ON : CHIP_OFF}`}
          >
            {ts(preset.label)}
          </button>
        );
      })}
    </div>
  );
}

// 新建弹窗与右侧编辑面板共用同一套字段，避免两处结构漂移。
function RoleFields({ draft, patch, uploadingAvatar, onUploadAvatar }: RoleFieldsProps) {
  const { ts } = useI18n();
  const required = <span className="rounded-full bg-[color:var(--mcdl-accent-soft)] px-2 py-0.5 text-[0.625rem] font-semibold text-[color:var(--mcdl-color-accent)]">{ts("必填")}</span>;
  const optional = <span className="text-[0.625rem] text-[color:var(--mcdl-text-muted)]">{ts("选填")}</span>;

  const nameField = (
    <Field key="name" label={ts("名字")} badge={required}>
      <input value={draft.name} onChange={(event) => patch("name", event.target.value)} placeholder={ts("例如：丽丽")} className={FIELD_CLASS} />
    </Field>
  );

  const relationField = (
    <Field key="relation" label={ts("关系")} badge={optional} hint={ts("决定数字人怎么理解你们之间的距离。")}>
      <div className="flex flex-wrap gap-2">
        {RELATIONS.map((item) => (
          <button key={item} type="button" onClick={() => patch("relation", item)} className={`${CHIP_BASE} ${draft.relation === item ? CHIP_ON : CHIP_OFF}`}>{ts(item)}</button>
        ))}
      </div>
    </Field>
  );

  const userTitleField = (
    <Field key="user_title" label={ts("用户称呼")} badge={optional}>
      <input value={draft.user_title} onChange={(event) => patch("user_title", event.target.value)} placeholder={ts("例如：小陈、宝贝、老板")} className={FIELD_CLASS} />
    </Field>
  );

  const knowledgeTitleField = (
    <Field key="knowledge_title" label={ts("知识库标题")} badge={optional} hint={ts("给这个角色的资料起个名字，方便以后查找。")}>
      <input value={draft.knowledge_title} onChange={(event) => patch("knowledge_title", event.target.value)} placeholder={ts("例如：花店价目与配送规则")} className={FIELD_CLASS} />
    </Field>
  );

  const personaField = (
    <Field key="persona" label={ts("人设撰写")} badge={optional} hint={ts("点下面的标签直接套用，也可以自己改写。")}>
      <PresetChips field="persona" value={draft.persona} onPick={(text) => patch("persona", text)} />
      <textarea
        aria-label={ts("人设撰写")}
        value={draft.persona}
        onChange={(event) => patch("persona", event.target.value)}
        rows={4}
        placeholder={ts("例如：你是丽丽，29 岁，在杭州开一家小花店。说话像老朋友，先接住情绪再给建议，偶尔拿自己开玩笑，但不说教。")}
        className={`${FIELD_CLASS} mt-3 resize-y leading-6`}
      />
    </Field>
  );

  const memoryField = (
    <Field key="memory" label={ts("核心记忆")} badge={optional} hint={ts("数字人会一直记住这些事，用来做承接和追问。")}>
      <PresetChips field="memory" value={draft.memory} onPick={(text) => patch("memory", text)} />
      <textarea
        aria-label={ts("核心记忆")}
        value={draft.memory}
        onChange={(event) => patch("memory", event.target.value)}
        rows={3}
        placeholder={ts("例如：用户叫小陈，做产品经理，养了一只叫豆豆的柯基。最近在准备半马，睡眠不太好，不喜欢被催进度。")}
        className={`${FIELD_CLASS} mt-3 resize-y leading-6`}
      />
    </Field>
  );

  const styleField = (
    <Field key="style" label={ts("说话风格")} badge={optional} hint={ts("说得快还是慢、句子长还是短，都会影响真实感。")}>
      <PresetChips field="style" value={draft.style} onPick={(text) => patch("style", text)} />
      <textarea
        aria-label={ts("说话风格")}
        value={draft.style}
        onChange={(event) => patch("style", event.target.value)}
        rows={2}
        placeholder={ts("例如：语速偏慢，句子短，常用「嗯」「其实吧」开头；不堆形容词，一次只说一件事。")}
        className={`${FIELD_CLASS} mt-3 resize-y leading-6`}
      />
    </Field>
  );

  const voiceField = (
    <Field key="voice" label={ts("音色")} badge={optional} hint={ts("克隆音色后续开放")}>
      <select value={draft.voice} onChange={(event) => patch("voice", event.target.value)} className={FIELD_CLASS}>
        {!VOICE_GROUPS.some((group) => group.items.some((item) => item.id === draft.voice)) && draft.voice ? (
          <option value={draft.voice}>{draft.voice}</option>
        ) : null}
        {VOICE_GROUPS.map((group) => (
          <optgroup key={group.label} label={ts(group.label)}>
            {group.items.map((item) => (
              <option key={item.id} value={item.id}>{item.name} · {ts(item.desc)}</option>
            ))}
          </optgroup>
        ))}
      </select>
    </Field>
  );

  const avatarField = (
    <Field key="avatar_url" label={ts("头像")} badge={optional} hint={ts("开始视频通话前需要先填好头像。")}>
      <div className="flex items-start gap-4">
        <div className="h-[4.5rem] w-[4.5rem] shrink-0 overflow-hidden rounded-2xl border border-[color:var(--mcdl-border-ghost)] bg-[color:var(--mcdl-color-surface-subtle)]">
          {draft.avatar_url ? (
            <img src={draft.avatar_url} alt="" className="h-full w-full object-cover" />
          ) : (
            <div className="flex h-full items-center justify-center text-[0.625rem] text-[color:var(--mcdl-text-muted)]">{ts("预览")}</div>
          )}
        </div>
        <div className="min-w-0 flex-1">
          <input
            value={draft.avatar_url}
            onChange={(event) => patch("avatar_url", event.target.value)}
            placeholder={ts("https:// 公网图片地址")}
            className={FIELD_CLASS}
          />
          <label className="mcdl-button-secondary mcdl-hover-control mt-2.5 inline-flex cursor-pointer px-4 py-2 text-[0.75rem]">
            {uploadingAvatar ? ts("上传中…") : ts("上传图片")}
            <input
              type="file"
              accept="image/png,image/jpeg,image/webp"
              className="hidden"
              disabled={uploadingAvatar}
              onChange={(event) => {
                onUploadAvatar(event.target.files?.[0]);
                event.target.value = "";
              }}
            />
          </label>
        </div>
      </div>
    </Field>
  );

  // 每行可以并排放 1~2 个字段：短字段并排省高度，长字段独占一行。
  const groups: { title: string; description: string; rows: ReactNode[][] }[] = [
    {
      title: ts("角色信息"),
      description: ts("先定下 TA 是谁、和你是什么关系，其余细节随时能再调。"),
      rows: [[nameField, userTitleField], [relationField], [knowledgeTitleField]],
    },
    {
      title: ts("语气与人设"),
      description: ts("这三项决定 TA 说话像不像一个人。不会写就直接点标签套一个。"),
      rows: [[personaField], [memoryField], [styleField]],
    },
    {
      title: ts("形象与声音"),
      description: ts("头像和音色是视频通话里最直观的部分。"),
      rows: [[voiceField], [avatarField]],
    },
  ];

  return (
    <div className="space-y-5">
      {groups.map((group) => (
        <section key={group.title} className="rounded-[1.5rem] bg-[color:var(--mcdl-color-surface-subtle)] p-6 sm:p-7">
          <h3 className="text-[0.9375rem] font-semibold tracking-normal text-[color:var(--mcdl-text-title)]">{group.title}</h3>
          <p className="mcdl-muted-copy mt-2 text-[0.75rem] leading-6">{group.description}</p>
          <div className="mt-6 space-y-6">
            {group.rows.map((row, rowIndex) => (
              <div key={rowIndex} className={row.length > 1 ? "grid gap-6 sm:grid-cols-2" : undefined}>
                {row.map((field, index) => <div key={index}>{field}</div>)}
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  );
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
  const [roleTab, setRoleTab] = useState<"default" | "mine">("default");
  const [defaultRoleBusy, setDefaultRoleBusy] = useState("");
  const [defaultRoleError, setDefaultRoleError] = useState("");
  const [text, setText] = useState("");
  const [docTitle, setDocTitle] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState("");
  const [prices, setPrices] = useState({ voice: 0, video: 0 });
  const [videoStage, setVideoStage] = useState<"idle" | "connecting" | "live">("idle");
  const [micOn, setMicOn] = useState(true);
  const [cameraOn, setCameraOn] = useState(true);
  const [elapsedSec, setElapsedSec] = useState(0);
  const [remoteReady, setRemoteReady] = useState(false);
  const [editorOpen, setEditorOpen] = useState(false);
  const [editorMode, setEditorMode] = useState<"create" | "edit">("create");
  const [editorDraft, setEditorDraft] = useState<Draft>(emptyDraft);
  const [editorError, setEditorError] = useState("");
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [mounted, setMounted] = useState(false);
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
    setMounted(true);
  }, []);

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
    if (!roleId) return;
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
  }, [roleId]);

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

  useEffect(() => {
    if (!editorOpen) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setEditorOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [editorOpen]);

  const visibleRoles = roles.filter((role) => {
    const q = query.trim();
    return !q || role.name.includes(q) || role.relation.includes(q);
  });

  const visibleDefaults = DIGITAL_HUMAN_DEFAULT_ROLES.filter((preset) => {
    const q = query.trim();
    return !q || preset.name.includes(q) || preset.type.includes(q) || preset.relation.includes(q);
  });

  const patchEditor = (key: keyof Draft, value: string) => setEditorDraft((prev) => ({ ...prev, [key]: value }));

  const openCreate = () => {
    setRoleTab("mine");
    setEditorMode("create");
    setEditorDraft(emptyDraft());
    setEditorError("");
    setEditorOpen(true);
  };

  const openRoleEditor = () => {
    if (!roleId) return;
    setEditorMode("edit");
    setEditorDraft({ ...draft });
    setEditorError("");
    setEditorOpen(true);
  };

  const uploadAvatar = async (file: File | undefined) => {
    if (!file) return;
    if (!file.type.startsWith("image/")) {
      setEditorError(ts("请选择图片文件"));
      return;
    }
    setUploadingAvatar(true);
    try {
      const url = await uploadFile(file);
      setEditorDraft((prev) => ({ ...prev, avatar_url: url }));
    } catch (err) {
      setEditorError(err instanceof Error ? ts(err.message) : ts("上传失败"));
    } finally {
      setUploadingAvatar(false);
    }
  };

  const refreshRoles = async () => {
    setRoles(await api<RoleSummary[]>("/api/digital-human/roles"));
  };

  const applyDefaultRole = async (preset: DigitalHumanDefaultRole) => {
    if (defaultRoleBusy || busy) return;
    setDefaultRoleError("");
    const existing = roles.find((role) => role.name === preset.name || role.avatar_url === preset.avatarUrl);
    if (existing) {
      setRoleTab("mine");
      setRoleId(existing.public_id);
      return;
    }
    setDefaultRoleBusy(preset.key);
    try {
      const created = await api<RoleDetail>("/api/digital-human/roles", {
        method: "POST",
        body: JSON.stringify({
          name: preset.name,
          relation: preset.relation,
          user_title: preset.userTitle,
          memory: preset.memory,
          style: preset.style,
          persona: preset.persona,
          voice: preset.voice,
          avatar_url: preset.avatarUrl,
          knowledge_title: "",
        }),
      });
      setRoles((prev) => [{
        public_id: created.public_id,
        name: created.name,
        relation: created.relation,
        user_title: created.user_title,
        avatar_url: created.avatar_url,
        knowledge_title: created.knowledge_title,
      }, ...prev]);
      setRoleTab("mine");
      setRoleId(created.public_id);
      setNotice(ts("已加入「我的角色」并选中"));
    } catch (err) {
      setDefaultRoleError(err instanceof Error ? ts(err.message) : ts("使用默认角色失败"));
    } finally {
      setDefaultRoleBusy("");
    }
  };

  const saveEditor = async () => {
    if (!editorDraft.name.trim()) {
      setEditorError(ts("请填写角色名字"));
      return;
    }
    const creating = editorMode === "create";
    setBusy("保存中");
    setEditorError("");
    try {
      const saved = await api<RoleDetail>(creating ? "/api/digital-human/roles" : `/api/digital-human/roles/${roleId}`, {
        method: creating ? "POST" : "PUT",
        body: JSON.stringify(editorDraft),
      });
      await refreshRoles();
      setEditorOpen(false);
      setRoleTab("mine");
      if (creating) setRoleId(saved.public_id);
      else setDraft(draftFromRole(saved));
      setNotice(ts("角色已保存"));
    } catch (err) {
      setEditorError(err instanceof Error ? ts(err.message) : ts("保存失败"));
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
      setEditorOpen(false);
      setRoleId(items[0]?.public_id || "");
      if (!items[0]) {
        setDraft(emptyDraft());
        setMessages([]);
        setKnowledge([]);
      }
    } catch (err) {
      setEditorError(err instanceof Error ? ts(err.message) : ts("删除失败"));
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

  const joinMedia = async (rtc: RtcInfo | undefined) => {
    if (!rtc?.token || !rtc.user_id) throw new Error("没有拿到通话房间凭证");
    const sdk = await loadRtcEngine();
    const engine = sdk.getInstance();
    engineRef.current = engine;
    await engine.setChannelProfile?.("communication");
    await engine.setDefaultSubscribeAllRemoteAudioStreams?.(true);
    await engine.setDefaultSubscribeAllRemoteVideoStreams?.(true);
    await engine.subscribeAllRemoteVideoStreams?.(true);
    const onVideo = (userId: unknown, _oldState: unknown, newState: unknown) => {
      if (userId == null) return;
      if (rtcSubscribed(newState) || newState === 2 || newState === "2") bindRemoteVideo(engine, String(userId), 1);
    };
    const onScreen = (userId: unknown, _oldState: unknown, newState: unknown) => {
      if (userId == null) return;
      if (rtcSubscribed(newState) || newState === 2 || newState === "2") bindRemoteVideo(engine, String(userId), 2);
    };
    engine.on?.("videoSubscribeStateChanged", onVideo);
    engine.on?.("screenShareSubscribeStateChanged", onScreen);
    engine.on?.("remoteUserOnLineNotify", (userId) => {
      void subscribeRemote(engine, String(userId || ""));
    });
    if (localVideoRef.current) {
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
    await engine.publishLocalVideoStream?.(true);
    window.setTimeout(() => {
      const listed = (engine as RtcEngine & { getOnlineRemoteUsers?: () => string[] }).getOnlineRemoteUsers?.() || [];
      listed.forEach((userId) => void subscribeRemote(engine, String(userId)));
    }, 1500);
    setMicOn(true);
    setCameraOn(true);
  };

  const startSession = async (mode: "text" | "video") => {
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
    joinMedia(rtc)
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
      if (!result.assistant) setNotice(ts("文字已送出，对方还没有回复。若无响应，请结束会话后重试。"));
    } catch (err) {
      setText(content);
      setNotice(err instanceof Error ? ts(err.message) : ts("发送失败"));
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

  const status = videoStage !== "idle" ? "视频通话中" : session ? "控制链路已就绪" : "已结束";
  const elapsedLabel = `${String(Math.floor(elapsedSec / 60)).padStart(2, "0")}:${String(elapsedSec % 60).padStart(2, "0")}`;
  const minutePrice = prices.video > 0 ? prices.video : prices.voice;
  const estimatedCost = Math.round((elapsedSec / 60) * minutePrice * 100) / 100;

  return (
    <div className="mcdl-workspace mcdl-workspace--digital-human flex h-full min-h-0 flex-col text-gray-900 dark:text-gray-100">
      {/* 视口右上角浮动着通知/语言/快捷菜单（AppShell），桌面端为其预留右侧空间，避免“角色设置”被覆盖。 */}
      <header className="flex h-[73px] shrink-0 items-center gap-3 border-b border-gray-100 bg-white px-4 dark:border-white/10 dark:bg-gray-900 lg:pr-48">
        <div className="flex min-w-0 items-center gap-2">
          <h1 className="text-base font-semibold">{ts("实时数字人对话")}</h1>
          <span className={`text-xs ${status === "已结束" ? "text-red-500" : "text-gray-500"}`}>{ts(status)}</span>
          {roleId && <span className="truncate text-xs text-gray-400">· {draft.name || ts("未命名")}</span>}
        </div>
        {roleId && (
          <button type="button" onClick={openRoleEditor} className="mcdl-button-secondary mcdl-hover-control ml-auto shrink-0 px-4 py-1.5 text-xs">
            {ts("角色设置")}
          </button>
        )}
      </header>
      <div className="grid min-h-0 flex-1 grid-cols-1 lg:grid-cols-[240px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col border-b border-gray-200 bg-white dark:border-white/10 dark:bg-gray-900 lg:border-b-0 lg:border-r">
          <div className="flex items-center justify-between px-3 py-3">
            <span className="text-sm font-medium">{ts("角色列表")}</span>
            <button
              type="button"
              className="rounded-full bg-primary px-3 py-1.5 text-xs font-semibold text-dark"
              onClick={openCreate}
            >
              {ts("新建角色")}
            </button>
          </div>
          <div className="mx-3 mb-2 grid grid-cols-2 rounded-lg bg-gray-100 p-0.5 text-xs dark:bg-white/5">
            <button
              type="button"
              onClick={() => setRoleTab("default")}
              className={`rounded-md px-2 py-1.5 font-medium transition ${roleTab === "default" ? "bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white" : "text-gray-500"}`}
            >
              {ts("默认角色")}
            </button>
            <button
              type="button"
              onClick={() => setRoleTab("mine")}
              className={`rounded-md px-2 py-1.5 font-medium transition ${roleTab === "mine" ? "bg-white text-gray-900 shadow-sm dark:bg-gray-800 dark:text-white" : "text-gray-500"}`}
            >
              {ts("我的角色")}{roles.length ? ` ${roles.length}` : ""}
            </button>
          </div>
          <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={roleTab === "default" ? ts("搜索默认角色") : ts("搜索角色")} className="mx-3 mb-2 rounded-md border border-gray-200 px-2 py-1.5 text-sm dark:border-white/10 dark:bg-gray-950" />
          {roleTab === "default" && (
            <p className="mx-3 mb-2 text-[10px] leading-4 text-gray-400">
              {ts("默认角色为只读模板，点击加入「我的角色」后可编辑。")}
            </p>
          )}
          <div className="min-h-0 flex-1 overflow-y-auto px-2 pb-3">
            {roleTab === "default" ? (
              <>
                <div className="grid grid-cols-2 gap-2 px-1">
                  {visibleDefaults.map((preset) => {
                    const added = roles.some((role) => role.name === preset.name || role.avatar_url === preset.avatarUrl);
                    const loading = defaultRoleBusy === preset.key;
                    return (
                      <button
                        key={preset.key}
                        type="button"
                        disabled={!!defaultRoleBusy || !!busy}
                        onClick={() => void applyDefaultRole(preset)}
                        className="group relative overflow-hidden rounded-xl border border-gray-200 bg-white text-left transition hover:border-primary/60 hover:shadow-sm disabled:opacity-60 dark:border-white/10 dark:bg-gray-950"
                      >
                        <span className="relative block aspect-[3/4] w-full overflow-hidden bg-gray-100 dark:bg-white/5">
                          <img src={preset.avatarUrl} alt={preset.name} loading="lazy" className="h-full w-full object-cover transition duration-300 group-hover:scale-[1.03]" />
                          {added && <span className="absolute left-1.5 top-1.5 rounded-full bg-black/65 px-1.5 py-0.5 text-[9px] font-medium text-white">{ts("已添加")}</span>}
                          {loading && <span className="absolute inset-0 flex items-center justify-center bg-black/45 text-[10px] font-medium text-white">{ts("添加中")}</span>}
                        </span>
                        <span className="block px-2 py-1.5">
                          <span className="block truncate text-xs font-semibold">{preset.name}</span>
                          <span className="mt-0.5 block truncate text-[10px] text-gray-400">{ts(preset.type)}</span>
                          <span className="mt-0.5 block truncate text-[9px] text-gray-400">{preset.voice}</span>
                        </span>
                      </button>
                    );
                  })}
                </div>
                {visibleDefaults.length === 0 && <div className="px-2 py-6 text-center text-xs text-gray-400">{ts("没有匹配的默认角色")}</div>}
                {defaultRoleError && <div className="mx-2 mt-2 text-[11px] leading-4 text-red-500">{defaultRoleError}</div>}
              </>
            ) : (
              <>
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
                      setRoleId(role.public_id);
                    }}
                    className={`mb-1 flex w-full items-center gap-2 rounded-lg px-2 py-2 text-left text-sm ${role.public_id === roleId ? "bg-primary/10 text-primary dark:bg-primary/10" : "hover:bg-gray-50 dark:hover:bg-white/5"}`}
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
              </>
            )}
          </div>
        </aside>

        <section className="relative flex min-h-0 flex-col">
          <div className="border-b border-gray-200 px-4 py-2 text-[11px] text-gray-500 dark:border-white/10">
            {ts("文字按模型计费，视频")} {minutePrice > 0 ? minutePrice.toFixed(2) : "—"} {ts("算力/分钟")}
          </div>
          <div ref={listRef} className="min-h-0 flex-1 space-y-3 overflow-y-auto px-4 py-4">
            {messages.map((message, index) => (
              <div key={`${message.id || index}-${message.created_at || index}`} className={`flex ${message.speaker === "user" ? "justify-end" : "justify-start"}`}>
                <div className={`max-w-[80%] rounded-2xl px-3 py-2 text-sm leading-6 ${message.speaker === "user" ? "bg-primary text-white" : "bg-white text-gray-800 dark:bg-gray-900 dark:text-gray-100"}`}>
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
              <button type="button" disabled={!!busy || videoStage !== "idle"} onClick={() => void startVideo()} className="rounded-lg border border-gray-200 px-3 py-1.5 text-xs disabled:opacity-50 dark:border-white/10">
                {videoStage === "connecting" ? ts("接通中") : ts("视频通话")}
              </button>
              <button type="submit" disabled={!!busy || !text.trim()} className="rounded-lg bg-primary px-3 py-1.5 text-xs text-white disabled:opacity-50">{ts("发送")}</button>
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

      </div>

      {mounted && editorOpen && createPortal(
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6">
          <div className="absolute inset-0 bg-black/55 backdrop-blur-[6px]" onClick={() => setEditorOpen(false)} />
          <div
            role="dialog"
            aria-modal="true"
            aria-labelledby="digital-human-editor-title"
            className="mcdl-dialog-surface relative flex max-h-[calc(100dvh-2rem)] w-full max-w-[46rem] flex-col overflow-hidden rounded-[var(--mcdl-radius-card)]"
          >
            <header className="shrink-0 px-6 pb-5 pt-7 sm:px-9 sm:pt-8">
              <div className="flex items-start justify-between gap-4">
                <div className="min-w-0">
                  <span className="mcdl-eyebrow">{ts("数字人")}</span>
                  <h2 id="digital-human-editor-title" className="mcdl-display mt-3 text-[1.625rem] leading-tight sm:text-[1.875rem]">
                    {editorMode === "create" ? ts("新建角色") : ts("角色设置")}
                  </h2>
                  <p className="mcdl-muted-copy mt-2 text-[0.8125rem] leading-6">
                    {editorMode === "create"
                      ? ts("只有名字必填，其余都可以留空，保存后随时能补。")
                      : ts("改完保存即刻生效，正在进行的通话仍沿用接通时的设定。")}
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setEditorOpen(false)}
                  aria-label={ts("关闭")}
                  className="mcdl-icon-button mcdl-hover-quiet mcdl-hover-press grid h-9 w-9 shrink-0 place-items-center rounded-full text-sm"
                >
                  ✕
                </button>
              </div>
            </header>
            <div className="min-h-0 flex-1 overflow-y-auto px-6 pb-8 sm:px-9">
              <RoleFields draft={editorDraft} patch={patchEditor} uploadingAvatar={uploadingAvatar} onUploadAvatar={(file) => void uploadAvatar(file)} />
              {editorMode === "edit" && (
                <section className="mt-5 rounded-[1.5rem] bg-[color:var(--mcdl-color-surface-subtle)] p-6 sm:p-7">
                  <h3 className="text-[0.9375rem] font-semibold tracking-normal text-[color:var(--mcdl-text-title)]">{ts("知识库")}</h3>
                  <p className="mcdl-muted-copy mt-2 text-[0.75rem] leading-6">{ts("上传 TXT、MD 或 JSON。通话中数字人只会检索这个角色的资料。")}</p>
                  <div className="mt-5 space-y-3">
                    <input value={docTitle} onChange={(event) => setDocTitle(event.target.value)} placeholder={ts("资料标题，可留空用文件名")} className={FIELD_CLASS} />
                    <input type="file" accept=".txt,.md,.json,text/plain" onChange={(event) => void onFile(event.target.files?.[0])} className="block w-full text-xs text-[color:var(--mcdl-text-body)]" />
                    <ul className="space-y-2">
                      {knowledge.map((doc) => (
                        <li key={doc.id} className="flex items-center justify-between gap-3 rounded-xl border border-[color:var(--mcdl-border-ghost)] px-3.5 py-2.5 text-xs">
                          <span className="truncate text-[color:var(--mcdl-text-body)]">{doc.title}</span>
                          <button
                            type="button"
                            className="shrink-0 text-[color:var(--mcdl-color-negative)]"
                            onClick={() => void api(`/api/digital-human/roles/${roleId}/knowledge/${doc.id}`, { method: "DELETE" }).then(() => setKnowledge((prev) => prev.filter((item) => item.id !== doc.id))).catch((err: Error) => setEditorError(err.message))}
                          >
                            {ts("移除")}
                          </button>
                        </li>
                      ))}
                    </ul>
                  </div>
                </section>
              )}
              {editorError && <p className="mt-5 rounded-xl bg-[color:var(--mcdl-accent-soft)] px-4 py-3 text-[0.75rem] leading-5 text-[color:var(--mcdl-color-negative)]">{editorError}</p>}
            </div>
            <footer className="flex shrink-0 items-center justify-end gap-3 bg-[color:var(--mcdl-color-surface-subtle)] px-6 py-4 sm:px-9 sm:py-5">
              {editorMode === "edit" && (
                <button type="button" disabled={busy === "删除中"} onClick={() => void removeRole()} className="mr-auto px-2 py-2 text-xs text-[color:var(--mcdl-color-negative)] disabled:opacity-50">
                  {busy === "删除中" ? ts("删除中") : ts("删除角色")}
                </button>
              )}
              <button type="button" onClick={() => setEditorOpen(false)} className="mcdl-button-secondary mcdl-hover-control px-6 py-2.5 text-sm">{ts("取消")}</button>
              <button type="button" disabled={busy === "保存中"} onClick={() => void saveEditor()} className="mcdl-button-primary mcdl-hover-primary-action px-7 py-2.5 text-sm font-semibold disabled:opacity-50">
                {busy === "保存中" ? ts("保存中") : ts("保存角色")}
              </button>
            </footer>
          </div>
        </div>,
        document.body,
      )}
    </div>
  );
}
