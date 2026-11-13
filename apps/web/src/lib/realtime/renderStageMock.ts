import type { RenderStage, RenderStageOptions } from "./renderStage";

/**
 * 本地演练用渲染台：只在开发环境通过 ?rehearse=1 动态加载。
 * 它把摄像头画面做一层风格化处理，模拟合成画面的观感；
 * 不连接任何实时通道，也不会产生计费。生产构建不包含入口。
 */

let characterImage: HTMLImageElement | null = null;

export function setMockCharacter(imageURL?: string) {
  characterImage = null;
  if (!imageURL || typeof window === "undefined") return;
  const image = new Image();
  image.crossOrigin = "anonymous";
  image.src = imageURL;
  image.onload = () => {
    characterImage = image;
  };
}

export async function createMockRenderStage(): Promise<RenderStage> {
  let frameHandle = 0;
  let outputTimer: number | null = null;
  let options: RenderStageOptions | null = null;
  let stream: MediaStream | null = null;

  const stop = () => {
    if (frameHandle) window.cancelAnimationFrame(frameHandle);
    frameHandle = 0;
    if (outputTimer !== null) {
      window.clearTimeout(outputTimer);
      outputTimer = null;
    }
    stream?.getTracks().forEach((track) => track.stop());
    stream = null;
    if (options?.remoteVideo) options.remoteVideo.srcObject = null;
    options = null;
  };

  return {
    async join(next: RenderStageOptions) {
      options = next;
      const source = next.localVideo.srcObject;
      if (!(source instanceof MediaStream)) {
        throw new Error("本地演练需要先开启摄像头预览");
      }
      const canvas = document.createElement("canvas");
      const width = next.localVideo.videoWidth || 1280;
      const height = next.localVideo.videoHeight || 720;
      canvas.width = width;
      canvas.height = height;
      const context = canvas.getContext("2d");
      if (!context) throw new Error("当前浏览器不支持画布预览");

      const draw = () => {
        if (!options) return;
        if (next.localVideo.videoWidth) {
          canvas.width = next.localVideo.videoWidth;
          canvas.height = next.localVideo.videoHeight;
        }
        context.save();
        context.filter = "contrast(1.08) saturate(1.05) sepia(0.22) hue-rotate(-8deg)";
        context.drawImage(next.localVideo, 0, 0, canvas.width, canvas.height);
        context.restore();

        const warm = context.createLinearGradient(0, 0, canvas.width, canvas.height);
        warm.addColorStop(0, "rgba(255, 135, 17, 0.10)");
        warm.addColorStop(0.55, "rgba(255, 135, 17, 0.02)");
        warm.addColorStop(1, "rgba(24, 18, 14, 0.28)");
        context.fillStyle = warm;
        context.fillRect(0, 0, canvas.width, canvas.height);

        if (characterImage?.complete && characterImage.naturalWidth) {
          const targetHeight = canvas.height * 0.34;
          const targetWidth = (characterImage.naturalWidth / characterImage.naturalHeight) * targetHeight;
          context.save();
          context.globalAlpha = 0.16;
          context.drawImage(characterImage, canvas.width - targetWidth - 24, canvas.height - targetHeight - 24, targetWidth, targetHeight);
          context.restore();
        }

        const vignette = context.createRadialGradient(
          canvas.width / 2,
          canvas.height / 2,
          canvas.width * 0.2,
          canvas.width / 2,
          canvas.height / 2,
          canvas.width * 0.72,
        );
        vignette.addColorStop(0, "rgba(0,0,0,0)");
        vignette.addColorStop(1, "rgba(0,0,0,0.34)");
        context.fillStyle = vignette;
        context.fillRect(0, 0, canvas.width, canvas.height);

        frameHandle = window.requestAnimationFrame(draw);
      };
      draw();

      stream = canvas.captureStream(30);
      next.remoteVideo.srcObject = stream;
      void next.remoteVideo.play().catch(() => undefined);
      next.remoteVideo.addEventListener("playing", () => next.onFirstFrame?.(), { once: true });
      outputTimer = window.setTimeout(() => {
        next.onRemoteOnline?.();
        next.onFirstFrame?.();
      }, 900);
    },
    async switchCamera() {
      /* 演练模式不切换物理摄像头 */
    },
    async leave() {
      const current = options;
      stop();
      current?.onRemoteOffline?.();
    },
  };
}
