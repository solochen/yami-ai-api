export type CharacterStudioDefaultCategory = "men" | "women" | "animals" | "cartoon" | "ancient";

export type CharacterStudioDefaultRole = {
  key: string;
  name: string;
  category: CharacterStudioDefaultCategory;
  fileName: string;
  src: string;
};

/** 平台默认角色：白底、单人、正面半身，便于直接做换角测试。 */
export const CHARACTER_STUDIO_DEFAULT_ROLES: CharacterStudioDefaultRole[] = [
  { key: "handsome-1", name: "顾言", category: "men", fileName: "handsome-1.jpg", src: "/character-studio/defaults/handsome-1.jpg" },
  { key: "handsome-2", name: "亚历克斯", category: "men", fileName: "handsome-2.jpg", src: "/character-studio/defaults/handsome-2.jpg" },
  { key: "beauty-1", name: "苏晚", category: "women", fileName: "beauty-1.jpg", src: "/character-studio/defaults/beauty-1.jpg" },
  { key: "beauty-2", name: "艾琳", category: "women", fileName: "beauty-2.jpg", src: "/character-studio/defaults/beauty-2.jpg" },
  { key: "cat", name: "橘座", category: "animals", fileName: "cat.jpg", src: "/character-studio/defaults/cat.jpg" },
  { key: "dog", name: "金豆", category: "animals", fileName: "dog.jpg", src: "/character-studio/defaults/dog.jpg" },
  { key: "toon-1", name: "阿蓝", category: "cartoon", fileName: "toon-1.jpg", src: "/character-studio/defaults/toon-1.jpg" },
  { key: "toon-2", name: "小满", category: "cartoon", fileName: "toon-2.jpg", src: "/character-studio/defaults/toon-2.jpg" },
  { key: "ancient-1", name: "昭华", category: "ancient", fileName: "ancient-1.jpg", src: "/character-studio/defaults/ancient-1.jpg" },
  { key: "ancient-2", name: "苏念", category: "ancient", fileName: "ancient-2.jpg", src: "/character-studio/defaults/ancient-2.jpg" },
];

export const CHARACTER_STUDIO_DEFAULT_CATEGORY_LABELS: Record<CharacterStudioDefaultCategory, string> = {
  men: "型男",
  women: "美女",
  animals: "动物",
  cartoon: "卡通",
  ancient: "古风",
};
