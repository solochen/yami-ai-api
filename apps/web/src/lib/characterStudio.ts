import { API_URL, api, uploadAsset, uploadFile } from "./api";

export type CharacterStudioConfig = {
  sale_price_per_minute: number;
  max_minutes: number;
  default_resolution: string;
  resolutions: string[];
  max_characters: number;
  source_url?: string;
  default_role_base_url?: string;
};

export type CharacterStudioCharacter = {
  public_id: string;
  name: string;
  image_url: string;
  created_at: string;
  updated_at: string;
};

export type CharacterStudioChannel = {
  app_id: string;
  channel_id: string;
  user_id: string;
  token: string;
  token_expire_at: string;
};

export type CharacterStudioTake = {
  public_id: string;
  url: string;
  asset_public_id?: string;
  duration_ms: number;
  size_bytes: number;
  created_at: string;
};

export type CharacterStudioSession = {
  public_id: string;
  status: "starting" | "live" | "reconnecting" | "wrapping" | "ended" | "failed";
  resolution: string;
  character?: CharacterStudioCharacter;
  channel?: CharacterStudioChannel;
  output_channel?: string;
  started_at?: string;
  elapsed_seconds: number;
  max_seconds: number;
  sale_price_per_minute: number;
  switch_count: number;
  billed_seconds: number;
  cost: number;
  error?: string;
  end_reason?: string;
  can_resume?: boolean;
  takes?: CharacterStudioTake[];
  created_at: string;
};

export type CharacterStudioDraft = {
  name: string;
  image_url: string;
  rights_confirmed?: boolean;
};

export type CharacterStudioTakeDraft = {
  url: string;
  asset_public_id?: string;
  duration_ms: number;
  size_bytes: number;
};

const base = "/api/character-studio";

export async function fetchCharacterStudioConfig(): Promise<CharacterStudioConfig> {
  return api<CharacterStudioConfig>(`${base}/config`);
}

export async function listCharacterStudioCharacters(): Promise<CharacterStudioCharacter[]> {
  return api<CharacterStudioCharacter[]>(`${base}/characters`);
}

export async function createCharacterStudioCharacter(draft: CharacterStudioDraft): Promise<CharacterStudioCharacter> {
  return api<CharacterStudioCharacter>(`${base}/characters`, {
    method: "POST",
    body: JSON.stringify(draft),
  });
}

export async function updateCharacterStudioCharacter(publicId: string, draft: CharacterStudioDraft): Promise<CharacterStudioCharacter> {
  return api<CharacterStudioCharacter>(`${base}/characters/${encodeURIComponent(publicId)}`, {
    method: "PUT",
    body: JSON.stringify(draft),
  });
}

export async function deleteCharacterStudioCharacter(publicId: string): Promise<void> {
  await api(`${base}/characters/${encodeURIComponent(publicId)}`, { method: "DELETE" });
}

export async function startCharacterStudioSession(characterId: string, resolution: string): Promise<CharacterStudioSession> {
  return api<CharacterStudioSession>(`${base}/sessions`, {
    method: "POST",
    body: JSON.stringify({ character_id: characterId, resolution }),
  });
}

export async function getCharacterStudioSession(publicId: string): Promise<CharacterStudioSession> {
  return api<CharacterStudioSession>(`${base}/sessions/${encodeURIComponent(publicId)}`);
}

export async function getActiveCharacterStudioSession(): Promise<CharacterStudioSession | null> {
  const result = await api<{ session: CharacterStudioSession | null }>(`${base}/sessions/active`);
  return result?.session || null;
}

export async function switchCharacterStudioCharacter(sessionId: string, characterId: string): Promise<{ character: CharacterStudioCharacter }> {
  return api<{ character: CharacterStudioCharacter }>(`${base}/sessions/${encodeURIComponent(sessionId)}/switch`, {
    method: "POST",
    body: JSON.stringify({ character_id: characterId }),
  });
}

export async function heartbeatCharacterStudioSession(sessionId: string): Promise<void> {
  await api(`${base}/sessions/${encodeURIComponent(sessionId)}/heartbeat`, { method: "POST" });
}

export async function endCharacterStudioSession(sessionId: string): Promise<CharacterStudioSession> {
  return api<CharacterStudioSession>(`${base}/sessions/${encodeURIComponent(sessionId)}/end`, { method: "POST" });
}

export async function addCharacterStudioTake(sessionId: string, draft: CharacterStudioTakeDraft): Promise<CharacterStudioTake> {
  return api<CharacterStudioTake>(`${base}/sessions/${encodeURIComponent(sessionId)}/takes`, {
    method: "POST",
    body: JSON.stringify(draft),
  });
}

export async function uploadCharacterStudioImage(file: File): Promise<string> {
  return uploadFile(file);
}

export async function uploadCharacterStudioTake(file: File): Promise<{ public_id: string; url: string }> {
  return uploadAsset(file, { kind: "video", name: file.name });
}

/** 浏览器运行时由本站代理，页面源码不出现第三方 CDN 域名。 */
export function characterStudioRuntimeURL(): string {
  return `${API_URL}/api/realtime/engine.js`;
}
