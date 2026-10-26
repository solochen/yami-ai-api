package main

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestBuildTencentAudioPayloadSpeech(t *testing.T) {
	input := map[string]interface{}{
		"voice_id":      "male-qn-qingse",
		"model_version": "speech-2.8-hd",
		"speed":         1.2,
		"vol":           1.0,
		"pitch":         0,
		"emotion":       "happy",
		"format":        "mp3",
		"sample_rate":   32000,
		"bitrate":       128000,
		"channel":       1,
		"output_format": "hex",
	}
	payload := buildTencentAudioPayload("audio_minimax_speech_28_hd", "minimax-speech-2.8-hd", "你好世界", input)

	if payload["model"] != "speech-2.8-hd" {
		t.Fatalf("model = %v, 应优先用 model_version", payload["model"])
	}
	if payload["text"] != "你好世界" {
		t.Fatalf("text = %v", payload["text"])
	}
	voice, _ := payload["voice_setting"].(map[string]interface{})
	if voice["voice_id"] != "male-qn-qingse" || voice["emotion"] != "happy" {
		t.Fatalf("voice_setting = %#v", voice)
	}
	audio, _ := payload["audio_setting"].(map[string]interface{})
	if intAny(audio["sample_rate"]) != 32000 || audio["format"] != "mp3" {
		t.Fatalf("audio_setting = %#v", audio)
	}
	if payload["output_format"] != "hex" || payload["stream"] != false {
		t.Fatalf("output_format/stream = %v/%v", payload["output_format"], payload["stream"])
	}
	// emotion=auto 不应下发
	payloadAuto := buildTencentAudioPayload("audio_minimax_speech_28_hd", "minimax-speech-2.8-hd", "hi",
		map[string]interface{}{"emotion": "auto", "voice_id": "v1"})
	voiceAuto, _ := payloadAuto["voice_setting"].(map[string]interface{})
	if _, exists := voiceAuto["emotion"]; exists {
		t.Fatalf("emotion=auto 不应下发: %#v", voiceAuto)
	}
}

func TestBuildTencentAudioPayloadMusic(t *testing.T) {
	input := map[string]interface{}{
		"model_version":    "music-3.0",
		"is_instrumental":  true,
		"lyrics_optimizer": true,
		"format":           "mp3",
		"output_format":    "hex",
	}
	payload := buildTencentAudioPayload("audio_minimax_music_26", "minimax-music-v3.0", "轻快的夏日午后", input)

	if payload["model"] != "music-3.0" || payload["prompt"] != "轻快的夏日午后" {
		t.Fatalf("music payload = %#v", payload)
	}
	if payload["is_instrumental"] != true || payload["lyrics_optimizer"] != true {
		t.Fatalf("music flags = %#v", payload)
	}
	if _, exists := payload["text"]; exists {
		t.Fatal("音乐请求不应带 text 字段")
	}
	// 带歌词时应下发 lyrics
	withLyrics := buildTencentAudioPayload("audio_minimax_music_26", "minimax-music-v3.0", "夏日",
		map[string]interface{}{"is_instrumental": false, "lyrics": "阳光洒在窗台"})
	if withLyrics["lyrics"] != "阳光洒在窗台" {
		t.Fatalf("lyrics = %v", withLyrics["lyrics"])
	}
}

func TestNormalizeTencentAudioResponse(t *testing.T) {
	pcm := []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}
	payload := map[string]interface{}{"audio_setting": map[string]interface{}{"format": "mp3"}}

	// MiniMax hex（output_format=hex）：hex 串本身也是合法 base64，必须按 hex 解
	hexBody, _ := json.Marshal(map[string]interface{}{"data": map[string]interface{}{"audio": hex.EncodeToString(pcm), "status": 2}})
	out, err := normalizeTencentAudioResponse(hexBody, "application/json", payload)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Data []struct {
			B64JSON   string `json:"b64_json"`
			MediaType string `json:"media_type"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Data) != 1 {
		t.Fatalf("data = %#v", parsed.Data)
	}
	decoded, err := base64.StdEncoding.DecodeString(parsed.Data[0].B64JSON)
	if err != nil || string(decoded) != string(pcm) {
		t.Fatalf("hex 解码结果不对: %v %v", err, decoded)
	}
	if parsed.Data[0].MediaType != "audio/mpeg" {
		t.Fatalf("media_type = %v", parsed.Data[0].MediaType)
	}

	// output_format=url
	urlBody, _ := json.Marshal(map[string]interface{}{"data": map[string]interface{}{"audio": "https://example.com/a.mp3"}})
	out, err = normalizeTencentAudioResponse(urlBody, "application/json", payload)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSON(out, "https://example.com/a.mp3") {
		t.Fatalf("url 未被保留: %s", out)
	}

	// OpenAI 风格 audio.data
	oaiBody, _ := json.Marshal(map[string]interface{}{"audio": map[string]interface{}{"data": base64.StdEncoding.EncodeToString(pcm)}})
	out, err = normalizeTencentAudioResponse(oaiBody, "application/json", payload)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSON(out, base64.StdEncoding.EncodeToString(pcm)) {
		t.Fatalf("OpenAI 风格未被规范化: %s", out)
	}

	// 裸二进制音频
	out, err = normalizeTencentAudioResponse(pcm, "audio/mpeg", payload)
	if err != nil {
		t.Fatal(err)
	}
	if !containsJSON(out, base64.StdEncoding.EncodeToString(pcm)) {
		t.Fatalf("裸二进制未被规范化: %s", out)
	}
}

func containsJSON(body []byte, needle string) bool {
	return len(needle) > 0 && len(body) > 0 && json.Valid(body) && stringContains(string(body), needle)
}

func stringContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(haystack, needle string) int {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}
