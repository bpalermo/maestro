package sidecar

import (
	"context"
	"errors"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	discoveryv3 "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
)

func (s *XdsSidecar) OnFetchRequest(_ context.Context, _ *discoveryv3.DiscoveryRequest) error {
	return errors.New("unimplemented")
}

func (s *XdsSidecar) OnFetchResponse(_ *discoveryv3.DiscoveryRequest, _ *discoveryv3.DiscoveryResponse) {
}

func (s *XdsSidecar) OnStreamOpen(_ context.Context, _ int64, _ string) error {
	return errors.New("unimplemented")
}

func (s *XdsSidecar) OnStreamClosed(_ int64, _ *corev3.Node) {
}

func (s *XdsSidecar) OnStreamRequest(_ int64, _ *discoveryv3.DiscoveryRequest) error {
	return errors.New("unimplemented")
}

func (s *XdsSidecar) OnStreamResponse(_ context.Context, _ int64, _ *discoveryv3.DiscoveryRequest, _ *discoveryv3.DiscoveryResponse) {
}

func (s *XdsSidecar) OnDeltaStreamOpen(_ context.Context, _ int64, _ string) error {
	return nil
}

func (s *XdsSidecar) OnDeltaStreamClosed(_ int64, _ *corev3.Node) {
}

func (s *XdsSidecar) OnStreamDeltaRequest(_ int64, _ *discoveryv3.DeltaDiscoveryRequest) error {
	return nil
}

func (s *XdsSidecar) OnStreamDeltaResponse(_ int64, _ *discoveryv3.DeltaDiscoveryRequest, _ *discoveryv3.DeltaDiscoveryResponse) {
}
