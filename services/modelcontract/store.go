package modelcontract

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type Queryer interface {
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
}

// LoadModels reads only capability metadata, never credentials or prices.
// to_jsonb keeps isolated legacy-schema fixtures readable as well.
func LoadModels(ctx context.Context, db Queryer) ([]Model, error) {
	rows, err := db.Query(ctx, `SELECT jsonb_build_object(
	 'id',m.id,'code',m.code,'public_alias',to_jsonb(m)->>'public_alias',
	 'display_name',COALESCE(to_jsonb(m)->>'display_name',m.code),'category',m.category,
	 'request_mode',m.request_mode,'is_enabled',m.is_enabled,
	 'runtime_rule',to_jsonb(m)->'runtime_rule','input_schema',to_jsonb(m)->'input_schema',
	 'new_api_endpoint',to_jsonb(m)->>'new_api_endpoint',
	 'routes',COALESCE((SELECT jsonb_agg(jsonb_build_object('is_enabled',r.is_enabled,
	   'runtime_rule',to_jsonb(r)->'runtime_rule','endpoint',to_jsonb(r)->>'endpoint'))
	   FROM model_routes r WHERE r.model_id=m.id),'[]'::jsonb))
	 FROM models m ORDER BY COALESCE((to_jsonb(m)->>'sort_order')::int,0),m.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Model{}
	for rows.Next() {
		var raw []byte
		var m Model
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, fmt.Errorf("模型能力配置解析失败：%w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
