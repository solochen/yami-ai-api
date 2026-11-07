type VideoAnalysisModel = {
  code: string;
  display_name?: string;
  tags?: string[];
  runtime_rule?: Record<string, unknown>;
};

export function supportsVideoAnalysis(model?: VideoAnalysisModel) {
  return supportsMediaAnalysis(model, "video");
}

export type AnalysisMediaKind = "image" | "video" | "audio";

export function supportsMediaAnalysis(model: VideoAnalysisModel | undefined, kind: AnalysisMediaKind) {
  if (!model) return false;
  const capabilities = (model.runtime_rule?.capabilities || {}) as Record<string, unknown>;
  // Match the admin checkboxes and API media validation: canonical switches win,
  // including false; legacy aliases only apply when the switch is absent.
  const keys = kind === "image"
    ? ["vision", "image_input", "multimodal"]
    : [`${kind}_analysis`, `${kind}_input`, `${kind}_understanding`];
  for (const key of keys) {
    if (typeof capabilities[key] === "boolean") return capabilities[key] === true;
  }
  return false;
}

export function canvasAnalysisMediaKinds(nodes: { data: Record<string, unknown> }[]): AnalysisMediaKind[] {
  return (["image", "video", "audio"] as const).filter(kind => nodes.some(({ data }) => {
    const refs = data[{ image: "referenceImageUrls", video: "referenceVideoUrls", audio: "referenceAudioUrls" }[kind]];
    return (Array.isArray(refs) && refs.some(Boolean))
      || (data.mediaKind === kind && (Boolean(data.assetUrl) || Array.isArray(data.assetUrls) && data.assetUrls.some(Boolean)))
      || (data.outputKind === kind && Boolean(data.outputUrl));
  }));
}

// Models configured without an input_schema still declare their supported
// aspect ratios in runtime_rule.image/video.supported_ratios (the admin form
// writes them there). Use that list when the schema exposes no ratio control,
// otherwise keyframes silently fall back to the model default (often 1:1) even
// though the node asks for 9:16.
export function supportsRuntimeAspectRatio(
  model: { category?: string; runtime_rule?: Record<string, unknown> } | undefined,
  ratio: string,
) {
  const mediaRule = (model?.runtime_rule?.[model.category === "video" ? "video" : "image"] || {}) as Record<string, unknown>;
  const supported = Array.isArray(mediaRule.supported_ratios) ? mediaRule.supported_ratios.map((item) => String(item).replace(/\s/g, "")) : [];
  return supported.includes(ratio.replace(/\s/g, ""));
}
