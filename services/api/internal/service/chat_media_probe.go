package service

import "context"

type mediaProbeContextKey struct{}

// WithMediaProbe marks a context as an admin capability probe. A probe must
// reach the upstream even when the model or route is configured to reject
// media, because probing is exactly how a wrong configuration gets corrected.
// The marker lives in the context (never in request params) so an API caller
// cannot bypass the gate.
func WithMediaProbe(ctx context.Context) context.Context {
	return context.WithValue(ctx, mediaProbeContextKey{}, true)
}

func isMediaProbe(ctx context.Context) bool {
	flagged, _ := ctx.Value(mediaProbeContextKey{}).(bool)
	return flagged
}

// mediaGateError applies the media capability gate before a route is used.
// Probes skip it.
func mediaGateError(ctx context.Context, model *ModelFull, route *ModelRoute, messages interface{}) error {
	if isMediaProbe(ctx) {
		return nil
	}
	return chatRouteMediaError(model, route, messages)
}
