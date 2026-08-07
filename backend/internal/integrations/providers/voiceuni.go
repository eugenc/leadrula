package providers

import (
	"context"
	"fmt"
)

type VoiceUniProvider struct{}

func (p *VoiceUniProvider) Slug() string { return "voiceuni" }

func (p *VoiceUniProvider) Deliver(_ context.Context, _ []byte, _ DeliveryPayload) (*DeliveryResult, error) {
	return nil, fmt.Errorf("voiceuni outbound delivery is not supported")
}

func (p *VoiceUniProvider) ValidateCredentials(_ context.Context, _ []byte, _ map[string]any) error {
	return nil
}
