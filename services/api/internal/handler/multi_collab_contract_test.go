package handler

import (
	"errors"
	"fmt"
	"testing"

	"github.com/starai/api/internal/service"
)

func TestNormalizeCollabParamsPromotesTopLevelChannelKey(t *testing.T) {
	input := service.CompletionInput{ChannelKey: " price_first "}
	got := normalizeCollabParams(input)
	if got.Params["channel_key"] != "price_first" {
		t.Fatalf("channel_key = %#v, want price_first", got.Params["channel_key"])
	}
}

func TestNormalizeCollabParamsKeepsExplicitParamsValue(t *testing.T) {
	input := service.CompletionInput{
		ChannelKey: "speed_first",
		Params:     map[string]interface{}{"channel_key": "price_first"},
	}
	got := normalizeCollabParams(input)
	if got.Params["channel_key"] != "price_first" {
		t.Fatalf("params.channel_key = %#v, want price_first", got.Params["channel_key"])
	}
}

func TestNormalizeCollabParamsWithoutChannelKeyLeavesParamsAlone(t *testing.T) {
	input := service.CompletionInput{}
	got := normalizeCollabParams(input)
	if got.Params != nil {
		t.Fatalf("params = %#v, want nil", got.Params)
	}
}

func TestSummaryErrorMessageDistinguishesCallFromSettlement(t *testing.T) {
	callErr := fmt.Errorf("%w: %v", errSummaryModelCall, errors.New("upstream 502"))
	if got := summaryErrorMessage(callErr); got == "" || got == summaryErrorMessage(errors.New("settlement")) {
		t.Fatalf("call failure and settlement failure must read differently, got %q", got)
	}
	settleErr := errors.New("settlement items do not match")
	if got := summaryErrorMessage(settleErr); got == summaryErrorMessage(callErr) {
		t.Fatalf("settlement failure must not reuse the call-failure message")
	}
}
