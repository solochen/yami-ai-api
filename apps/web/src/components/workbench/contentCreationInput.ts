const legacyContentCreationPlaceholders = new Set([
  "输入创作要求，或导入公众号、小红书、今日头条等平台的公开内容链接",
  "Enter requirements or import a public WeChat, Xiaohongshu, Toutiao, or other article URL",
  "制作要件を入力するか、WeChat、RED、Toutiao などの公開記事 URL を読み込みます",
  "제작 요구사항을 입력하거나 WeChat, 샤오홍슈, Toutiao 등의 공개 글 URL을 가져오세요",
  "Nhập yêu cầu sáng tạo hoặc nhập URL bài viết công khai từ WeChat, Xiaohongshu, Toutiao hay nền tảng khác",
]);

const contentCardPromptTemplates = [
  "为第 {index} 张内容卡片生成统一风格的高质量视觉底图。只表现对应信息点，预留干净的文字排版空间，不在图片中直接绘制正文、标签或水印。",
  "Generate the coordinated visual background for content card {index}. Focus on that card's idea, leave clean space for typography, and do not render body copy, hashtags, or watermarks in the image.",
  "コンテンツカード {index} 用に統一感のある高品質な背景画像を生成してください。そのカードの要点だけを表現し、文字組み用の余白を確保し、本文、タグ、透かしを画像内に描かないでください。",
  "콘텐츠 카드 {index}에 맞는 통일된 스타일의 고품질 배경 이미지를 생성하세요. 해당 카드의 핵심만 표현하고 글자 배치를 위한 여백을 남기며 본문, 태그, 워터마크를 이미지에 직접 넣지 마세요.",
  "Tạo ảnh nền chất lượng cao, đồng nhất về phong cách cho thẻ nội dung {index}. Chỉ thể hiện ý chính của thẻ, chừa khoảng trống sạch để dàn chữ và không vẽ trực tiếp nội dung, thẻ hoặc hình mờ lên ảnh.",
];

function withoutTemplateDefaults(value: unknown, defaults: Iterable<string>) {
  const prompt = String(value || "");
  const trimmed = prompt.trim();
  if (!trimmed) return "";
  for (const defaultValue of defaults) {
    const template = String(defaultValue || "").trim();
    if (!template) continue;
    if (trimmed === template) return "";
    if (trimmed.startsWith(template)) return trimmed.slice(template.length).trim();
  }
  return prompt;
}

export function contentCreationInputValue(value: unknown, additionalDefaults: Iterable<string> = []) {
  return withoutTemplateDefaults(value, [...legacyContentCreationPlaceholders, ...additionalDefaults]);
}

export function contentCreationImagePromptValue(value: unknown, index: number, additionalDefaults: Iterable<string> = []) {
  const localizedDefaults = contentCardPromptTemplates.map((template) => template.replace("{index}", String(index)));
  return withoutTemplateDefaults(value, [...localizedDefaults, ...additionalDefaults]);
}
