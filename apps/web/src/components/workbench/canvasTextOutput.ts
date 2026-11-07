const clarificationOpeners = [
  /^(?:(?:为了|如需|若要)[^。！？\n]{0,60}[，,]?\s*)?(?:请|麻烦(?:您)?|还需要(?:您)?|我(?:还)?需要(?:您)?)\s*(?:补充|提供|明确|说明|告知|告诉我|确认|选择|指定)/u,
  /^(?:could you|please)\s+(?:provide|clarify|specify|tell me|confirm|choose|select)\b/i,
  /^(?:i(?: still)? need you to|i(?: still)? need)\s+(?:provide|clarify|specify|tell me|confirm|choose|select)\b/i,
];

export function canvasTextIsClarification(text: string) {
  const value = String(text || "").trim().replace(/^[\s`"'“”‘’]+/u, "");
  return clarificationOpeners.some((pattern) => pattern.test(value));
}
