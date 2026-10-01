"use client";

import Link from "next/link";
import Image from "next/image";
import { useRouter } from "next/navigation";
import { type CSSProperties, type Dispatch, type SetStateAction, useEffect, useMemo, useRef, useState } from "react";
import { ArrowRight, Bot, Boxes, Check, Clock3, Code2, Compass, Copy, Download, Headphones, ImageIcon, KeyRound, MessageCircle, Phone, Play, UserRound, Wand2, X } from "lucide-react";
import { siAlibabacloud, siAnthropic, siDeepseek, siFlux, siGooglegemini, siHuggingface, siKuaishou, type SimpleIcon } from "simple-icons";
import { LoginModal } from "@/components/LoginModal";
import { SiteBrand, useSiteBranding } from "@/components/SiteBrand";
import { UILanguageSelector } from "@/components/UILanguageSelector";
import { useI18n } from "@/i18n/I18nProvider";
import { hasUserSession } from "@/lib/api";
import { useAuthStore } from "@/store/auth";
import { loadReferenceGalleryManifest, randomReferenceCases, referenceImageURL, referenceTagEntries, referenceTaxonomyLabel, type ReferenceGalleryItem } from "@/components/workbench/galleryReference";

const MODEL_LOGOS = ["GPT", "Claude", "Gemini", "Sora", "Flux", "Kling", "MJ", "Qwen", "DeepSeek", "Runway"];
type TickerLogo = { name: string; color: string; icon?: SimpleIcon; mark?: string; variant?: "ring" | "spark" | "rune" | "waves" };
const MODEL_TICKER: TickerLogo[] = [
  { name: "OpenAI", mark: "∞", color: "#10A37F", variant: "ring" },
  { name: "Claude", icon: siAnthropic, color: `#${siAnthropic.hex}` },
  { name: "Gemini", icon: siGooglegemini, color: `#${siGooglegemini.hex}` },
  { name: "DeepSeek", icon: siDeepseek, color: `#${siDeepseek.hex}` },
  { name: "Qwen", icon: siAlibabacloud, color: `#${siAlibabacloud.hex}` },
  { name: "Flux", icon: siFlux, color: `#${siFlux.hex}` },
  { name: "Kling", icon: siKuaishou, color: `#${siKuaishou.hex}` },
  { name: "Hugging Face", icon: siHuggingface, color: `#${siHuggingface.hex}` },
  { name: "Runway", mark: "R", color: "#00D8FF", variant: "waves" },
  { name: "Midjourney", mark: "M", color: "#F5F7FB", variant: "rune" },
  { name: "Stable Diffusion", mark: "S", color: "#2563EB", variant: "spark" },
  { name: "Pika", mark: "P", color: "#FF4FD8", variant: "spark" },
  { name: "Sora", mark: "S", color: "#F8FAFC", variant: "waves" },
  { name: "Luma", mark: "L", color: "#7DD3FC", variant: "spark" },
  { name: "Veo", mark: "V", color: "#34D399", variant: "ring" },
  { name: "MiniMax", mark: "M", color: "#A78BFA", variant: "rune" },
  { name: "Doubao", mark: "D", color: "#FB7185", variant: "spark" },
  { name: "GLM", mark: "G", color: "#60A5FA", variant: "ring" },
  { name: "PixVerse", mark: "P", color: "#F97316", variant: "waves" },
  { name: "Hailuo", mark: "H", color: "#FACC15", variant: "spark" },
  { name: "Grok", mark: "G", color: "#CBD5E1", variant: "rune" },
  { name: "Hunyuan", mark: "H", color: "#38BDF8", variant: "ring" },
  { name: "Ideogram", mark: "I", color: "#F472B6", variant: "spark" },
  { name: "Recraft", mark: "R", color: "#A3E635", variant: "waves" },
  { name: "Wanxiang", mark: "W", color: "#818CF8", variant: "spark" },
  { name: "Kimi", mark: "K", color: "#22D3EE", variant: "ring" },
  { name: "Mistral", mark: "M", color: "#F59E0B", variant: "rune" },
  { name: "Perplexity", mark: "P", color: "#14B8A6", variant: "waves" },
];

const LANDING_GALLERY_LIMIT = 12;

function InteractiveHeroCanvas() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;
    const ctx = canvas.getContext("2d");
    if (!ctx) return;

    let width = 0;
    let height = 0;
    let frame = 0;
    let idleFrames = 0;
    let animation = 0;
    let paused = false;
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches;
    const isCompact = window.matchMedia?.("(max-width: 768px)")?.matches;
    const pointer = { x: window.innerWidth * 0.5, y: window.innerHeight * 0.46, px: 0, py: 0, active: false };
    const waves: Array<{ x: number; y: number; r: number; life: number; hue: number }> = [];
    const meteors: Array<{ x: number; y: number; vx: number; vy: number; life: number; hue: number }> = [];
    const logos: Array<{ x: number; y: number; text: string; life: number; drift: number }> = [];
    const nodes = Array.from({ length: reduceMotion ? 18 : isCompact ? 34 : 76 }, () => ({
      x: Math.random(),
      y: Math.random(),
      vx: (Math.random() - 0.5) * 0.18,
      vy: (Math.random() - 0.5) * 0.18,
      r: Math.random() * 1.8 + 0.6,
    }));

    const resize = () => {
      const ratio = Math.min(window.devicePixelRatio || 1, 2);
      width = window.innerWidth;
      height = Math.max(window.innerHeight, 720);
      canvas.width = Math.floor(width * ratio);
      canvas.height = Math.floor(height * ratio);
      canvas.style.width = `${width}px`;
      canvas.style.height = `${height}px`;
      ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    };

    const addWave = (x: number, y: number) => {
      waves.push({ x, y, r: 5, life: 1, hue: Math.random() > 0.5 ? 28 : 38 });
      if (waves.length > 24) waves.shift();
    };

    const addMeteor = (x: number, y: number, dx: number, dy: number) => {
      const speed = Math.max(3, Math.min(16, Math.hypot(dx, dy) * 0.28));
      const len = Math.max(1, Math.hypot(dx, dy));
      meteors.push({ x, y, vx: (dx / len) * speed, vy: (dy / len) * speed, life: 1, hue: Math.random() > 0.5 ? 28 : 38 });
      if (meteors.length > 36) meteors.shift();
    };

    const addLogo = () => {
      logos.push({
        x: pointer.x + (Math.random() - 0.5) * 110,
        y: pointer.y + (Math.random() - 0.5) * 90,
        text: MODEL_LOGOS[Math.floor(Math.random() * MODEL_LOGOS.length)],
        life: 1,
        drift: Math.random() * 0.8 + 0.25,
      });
      if (logos.length > 8) logos.shift();
    };

    const onPointerMove = (event: PointerEvent) => {
      pointer.active = true;
      pointer.px = pointer.x;
      pointer.py = pointer.y;
      pointer.x = event.clientX;
      pointer.y = event.clientY;
      idleFrames = 0;
      if (frame % 2 === 0) addWave(pointer.x, pointer.y);
      addMeteor(pointer.x, pointer.y, pointer.x - pointer.px, pointer.y - pointer.py);
    };

    const onPointerDown = (event: PointerEvent) => {
      for (let i = 0; i < 5; i++) {
        waves.push({ x: event.clientX, y: event.clientY, r: 12 + i * 15, life: 1, hue: i % 2 ? 28 : 38 });
      }
    };

    const draw = () => {
      if (paused) {
        animation = requestAnimationFrame(draw);
        return;
      }
      frame += 1;
      idleFrames += 1;
      ctx.clearRect(0, 0, width, height);
      ctx.fillStyle = "#0a0a0a";
      ctx.fillRect(0, 0, width, height);

      nodes.forEach((p, i) => {
        p.x += p.vx / width;
        p.y += p.vy / height;
        if (p.x < 0 || p.x > 1) p.vx *= -1;
        if (p.y < 0 || p.y > 1) p.vy *= -1;
        const x = p.x * width;
        const y = p.y * height;
        const dx = x - pointer.x;
        const dy = y - pointer.y;
        const pull = Math.max(0, 1 - Math.hypot(dx, dy) / 260);
        ctx.beginPath();
        ctx.arc(x + dx * pull * 0.018, y + dy * pull * 0.018, p.r + pull * 1.8, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(${pull > 0 ? "255,135,17" : "245,239,235"},${0.14 + pull * 0.32})`;
        ctx.fill();
        for (let j = i + 1; j < nodes.length; j++) {
          const b = nodes[j];
          const bx = b.x * width;
          const by = b.y * height;
          const dist = Math.hypot(x - bx, y - by);
          if (dist < 120) {
            ctx.beginPath();
            ctx.moveTo(x, y);
            ctx.lineTo(bx, by);
            ctx.strokeStyle = `rgba(245,239,235,${0.055 * (1 - dist / 120)})`;
            ctx.stroke();
          }
        }
      });

      waves.forEach((w, idx) => {
        w.r += 4.6;
        w.life -= 0.025;
        ctx.beginPath();
        ctx.arc(w.x, w.y, w.r, 0, Math.PI * 2);
        ctx.strokeStyle = `hsla(${w.hue}, 92%, 62%, ${Math.max(0, w.life) * 0.38})`;
        ctx.lineWidth = 1.6;
        ctx.stroke();
        if (w.life <= 0) waves.splice(idx, 1);
      });

      meteors.forEach((m, idx) => {
        m.x += m.vx;
        m.y += m.vy;
        m.life -= 0.035;
        ctx.beginPath();
        ctx.moveTo(m.x, m.y);
        ctx.lineTo(m.x - m.vx * 4.5, m.y - m.vy * 4.5);
        ctx.strokeStyle = `hsla(${m.hue}, 94%, 66%, ${Math.max(0, m.life) * 0.7})`;
        ctx.lineWidth = 2;
        ctx.stroke();
        if (m.life <= 0) meteors.splice(idx, 1);
      });

      if (pointer.active && idleFrames > 72 && frame % 52 === 0) addLogo();
      logos.forEach((logo, idx) => {
        logo.life -= 0.012;
        logo.y -= logo.drift;
        const alpha = Math.max(0, Math.min(1, logo.life));
        ctx.save();
        ctx.globalAlpha = alpha;
        ctx.font = "600 13px Manrope, ui-sans-serif, system-ui";
        const tw = ctx.measureText(logo.text).width + 24;
        ctx.fillStyle = "rgba(22, 22, 22, 0.84)";
        ctx.strokeStyle = "rgba(245,239,235,0.14)";
        ctx.lineWidth = 1;
        ctx.beginPath();
        ctx.roundRect(logo.x - tw / 2, logo.y - 16, tw, 30, 15);
        ctx.fill();
        ctx.stroke();
        ctx.fillStyle = "rgba(245,239,235,0.9)";
        ctx.fillText(logo.text, logo.x - tw / 2 + 12, logo.y + 4);
        ctx.restore();
        if (logo.life <= 0) logos.splice(idx, 1);
      });

      if (!reduceMotion) {
        animation = requestAnimationFrame(draw);
      }
    };

    const onVisibilityChange = () => {
      paused = document.hidden;
      if (!paused && !reduceMotion) {
        cancelAnimationFrame(animation);
        animation = requestAnimationFrame(draw);
      }
    };

    resize();
    paused = document.hidden;
    window.addEventListener("resize", resize);
    window.addEventListener("pointermove", onPointerMove);
    window.addEventListener("pointerdown", onPointerDown);
    document.addEventListener("visibilitychange", onVisibilityChange);
    draw();

    return () => {
      cancelAnimationFrame(animation);
      window.removeEventListener("resize", resize);
      window.removeEventListener("pointermove", onPointerMove);
      window.removeEventListener("pointerdown", onPointerDown);
      document.removeEventListener("visibilitychange", onVisibilityChange);
    };
  }, []);

  return <canvas ref={canvasRef} className="pointer-events-none absolute inset-0 h-[max(100vh,720px)] w-full" />;
}

function AnimatedHeadline() {
  const { t } = useI18n();
  const [index, setIndex] = useState(0);
  const [text, setText] = useState("");
  const [deleting, setDeleting] = useState(false);
  const phrases = useMemo(
    () => [
      t("landing.hero.phrase1"),
      t("landing.hero.phrase2"),
      t("landing.hero.phrase3"),
      t("landing.hero.phrase4"),
    ],
    [t]
  );

  useEffect(() => {
    const phrase = phrases[index % phrases.length];
    const doneTyping = !deleting && text === phrase;
    const doneDeleting = deleting && text === "";
    const delay = doneTyping ? 1300 : deleting ? 34 : 72;
    const timer = window.setTimeout(() => {
      if (doneTyping) {
        setDeleting(true);
        return;
      }
      if (doneDeleting) {
        setDeleting(false);
        setIndex((v) => v + 1);
        return;
      }
      setText((current) => (deleting ? phrase.slice(0, Math.max(0, current.length - 1)) : phrase.slice(0, current.length + 1)));
    }, delay);
    return () => window.clearTimeout(timer);
  }, [deleting, index, phrases, text]);

  return (
    <h1 className="mcdl-display max-w-5xl text-[clamp(3rem,7vw,6.5rem)] font-semibold leading-[0.92]">
      {t("landing.titlePrefix")}
      <span className="mt-3 block min-h-[1.05em] text-[var(--mcdl-color-accent)]">
        {t("landing.titleSuffix", { value: text })}
        <span className="ml-1 inline-block h-[0.78em] w-[0.06em] translate-y-[0.08em] animate-[landingBlink_1s_steps(2,end)_infinite] bg-[var(--mcdl-color-accent)]" />
      </span>
    </h1>
  );
}

function ModelTickerLogo({ item }: { item: TickerLogo }) {
  if (item.icon) {
    return (
      <svg viewBox="0 0 24 24" className="h-6 w-6" role="img" aria-label={item.name}>
        <path d={item.icon.path} fill={item.color} />
      </svg>
    );
  }
  const glow = `0 0 18px ${item.color}66`;
  if (item.variant === "ring") {
    return (
      <span className="relative flex h-7 w-7 items-center justify-center" aria-label={item.name} role="img">
        <span className="absolute inset-0 rounded-full border-2 opacity-80" style={{ borderColor: item.color, boxShadow: glow }} />
        <span className="absolute h-3.5 w-3.5 rounded-full border-2 opacity-70" style={{ borderColor: item.color }} />
      </span>
    );
  }
  if (item.variant === "waves") {
    return (
      <span className="flex h-7 w-7 items-center justify-center gap-0.5" aria-label={item.name} role="img">
        {[0, 1, 2].map((i) => (
          <span key={i} className="h-5 w-1 rounded-full" style={{ background: item.color, opacity: 0.55 + i * 0.18, boxShadow: glow }} />
        ))}
      </span>
    );
  }
  return (
    <span
      className="flex h-7 min-w-7 items-center justify-center text-sm font-black tracking-normal"
      style={{ color: item.color, textShadow: glow }}
      aria-label={item.name}
      role="img"
    >
      {item.mark}
    </span>
  );
}

function ModelTicker() {
  const items = [...MODEL_TICKER, ...MODEL_TICKER];
  return (
    <div className="mcdl-glass mt-9 w-full max-w-full overflow-hidden rounded-[1.35rem] py-3 lg:hidden">
      <div className="flex w-max animate-[landingMarquee_26s_linear_infinite] items-center gap-3 px-3">
        {items.map((item, index) => (
          <span
            key={`${item.name}-${index}`}
            title={item.name}
            className="mcdl-hover-control mcdl-hover-press group flex h-12 w-12 items-center justify-center rounded-2xl border border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.055)]"
          >
            <ModelTickerLogo item={item} />
          </span>
        ))}
      </div>
    </div>
  );
}

function OrbitTypewriter({ items, activeIndex, setActiveIndex }: { items: TickerLogo[]; activeIndex: number; setActiveIndex: Dispatch<SetStateAction<number>> }) {
  const [text, setText] = useState(items[0]?.name || "");
  const [deleting, setDeleting] = useState(false);
  const [visibleTick, setVisibleTick] = useState(0);

  useEffect(() => {
    const onVisibilityChange = () => {
      if (!document.hidden) setVisibleTick((value) => value + 1);
    };
    document.addEventListener("visibilitychange", onVisibilityChange);
    return () => document.removeEventListener("visibilitychange", onVisibilityChange);
  }, []);

  useEffect(() => {
    if (!items.length) return;
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches;
    if (reduceMotion) {
      setText(items[activeIndex]?.name || "");
      return;
    }
    if (document.hidden) return;
    const active = items[activeIndex % items.length];
    const name = active.name;
    const doneTyping = !deleting && text === name;
    const doneDeleting = deleting && text === "";
    const delay = doneTyping ? 2100 : deleting ? 58 : 118;
    const timer = window.setTimeout(() => {
      if (doneTyping) {
        setDeleting(true);
        return;
      }
      if (doneDeleting) {
        setDeleting(false);
        setActiveIndex((current) => {
          if (items.length <= 1) return current;
          let next = Math.floor(Math.random() * items.length);
          if (next === current) next = (next + 1) % items.length;
          return next;
        });
        return;
      }
      setText((current) => (deleting ? name.slice(0, Math.max(0, current.length - 1)) : name.slice(0, current.length + 1)));
    }, delay);
    return () => window.clearTimeout(timer);
  }, [activeIndex, deleting, items, setActiveIndex, text, visibleTick]);

  const active = items[activeIndex % items.length] || items[0];
  if (!active) return null;

  return (
    <div className="absolute left-1/2 top-1/2 z-10 flex -translate-x-1/2 -translate-y-1/2 flex-col items-center text-center opacity-60 transition duration-500 group-hover/orbit:opacity-100">
      <div key={active.name} className="relative flex h-14 w-14 animate-[landingOrbitFocusIn_.65s_ease-out] items-center justify-center rounded-2xl border border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.035)] text-[var(--mcdl-text-body)] shadow-[0_0_36px_rgba(255,135,17,.08)]">
        <span className="absolute inset-0 rounded-2xl bg-[rgba(255,135,17,.05)] blur-xl" />
        <span className="relative opacity-70 transition group-hover/orbit:opacity-100">
          <ModelTickerLogo item={active} />
        </span>
      </div>
      <div className="mt-3 min-h-[22px] max-w-[170px] truncate text-sm font-semibold tracking-normal text-[var(--mcdl-text-body)] transition group-hover/orbit:text-[var(--mcdl-text-title)]">
        {text}
        <span className="ml-0.5 inline-block h-[1em] w-px translate-y-0.5 animate-[landingBlink_1s_steps(2,end)_infinite] bg-[var(--mcdl-color-accent)]" />
      </div>
    </div>
  );
}

function ModelOrbit() {
  const items = MODEL_TICKER.slice(0, 24);
  const [activeIndex, setActiveIndex] = useState(0);
  return (
    <div className="group/orbit relative z-30 isolate hidden h-[620px] w-[620px] shrink-0 items-center justify-center overflow-visible lg:flex">
      <div className="absolute inset-28 rounded-full border border-[rgba(245,239,235,.06)] bg-[rgba(255,135,17,.018)]" />
      <OrbitTypewriter items={items} activeIndex={activeIndex} setActiveIndex={setActiveIndex} />
      <div className="absolute left-1/2 top-1/2 z-20 h-0 w-0 animate-[landingOrbitSpin_46s_linear_infinite] overflow-visible motion-reduce:animate-none group-hover/orbit:[animation-play-state:paused]">
        {items.map((item, index) => {
          const angle = (360 / items.length) * index;
          const active = items[activeIndex % items.length];
          const isActive = item.name === active?.name;
          return (
            <div
              key={item.name}
              className="absolute left-0 top-0 hover:z-50"
              style={{ transform: `rotate(${angle}deg) translate(260px)`, transformOrigin: "0 0" }}
            >
              <div
                className="animate-[landingOrbitNodeSpin_46s_linear_infinite] motion-reduce:animate-none group-hover/orbit:[animation-play-state:paused]"
                style={{ "--start-angle": `-${angle}deg` } as CSSProperties}
              >
                {isActive ? (
                  <div className="invisible h-[64px] w-[68px]" aria-hidden="true" />
                ) : (
                  <div
                    className="mcdl-hover-control group/node flex w-[68px] flex-col items-center gap-1.5 rounded-2xl border border-transparent bg-[rgba(22,22,22,.5)] px-1.5 py-2 opacity-35 backdrop-blur-sm hover:opacity-100"
                    title={item.name}
                  >
                    <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-xl bg-[rgba(245,239,235,.025)] opacity-50 transition group-hover/node:opacity-100">
                      <ModelTickerLogo item={item} />
                    </span>
                    <span className="w-full truncate text-center text-[10px] font-semibold text-[var(--mcdl-text-muted)] transition group-hover/node:text-[var(--mcdl-text-title)]">
                      {item.name}
                    </span>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

type CustomerServiceConfig = ReturnType<typeof useSiteBranding>;

function isCustomerServiceEnabled(value: unknown) {
  return value === undefined || !(value === false || value === 0 || String(value).toLowerCase() === "false");
}

function ThirdPartyCustomerServiceScript({ enabled, code }: { enabled: unknown; code?: string }) {
  const mountRef = useRef<HTMLSpanElement>(null);
  const active = isCustomerServiceEnabled(enabled);

  useEffect(() => {
    const source = code?.trim();
    const mount = mountRef.current;
    if (!active || !source || !mount) return;

    const runtimeWindow = window as Window & { __starAICustomerServiceScript?: string };
    if (runtimeWindow.__starAICustomerServiceScript === source) return;

    const template = document.createElement("template");
    template.innerHTML = source;
    const scripts = Array.from(template.content.querySelectorAll("script"));
    if (scripts.length === 0) return;

    runtimeWindow.__starAICustomerServiceScript = source;
    scripts.forEach((sourceScript) => {
      const script = document.createElement("script");
      Array.from(sourceScript.attributes).forEach((attribute) => script.setAttribute(attribute.name, attribute.value));
      script.textContent = sourceScript.textContent;
      script.dataset.staraiCustomerService = "true";
      mount.appendChild(script);
    });

    return () => {
      if (runtimeWindow.__starAICustomerServiceScript === source) delete runtimeWindow.__starAICustomerServiceScript;
    };
  }, [active, code]);

  if (!active || !code?.trim()) return null;
  return <span ref={mountRef} className="hidden" aria-hidden="true" />;
}

function CustomerService({ config }: { config: CustomerServiceConfig }) {
  const { t } = useI18n();
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState("");
  const enabled = isCustomerServiceEnabled(config.customer_service_enabled);
  if (!enabled) return null;

  const title = config.customer_service_title || t("customerService.title");
  const name = config.customer_service_name || t("customerService.name");
  const subtitle = config.customer_service_subtitle || t("customerService.subtitle");
  const qrTip = config.customer_service_qr_tip || t("customerService.qrTip");
  const copyValue = async (label: string, value?: string) => {
    if (!value) return;
    await navigator.clipboard?.writeText(value);
    setCopied(label);
    window.setTimeout(() => setCopied(""), 1600);
  };
  const downloadQR = () => {
    if (!config.customer_service_qr_url) return;
    const link = document.createElement("a");
    link.href = config.customer_service_qr_url;
    link.download = "customer-service-qr";
    link.target = "_blank";
    link.rel = "noreferrer";
    link.click();
  };

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label={t("customerService.open")}
        className="mcdl-glass mcdl-hover-control mcdl-hover-press group fixed bottom-5 right-4 z-[80] flex h-16 w-16 items-center justify-center overflow-hidden rounded-2xl p-1.5 text-[var(--mcdl-color-accent)] sm:bottom-7 sm:right-7 sm:h-[72px] sm:w-[72px]"
      >
        {config.customer_service_floating_image ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={config.customer_service_floating_image} alt={name} className="h-full w-full rounded-xl object-contain" />
        ) : (
          <Headphones size={30} />
        )}
        <span className="absolute right-1.5 top-1.5 h-2.5 w-2.5 rounded-full border-2 border-[var(--mcdl-color-surface)] bg-[var(--mcdl-color-positive)]" />
      </button>

      {open && (
        <div className="fixed inset-0 z-[120] flex items-end justify-center bg-black/75 p-0 backdrop-blur-sm sm:items-center sm:p-5" onClick={() => setOpen(false)}>
          <div
            role="dialog"
            aria-modal="true"
            aria-label={t("customerService.dialog")}
            className="mcdl-editorial-card max-h-[92vh] w-full overflow-y-auto rounded-t-[1.75rem] text-[var(--mcdl-text-title)] sm:max-w-[390px] sm:rounded-[1.75rem]"
            onClick={(event) => event.stopPropagation()}
          >
            <div className="flex items-center justify-between px-5 pb-3 pt-5">
              <div className="flex items-center gap-3">
                <div className="grid h-10 w-10 place-items-center rounded-xl bg-[rgba(255,135,17,.1)] text-[var(--mcdl-color-accent)]"><Headphones size={21} /></div>
                <div>
                  <div className="text-lg font-bold">{title}</div>
                  <div className="mcdl-body-copy text-xs">{subtitle}</div>
                </div>
              </div>
              <button type="button" onClick={() => setOpen(false)} className="mcdl-icon-button mcdl-hover-control mcdl-hover-press grid h-9 w-9 place-items-center rounded-xl" aria-label={t("common.close")}>
                <X size={19} />
              </button>
            </div>

            <div className="space-y-4 p-5">
              <div className="mcdl-stage flex items-center gap-3 border border-[var(--mcdl-border-ghost)] p-4">
                <div className="grid h-12 w-12 shrink-0 place-items-center overflow-hidden rounded-xl bg-[rgba(245,239,235,.04)] text-[var(--mcdl-text-body)]">
                  {config.customer_service_avatar ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img src={config.customer_service_avatar} alt={name} className="h-full w-full object-cover" />
                  ) : (
                    <UserRound size={25} />
                  )}
                </div>
                <div>
                  <div className="font-semibold">{name}</div>
                  <div className="mt-1 inline-flex items-center gap-1.5 rounded-full bg-[rgba(98,198,147,.1)] px-2 py-1 text-[11px] font-medium text-[var(--mcdl-color-positive)]">
                    <span className="h-1.5 w-1.5 rounded-full bg-[var(--mcdl-color-positive)]" />{t("customerService.online")}
                  </div>
                </div>
              </div>

              {config.customer_service_qr_url && (
                <div className="text-center">
                  <div className="mx-auto w-fit rounded-2xl bg-white p-2.5 shadow-lg">
                    {/* eslint-disable-next-line @next/next/no-img-element */}
                    <img src={config.customer_service_qr_url} alt={t("客服微信二维码")} className="h-44 w-44 object-contain sm:h-48 sm:w-48" />
                  </div>
                  <div className="mcdl-muted-copy mt-2 text-xs">{qrTip}</div>
                </div>
              )}

              <div className="space-y-2">
                {config.customer_service_phone && (
                  <button type="button" onClick={() => copyValue("phone", config.customer_service_phone)} className="mcdl-hover-quiet mcdl-hover-press flex w-full items-center justify-between rounded-xl border border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.035)] px-3.5 py-3 text-left transition">
                    <span className="flex items-center gap-3"><Phone size={18} className="text-[var(--mcdl-color-accent)]" /><span><span className="mcdl-muted-copy block text-[11px]">{t("customerService.phone")}</span><span className="text-sm font-semibold">{config.customer_service_phone}</span></span></span>
                    {copied === "phone" ? <Check size={17} className="text-[var(--mcdl-color-positive)]" /> : <Copy size={17} className="mcdl-muted-copy" />}
                  </button>
                )}
                {config.customer_service_wechat && (
                  <button type="button" onClick={() => copyValue("wechat", config.customer_service_wechat)} className="mcdl-hover-quiet mcdl-hover-press flex w-full items-center justify-between rounded-xl border border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.035)] px-3.5 py-3 text-left transition">
                    <span className="flex items-center gap-3"><MessageCircle size={18} className="text-[var(--mcdl-color-accent)]" /><span><span className="mcdl-muted-copy block text-[11px]">{t("customerService.wechat")}</span><span className="text-sm font-semibold">{config.customer_service_wechat}</span></span></span>
                    {copied === "wechat" ? <Check size={17} className="text-[var(--mcdl-color-positive)]" /> : <Copy size={17} className="mcdl-muted-copy" />}
                  </button>
                )}
                {config.customer_service_hours && (
                  <div className="flex items-center gap-3 rounded-xl border border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.035)] px-3.5 py-3">
                    <Clock3 size={18} className="text-[var(--mcdl-color-accent)]" />
                    <span><span className="mcdl-muted-copy block text-[11px]">{t("customerService.hours")}</span><span className="text-sm font-semibold">{config.customer_service_hours}</span></span>
                  </div>
                )}
              </div>
            </div>

            <div className="grid grid-cols-2 gap-2 p-5 pt-3">
              <button type="button" onClick={() => setOpen(false)} className="mcdl-button-secondary mcdl-hover-control mcdl-hover-press rounded-xl py-3 text-sm font-semibold">{t("common.gotIt")}</button>
              <button type="button" onClick={downloadQR} disabled={!config.customer_service_qr_url} className="mcdl-button-primary mcdl-hover-primary-action mcdl-hover-press inline-flex items-center justify-center gap-2 rounded-xl py-3 text-sm font-semibold disabled:cursor-not-allowed disabled:opacity-40">
                <Download size={17} />{t("customerService.downloadQR")}
              </button>
            </div>
          </div>
        </div>
      )}
    </>
  );
}

function GalleryPreview({ item }: { item: ReferenceGalleryItem }) {
  const { locale } = useI18n();
  const [imageFailed, setImageFailed] = useState(false);
  const tags = referenceTagEntries(item).slice(0, 3);
  return (
    <Link
      href="/app/gallery"
      onClick={(event) => {
        event.preventDefault();
        window.location.assign(event.currentTarget.href);
      }}
      className="mcdl-editorial-card mcdl-hover-card mcdl-hover-press group block min-w-0 overflow-hidden text-left"
    >
      <div className="mcdl-stage relative aspect-[4/5] overflow-hidden rounded-b-none">
        {imageFailed ? (
          <div className="flex h-full flex-col items-center justify-center gap-3 px-5 text-center">
            <span className="grid h-12 w-12 place-items-center rounded-2xl border border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.035)] text-[var(--mcdl-color-accent)]">
              <ImageIcon size={22} />
            </span>
            <span className="mcdl-muted-copy text-xs">{referenceTaxonomyLabel(item.category, locale)}</span>
          </div>
        ) : (
          <Image
            src={referenceImageURL(item.image)}
            alt={item.imageAlt || item.title}
            fill
            sizes="(min-width: 1024px) 25vw, 50vw"
            className="mcdl-media-hover object-contain"
            onError={() => setImageFailed(true)}
          />
        )}
      </div>
      <div className="p-3.5">
        <div className="flex items-center gap-2 text-[10px] font-semibold uppercase tracking-[0.12em] text-[var(--mcdl-color-accent)]">
          <span className="truncate">{referenceTaxonomyLabel(item.category, locale)}</span>
          <span className="mcdl-numeric mcdl-muted-copy ml-auto shrink-0 normal-case tracking-normal">Case {item.id}</span>
        </div>
        <h3 className="mt-2 truncate text-sm font-semibold text-[var(--mcdl-text-title)]">{item.title}</h3>
        <p className="mcdl-body-copy mt-2 line-clamp-2 text-xs leading-5">{item.promptPreview || item.prompt}</p>
        <div className="mt-3 flex flex-wrap gap-1.5">
          {tags.map((tag) => (
            <span key={tag.key} className="mcdl-muted-copy rounded-full border border-[var(--mcdl-border-ghost)] px-2 py-0.5 text-[10px]">
              {referenceTaxonomyLabel(tag.label, locale)}
            </span>
          ))}
        </div>
      </div>
    </Link>
  );
}

export default function LandingPageClient() {
  const router = useRouter();
  const { t } = useI18n();
  const { token, hydrate } = useAuthStore();
  const [showLogin, setShowLogin] = useState(false);
  const [gallery, setGallery] = useState<ReferenceGalleryItem[]>([]);
  const [galleryLoading, setGalleryLoading] = useState(true);
  const branding = useSiteBranding();
  const { site_name, site_copyright, api_docs_enabled, api_docs_operations } = branding;
  const apiDocsVisible = api_docs_enabled !== false && (!api_docs_operations || Object.keys(api_docs_operations).length === 0 || Object.values(api_docs_operations).some((value) => value !== false));
  const copyrightText = site_copyright || `© ${new Date().getFullYear()} ${site_name || "StarAI"}. All rights reserved.`;

  useEffect(() => {
    hydrate();
  }, [hydrate]);

  const enterAppOrLogin = () => {
    if (token || hasUserSession()) {
      router.push("/app");
      return;
    }
    setShowLogin(true);
  };

  useEffect(() => {
    const query = new URLSearchParams(window.location.search);
    if (query.get("referral_code") || query.get("login") === "1") setShowLogin(true);
  }, []);

  useEffect(() => {
    let active = true;
    loadReferenceGalleryManifest()
      .then((manifest) => {
        if (active) setGallery(randomReferenceCases(manifest.cases, LANDING_GALLERY_LIMIT));
      })
      .catch(() => { if (active) setGallery([]); })
      .finally(() => { if (active) setGalleryLoading(false); });
    return () => { active = false; };
  }, []);

  const capabilityCards = useMemo(
    () => [
      { title: t("landing.capability.chat.title"), desc: t("landing.capability.chat.desc"), icon: Bot },
      { title: t("landing.capability.media.title"), desc: t("landing.capability.media.desc"), icon: ImageIcon },
      { title: t("landing.capability.api.title"), desc: t("landing.capability.api.desc"), icon: Code2 },
      { title: t("landing.capability.agent.title"), desc: t("landing.capability.agent.desc"), icon: Boxes },
    ],
    [t],
  );

  return (
    <div className="mcdl-marketing-page">
      <style jsx global>{`
        @keyframes landingBlink {
          0%,
          45% {
            opacity: 1;
          }
          46%,
          100% {
            opacity: 0;
          }
        }
        @keyframes landingMarquee {
          from {
            transform: translateX(0);
          }
          to {
            transform: translateX(-50%);
          }
        }
        @keyframes landingOrbitSpin {
          from {
            transform: rotate(0deg);
          }
          to {
            transform: rotate(360deg);
          }
        }
        @keyframes landingOrbitNodeSpin {
          from {
            transform: translate(-50%, -50%) rotate(var(--start-angle));
          }
          to {
            transform: translate(-50%, -50%) rotate(calc(var(--start-angle) - 360deg));
          }
        }
        @keyframes landingOrbitFocusIn {
          from {
            opacity: 0;
            transform: translate3d(0, 18px, 0) scale(0.82);
          }
          to {
            opacity: 1;
            transform: translate3d(0, 0, 0) scale(1);
          }
        }
      `}</style>
      <section className="relative min-h-screen overflow-hidden bg-[var(--mcdl-color-surface)]">
        <InteractiveHeroCanvas />
        <nav className="mcdl-glass relative z-[70] mx-3 mt-3 flex max-w-[calc(80rem-2rem)] items-center justify-between gap-2 rounded-[1.4rem] px-3 py-3 sm:mx-auto sm:mt-5 sm:px-5">
          <SiteBrand
            href="/"
            className="min-w-0 flex-1 gap-2 pr-1"
            nameClassName="text-lg font-bold text-[var(--mcdl-text-title)] sm:text-xl"
            subtitleClassName="mcdl-body-copy max-w-[92px] text-xs min-[390px]:max-w-[130px] sm:max-w-none sm:text-sm"
            badgeClassName="h-8 w-8 rounded-lg !bg-[var(--mcdl-color-accent)] text-sm !text-[var(--mcdl-color-on-accent)]"
          />
          <div className="flex shrink-0 items-center gap-1.5 sm:gap-4">
            {apiDocsVisible && (
              <Link href="/app/api-docs" onClick={(event) => { event.preventDefault(); window.location.assign(event.currentTarget.href); }} className="mcdl-button-secondary mcdl-hover-quiet mcdl-hover-press hidden px-4 py-2 text-sm sm:inline-flex">
                {t("landing.apiDocs")}
              </Link>
            )}
            <button onClick={enterAppOrLogin} className="mcdl-button-secondary mcdl-hover-control mcdl-hover-press h-9 max-w-[54px] truncate whitespace-nowrap px-2.5 text-xs leading-none sm:h-10 sm:max-w-none sm:px-5 sm:text-sm">
              {t("landing.login")}
            </button>
            <button onClick={enterAppOrLogin} className="mcdl-button-primary mcdl-hover-primary-action mcdl-hover-press h-9 max-w-[78px] truncate whitespace-nowrap px-3 text-xs font-semibold leading-none min-[390px]:max-w-[92px] sm:h-10 sm:max-w-none sm:px-5 sm:text-sm">
              <span className="block truncate">{t("landing.start")}</span>
            </button>
            <UILanguageSelector compact tone="dark" className="mcdl-landing-language" />
          </div>
        </nav>

        <main className="relative z-10 mx-auto flex min-h-[calc(100vh-88px)] w-full max-w-[80rem] min-w-0 flex-col justify-center overflow-hidden px-5 pb-16 pt-10 sm:px-7">
          <div className="grid w-full min-w-0 items-center gap-10 lg:grid-cols-[minmax(0,1fr)_620px] xl:gap-16">
            <div className="w-full max-w-4xl min-w-0">
              <div className="mcdl-eyebrow mb-8">
                {t("landing.badge")}
              </div>
              <AnimatedHeadline />
              <p className="mcdl-body-copy mt-7 max-w-full break-words text-base leading-8 sm:max-w-2xl sm:text-lg">
                {t("landing.desc", { site: site_name || "StarAI" })}
              </p>
              <div className="mt-10 flex min-w-0 flex-col gap-3 sm:flex-row">
                <button onClick={enterAppOrLogin} className="mcdl-button-primary mcdl-hover-primary-action mcdl-hover-press group box-border w-full min-w-0 max-w-full gap-2 overflow-hidden px-5 py-3.5 text-sm font-semibold sm:w-auto sm:px-7 sm:text-base">
                  <span className="truncate">{t("landing.freeStart")}</span>
                  <ArrowRight size={18} className="transition-transform group-hover:translate-x-[var(--mcdl-hover-icon-shift)]" />
                </button>
                <Link href="/app/gallery" onClick={(event) => { event.preventDefault(); window.location.assign(event.currentTarget.href); }} className="mcdl-button-secondary mcdl-hover-control mcdl-hover-press box-border w-full min-w-0 max-w-full gap-2 overflow-hidden px-5 py-3.5 text-sm font-semibold sm:w-auto sm:px-7 sm:text-base">
                  <Compass size={18} className="shrink-0" />
                  <span className="truncate">{t("landing.gallery")}</span>
                </Link>
              </div>
              <ModelTicker />
            </div>

            <div className="relative z-30 justify-self-end overflow-visible">
              <ModelOrbit />
            </div>
          </div>

          <div className="mt-10 grid w-full max-w-4xl min-w-0 grid-cols-2 gap-3 sm:grid-cols-4 lg:mt-6 lg:max-w-[calc(100%-660px)]">
            {[
              ["20+", t("landing.stat.models")],
              ["1", t("landing.stat.wallet")],
              ["24h", t("landing.stat.api")],
              ["4", t("landing.stat.workflow")],
            ].map(([value, label]) => (
              <div key={label} className="mcdl-glass mcdl-hover-card rounded-2xl px-4 py-4">
                <div className="mcdl-numeric text-2xl font-semibold text-[var(--mcdl-text-title)]">{value}</div>
                <div className="mcdl-muted-copy mt-1 text-xs">{label}</div>
              </div>
            ))}
          </div>
        </main>
      </section>

      <section className="mcdl-marketing-section mcdl-marketing-section--subtle">
        <div className="mcdl-marketing-inner">
          <div className="mb-10 flex flex-col justify-between gap-4 md:flex-row md:items-end">
            <div>
              <div className="mcdl-eyebrow mb-4">MODEL ROUTER</div>
              <h2 className="mcdl-display text-3xl font-semibold sm:text-5xl">{t("landing.section.capability")}</h2>
            </div>
            <p className="mcdl-body-copy max-w-xl text-sm leading-7">{t("landing.section.capabilityDesc")}</p>
          </div>
          <div className="grid gap-4 md:grid-cols-4">
            {capabilityCards.map((card) => {
              const Icon = card.icon;
              return (
                <div key={card.title} className="mcdl-editorial-card mcdl-hover-card relative overflow-hidden p-6">
                  <Icon className="text-[var(--mcdl-color-accent)]" size={28} />
                  <h3 className="mt-6 text-lg font-semibold">{card.title}</h3>
                  <p className="mcdl-body-copy mt-3 text-sm leading-6">{card.desc}</p>
                </div>
              );
            })}
          </div>
        </div>
      </section>

      <section className="mcdl-marketing-section mcdl-marketing-section--base">
        <div className="mcdl-marketing-inner grid gap-12 lg:grid-cols-[0.82fr_1.18fr] lg:items-center">
          <div>
            <div className="mcdl-eyebrow mb-4">WORKFLOW</div>
            <h2 className="mcdl-display text-3xl font-semibold leading-tight sm:text-5xl">{t("landing.section.flow")}</h2>
            <p className="mcdl-body-copy mt-5 text-sm leading-7">
              {t("landing.section.flowDesc")}
            </p>
            <div className="mt-8 grid gap-3">
              {[
                ["01", t("landing.flow.step1.title"), t("landing.flow.step1.desc")],
                ["02", t("landing.flow.step2.title"), t("landing.flow.step2.desc")],
                ["03", t("landing.flow.step3.title"), t("landing.flow.step3.desc")],
              ].map(([no, title, desc]) => (
                  <div key={no} className="mcdl-editorial-card mcdl-hover-card flex gap-4 rounded-2xl p-4">
                  <div className="mcdl-numeric grid h-10 w-10 shrink-0 place-items-center rounded-xl bg-[var(--mcdl-color-accent)] text-sm font-bold text-[var(--mcdl-color-on-accent)]">{no}</div>
                  <div>
                    <div className="font-semibold text-[var(--mcdl-text-title)]">{title}</div>
                    <div className="mcdl-body-copy mt-1 text-sm">{desc}</div>
                  </div>
                </div>
              ))}
            </div>
          </div>
          <div className="mcdl-editorial-card p-3 sm:p-4">
            <div className="mcdl-stage rounded-[1.4rem] border border-[var(--mcdl-border-ghost)] p-4">
              <div className="mb-4 flex items-center justify-between pb-3">
                <div className="flex items-center gap-2">
                  <span className="h-3 w-3 rounded-full bg-red-400" />
                  <span className="h-3 w-3 rounded-full bg-amber-300" />
                  <span className="h-3 w-3 rounded-full bg-[var(--mcdl-color-accent)]" />
                </div>
                <div className="mcdl-muted-copy rounded-full border border-[var(--mcdl-border-ghost)] px-3 py-1 text-xs">{t("landing.liveWorkspace")}</div>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                {[t("landing.workspace.card1"), t("landing.workspace.card2"), t("landing.workspace.card3"), t("landing.workspace.card4")].map((item) => (
                  <div key={item} className="rounded-2xl border border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.035)] p-4">
                    <div className="mb-4 flex items-center justify-between">
                      <span className="text-sm font-semibold">{item}</span>
                      <Wand2 size={16} className="text-[var(--mcdl-color-accent)]" />
                    </div>
                    <div className="space-y-2">
                      <div className="h-2 overflow-hidden rounded-full bg-[rgba(245,239,235,.08)]"><span className="block h-full w-2/3 rounded-full bg-[var(--mcdl-color-accent)] opacity-80" /></div>
                      <div className="h-2 w-5/6 overflow-hidden rounded-full bg-[rgba(245,239,235,.08)]"><span className="block h-full w-1/2 rounded-full bg-[var(--mcdl-text-body)] opacity-50" /></div>
                      <div className="h-2 w-2/3 overflow-hidden rounded-full bg-[rgba(245,239,235,.08)]"><span className="block h-full w-3/5 rounded-full bg-[var(--mcdl-text-title)] opacity-35" /></div>
                    </div>
                  </div>
                ))}
              </div>
              <div className="mt-4 rounded-2xl border border-[rgba(255,135,17,.28)] bg-[rgba(255,135,17,.08)] p-4">
                <div className="flex items-center gap-2 text-sm font-semibold text-[var(--mcdl-color-accent)]">
                  <Play size={16} />
                  {t("landing.workspace.done")}
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section className="mcdl-marketing-section mcdl-marketing-section--subtle">
        <div className="mcdl-marketing-inner">
          <div className="mb-10 flex flex-col justify-between gap-4 md:flex-row md:items-end">
            <div>
              <div className="mcdl-eyebrow mb-4">INSPIRATION GALLERY</div>
              <h2 className="mcdl-display text-3xl font-semibold sm:text-5xl">{t("landing.section.gallery")}</h2>
            </div>
            <Link href="/app/gallery" onClick={(event) => { event.preventDefault(); window.location.assign(event.currentTarget.href); }} className="mcdl-button-secondary mcdl-hover-control mcdl-hover-press gap-2 px-5 py-2.5 text-sm font-semibold">
              {t("landing.viewAll")}
              <ArrowRight size={16} />
            </Link>
          </div>
          <div className="grid grid-cols-2 gap-3 sm:gap-4 lg:grid-cols-4">
            {galleryLoading
              ? Array.from({ length: LANDING_GALLERY_LIMIT }, (_, index) => (
                <div key={index} aria-hidden="true" className="mcdl-editorial-card overflow-hidden rounded-2xl">
                  <div className="mcdl-stage aspect-[4/5] animate-pulse rounded-b-none" />
                  <div className="space-y-3 p-3.5">
                    <div className="h-3 w-1/3 animate-pulse rounded-full bg-[rgba(245,239,235,.1)]" />
                    <div className="h-4 w-3/4 animate-pulse rounded-full bg-[rgba(245,239,235,.1)]" />
                    <div className="h-3 w-full animate-pulse rounded-full bg-[rgba(245,239,235,.07)]" />
                  </div>
                </div>
              ))
              : gallery.length > 0
                ? gallery.map((item) => <GalleryPreview key={item.id} item={item} />)
                : <div className="mcdl-body-copy col-span-full rounded-2xl border border-dashed border-[var(--mcdl-border-ghost)] bg-[rgba(245,239,235,.025)] px-5 py-12 text-center text-sm">{t("gallery.referenceLoadFailed")}</div>}
          </div>
        </div>
      </section>

      <section className="mcdl-marketing-section mcdl-marketing-section--base">
        <div className="mcdl-marketing-inner grid gap-6 md:grid-cols-3">
          <div className="mcdl-editorial-card mcdl-hover-card p-6">
            <KeyRound className="text-[var(--mcdl-color-accent)]" size={28} />
            <h3 className="mt-6 text-xl font-semibold">{t("landing.feature.apiKey.title")}</h3>
            <p className="mcdl-body-copy mt-3 text-sm leading-6">{t("landing.feature.apiKey.desc")}</p>
          </div>
          <div className="mcdl-editorial-card mcdl-hover-card p-6">
            <Copy className="text-[var(--mcdl-color-accent)]" size={28} />
            <h3 className="mt-6 text-xl font-semibold">{t("landing.feature.referral.title")}</h3>
            <p className="mcdl-body-copy mt-3 text-sm leading-6">{t("landing.feature.referral.desc")}</p>
          </div>
          <div className="mcdl-editorial-card mcdl-hover-card p-6">
            <Compass className="text-[var(--mcdl-color-accent)]" size={28} />
            <h3 className="mt-6 text-xl font-semibold">{t("landing.feature.gallery.title")}</h3>
            <p className="mcdl-body-copy mt-3 text-sm leading-6">{t("landing.feature.gallery.desc")}</p>
          </div>
        </div>

        <div className="mcdl-editorial-card relative mx-auto mt-16 max-w-4xl overflow-hidden px-6 py-12 text-center sm:px-12">
          <div className="mcdl-eyebrow mb-5">ETHEREAL VAULT</div>
          <h2 className="mcdl-display text-3xl font-semibold sm:text-5xl">{t("landing.cta")}</h2>
          <p className="mcdl-body-copy mx-auto mt-5 max-w-2xl text-sm leading-7">{t("landing.ctaDesc")}</p>
          <button onClick={enterAppOrLogin} className="mcdl-button-primary mcdl-hover-primary-action mcdl-hover-press mt-8 gap-2 px-8 py-3.5 font-semibold">
            {t("landing.tryNow")}
            <ArrowRight size={18} />
          </button>
        </div>
      </section>

      <footer className="mcdl-muted-copy relative overflow-hidden bg-[var(--mcdl-color-surface-subtle)] px-5 py-7 text-center text-xs sm:px-7">
        <div className="relative mx-auto flex max-w-[80rem] flex-col items-center justify-center gap-2 sm:flex-row sm:justify-between">
          <div className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 rounded-full bg-[var(--mcdl-color-accent)]" />
            <span>{site_name || "StarAI"}</span>
          </div>
          <div className="max-w-full break-words">{copyrightText}</div>
        </div>
      </footer>

      {branding.customer_service_mode === "custom_script" && branding.customer_service_custom_script?.trim() ? (
        <ThirdPartyCustomerServiceScript enabled={branding.customer_service_enabled} code={branding.customer_service_custom_script} />
      ) : (
        <CustomerService config={branding} />
      )}
      <LoginModal open={showLogin} onClose={() => setShowLogin(false)} />
    </div>
  );
}
