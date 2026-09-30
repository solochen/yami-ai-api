package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
)

func normalizeViralVideoBreakdownInputs(inputs map[string]interface{}) error {
	if inputs == nil {
		return errors.New("请粘贴抖音分享文本、视频链接，或上传本地视频")
	}
	videoURL := strings.TrimSpace(stringValue(inputs["video_url"]))
	share := strings.TrimSpace(firstAgentString(stringValue(inputs["share_text"]), stringValue(inputs["source_url"]), stringValue(inputs["url"])))
	if videoURL == "" && share == "" {
		return errors.New("请粘贴抖音分享文本、视频链接，或上传本地视频")
	}
	if videoURL != "" {
		if _, err := parsePublicHTTPURL(videoURL); err != nil {
			return err
		}
		inputs["video_url"] = videoURL
	}
	if share != "" {
		link, err := parsePublicHTTPURL(share)
		if err != nil {
			return err
		}
		inputs["source_url"] = link
		inputs["share_text"] = share
	}
	if videoURL == "" {
		inputs["video_url"] = inputs["source_url"]
	}
	return nil
}

func parsePublicHTTPURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if start := strings.Index(raw, "http"); start >= 0 {
		raw = raw[start:]
	}
	if end := strings.IndexAny(raw, " \t\r\n"); end >= 0 {
		raw = raw[:end]
	}
	raw = strings.Trim(raw, `"'<>，。；;）)]】`)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" {
		return "", errors.New("仅支持公开的 HTTP/HTTPS 链接")
	}
	return parsed.String(), nil
}

func (s *AgentService) SaveBreakdownRewrite(ctx context.Context, userID int64, publicID, kind, text string) error {
	switch kind {
	case "shorten", "oral", "storyboard", "english":
	default:
		return errors.New("改写类型无效")
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("改写结果为空")
	}
	patch, _ := json.Marshal(map[string]string{kind: text})
	tag, err := s.db.Exec(ctx, `
		UPDATE workflow_projects AS project
		SET outputs = jsonb_set(COALESCE(project.outputs, '{}'::jsonb), '{rewrites}', COALESCE(project.outputs->'rewrites', '{}'::jsonb) || $1::jsonb, true),
		    updated_at = now()
		FROM workflow_definitions AS definition
		WHERE project.workflow_id = definition.id
		  AND project.public_id = $2
		  AND project.user_id = $3
		  AND definition.code = 'viral_video_breakdown'
		  AND project.status = 'succeeded'`, patch, publicID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errors.New("剧本完成后才能改写")
	}
	return nil
}
