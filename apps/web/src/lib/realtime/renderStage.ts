import { characterStudioRuntimeURL, type CharacterStudioChannel } from "@/lib/characterStudio";

/**
 * 浏览器实时通道适配层。
 *
 * 页面只依赖这里的 RenderStage 接口；运行时脚本由本站中性路径代理加载。
 * 第三方运行时只在本文件内绑定，页面文案与业务组件不引用它的品牌/字段。
 */

type RuntimeEngine = {
  setChannelProfile?: (profile: string) => Promise<void> | void;
  setDefaultSubscribeAllRemoteAudioStreams?: (enabled: boolean) => Promise<void> | void;
  setDefaultSubscribeAllRemoteVideoStreams?: (enabled: boolean) => Promise<void> | void;
  subscribeRemoteVideoStream?: (userId: string, subscribe: boolean, track?: number) => Promise<void> | void;
  setLocalViewConfig?: (element: HTMLVideoElement, track: number) => Promise<void> | void;
  setRemoteViewConfig?: (element: HTMLVideoElement | null, userId: string, track: number) => void;
  publishLocalAudioStream?: (enabled: boolean) => Promise<void> | void;
  publishLocalVideoStream?: (enabled: boolean) => Promise<void> | void;
  joinChannel: (...args: unknown[]) => Promise<void>;
  leaveChannel?: () => Promise<void> | void;
  destroy?: () => void;
  switchCamera?: () => Promise<void> | void;
  getOnlineRemoteUsers?: () => string[];
  on?: (event: string, handler: (...args: unknown[]) => void) => void;
};

type RuntimeSDK = {
  getInstance: () => RuntimeEngine;
  isSupported?: () => Promise<{ support?: boolean; reason?: string }> | { support?: boolean; reason?: string };
  setLogLevel?: (level: number) => void;
};

type RuntimeWindow = Window & { AliRtcEngine?: RuntimeSDK };

let loading: Promise<RuntimeSDK> | null = null;

async function loadRuntime(): Promise<RuntimeSDK> {
  if (typeof window === "undefined") throw new Error("实时通道只能在浏览器中使用");
  const host = window as RuntimeWindow;
  if (host.AliRtcEngine) return host.AliRtcEngine;
  if (loading) return loading;
  loading = new Promise<RuntimeSDK>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>("script[data-realtime-runtime='1']");
    const done = () => {
      if (host.AliRtcEngine) resolve(host.AliRtcEngine);
      else reject(new Error("实时运行服务没有就绪，请刷新后重试"));
    };
    if (existing) {
      existing.addEventListener("load", done, { once: true });
      existing.addEventListener("error", () => reject(new Error("实时运行服务加载失败，请检查网络后重试")), { once: true });
      return;
    }
    const script = document.createElement("script");
    script.src = characterStudioRuntimeURL();
    script.async = true;
    script.dataset.realtimeRuntime = "1";
    script.addEventListener("load", done, { once: true });
    script.addEventListener("error", () => reject(new Error("实时运行服务加载失败，请检查网络后重试")), { once: true });
    document.head.appendChild(script);
  }).finally(() => {
    loading = null;
  });
  return loading;
}

const sleep = (ms: number) => new Promise<void>((resolve) => {
  window.setTimeout(resolve, ms);
});

/** 提前把运行时脚本加载好，避免开播后才下载导致接入窗口被拉长。 */
export async function preloadRenderStage(): Promise<void> {
  try {
    const sdk = await loadRuntime();
    await sdk.isSupported?.();
  } catch {
    /* 预热失败不阻塞开播，join 时会再尝试加载 */
  }
}

function subscribed(value: unknown) {
  return value === 3 || value === "3" || value === "subscribed";
}

export type RenderStageOptions = {
  channel: CharacterStudioChannel;
  outputChannel: string;
  localVideo: HTMLVideoElement;
  remoteVideo: HTMLVideoElement;
  onFirstFrame?: () => void;
  onRemoteOnline?: () => void;
  onRemoteOffline?: () => void;
  onClosed?: () => void;
};

export type RenderStage = {
  join: (options: RenderStageOptions) => Promise<void>;
  switchCamera: () => Promise<void>;
  leave: () => Promise<void>;
};

export async function createRenderStage(): Promise<RenderStage> {
  const sdk = await loadRuntime();
  const supported = await sdk.isSupported?.();
  if (supported && supported.support === false) {
    throw new Error("当前浏览器不支持实时视频，请使用 Chrome 或 Edge");
  }
  sdk.setLogLevel?.(0);
  const engine = sdk.getInstance();
  let firstFrame = false;
  let remoteBound = false;
  let options: RenderStageOptions | null = null;
  let frameTimer: number | null = null;

  const markFirstFrame = () => {
    if (firstFrame) return;
    firstFrame = true;
    if (frameTimer !== null) {
      window.clearTimeout(frameTimer);
      frameTimer = null;
    }
    options?.onFirstFrame?.();
  };

  const bindRemote = () => {
    if (!options || remoteBound) return;
    remoteBound = true;
    try {
      engine.setRemoteViewConfig?.(options.remoteVideo, options.outputChannel, 1);
    } catch {
      /* 远端画面可能还没推上来，等下一个订阅事件再绑 */
      remoteBound = false;
      return;
    }
    void options.remoteVideo.play().catch(() => undefined);
  };

  const subscribeOutput = () => {
    if (!options) return;
    try {
      void engine.subscribeRemoteVideoStream?.(options.outputChannel, true, 1);
    } catch {
      /* 远端用户可能还没进频道 */
    }
    bindRemote();
  };

  engine.on?.("videoSubscribeStateChanged", (userId, _oldState, newState) => {
    if (!options || String(userId) !== options.outputChannel) return;
    if (subscribed(newState)) {
      options.onRemoteOnline?.();
      subscribeOutput();
    }
  });
  engine.on?.("remoteUserOnLineNotify", (userId) => {
    if (!options) return;
    if (String(userId) === options.outputChannel) subscribeOutput();
  });
  engine.on?.("remoteUserOffLineNotify", (userId) => {
    if (!options) return;
    if (String(userId) === options.outputChannel) options.onRemoteOffline?.();
  });
  engine.on?.("bye", () => options?.onClosed?.());
  engine.on?.("authInfoExpired", () => options?.onClosed?.());

  return {
    async join(next: RenderStageOptions) {
      options = next;
      const expire = Number(next.channel.token_expire_at);
      firstFrame = false;
      remoteBound = false;
      next.remoteVideo.addEventListener("playing", markFirstFrame, { once: true });
      await engine.setChannelProfile?.("communication");
      await engine.setDefaultSubscribeAllRemoteAudioStreams?.(false);
      await engine.setDefaultSubscribeAllRemoteVideoStreams?.(true);
      try {
        await engine.setLocalViewConfig?.(next.localVideo, 1);
      } catch {
        /* 入会后会再绑定一次本地预览 */
      }
      try {
        // Web SDK 以创建接口返回的 authInfo 字符串入会；对象形态作为不同版本的兼容回退。
        await engine.joinChannel(next.channel.token, next.channel.user_id);
      } catch {
        await engine.joinChannel(
          {
            appId: next.channel.app_id,
            channelId: next.channel.channel_id,
            userId: next.channel.user_id,
            token: next.channel.token,
            timestamp: Number.isFinite(expire) ? expire : undefined,
          },
          next.channel.user_id,
        );
      }
      try {
        await engine.setLocalViewConfig?.(next.localVideo, 1);
      } catch {
        /* 部分浏览器需要等一帧才能绑定本地画面 */
      }
      await engine.publishLocalAudioStream?.(false);
      try {
        await engine.publishLocalVideoStream?.(true);
      } catch {
        // 设备刚被预览释放时可能短暂占用，等一帧再发布一次。
        await sleep(500);
        await engine.publishLocalVideoStream?.(true);
      }
      subscribeOutput();
      window.setTimeout(() => {
        try {
          const online = engine.getOnlineRemoteUsers?.() || [];
          if (online.includes(next.outputChannel)) subscribeOutput();
        } catch {
          /* 远端用户列表不是所有版本都暴露 */
        }
      }, 1500);
      // 订阅事件可能被浏览器吞掉，给一个保守的兜底绑定。
      frameTimer = window.setTimeout(() => {
        if (!remoteBound) subscribeOutput();
      }, 4000);
    },
    async switchCamera() {
      await engine.switchCamera?.();
    },
    async leave() {
      if (frameTimer !== null) {
        window.clearTimeout(frameTimer);
        frameTimer = null;
      }
      options = null;
      try {
        await engine.leaveChannel?.();
      } catch {
        /* 已经离会 */
      }
      try {
        engine.destroy?.();
      } catch {
        /* 已经销毁 */
      }
    },
  };
}
