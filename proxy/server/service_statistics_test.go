package server

import (
	"testing"

	"github.com/ivpn/dns/proxy/collector/channel"
	"github.com/ivpn/dns/proxy/mocks"
	"github.com/ivpn/dns/proxy/model"
	"github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// specRef: proxy-statistics-behaviour.md #Y2 #Y3 #Y4
func TestEmitServiceStatistics_CountersOnly(t *testing.T) {
	tests := []struct {
		name   string
		status model.Status
		nilRes bool
		adFlag bool
		want   model.Queries
	}{
		{
			name:   "processed query without DNSSEC",
			status: model.StatusProcessed,
			want:   model.Queries{Total: 1},
		},
		{
			name:   "blocked query counts as blocked",
			status: model.StatusBlocked,
			want:   model.Queries{Total: 1, Blocked: 1},
		},
		{
			name:   "authenticated response counts as dnssec",
			status: model.StatusProcessed,
			adFlag: true,
			want:   model.Queries{Total: 1, DNSSEC: 1},
		},
		{
			name:   "unavailable result is neither blocked nor dnssec",
			status: model.StatusUnavailable,
			nilRes: true,
			want:   model.Queries{Total: 1},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			statsCh := mocks.NewCollectorChannel(t)
			s := newPostResolveServer(t, mocks.NewFilter(t), mocks.NewCache(t), map[string]channel.CollectorChannel{
				model.TYPE_STATISTICS: statsCh,
			})
			reqCtx := newPostResolveReqCtx(tt.status, nil)
			dctx := newPostResolveDNSContext("example.com", dns.TypeA)
			if tt.nilRes {
				dctx.Res = nil
			} else {
				dctx.Res.AuthenticatedData = tt.adFlag
			}

			var got model.EventStatistics
			statsCh.On("Send", mock.MatchedBy(func(data any) bool {
				evt, ok := data.(model.EventStatistics)
				if ok {
					got = evt
				}
				return ok
			})).Return(nil).Once()

			s.EmitServiceStatistics(reqCtx, dctx)

			require.Equal(t, model.EventStatistics{Queries: tt.want}, got)
		})
	}
}

// specRef: proxy-statistics-behaviour.md #Y2
func TestEmitServiceStatistics_SendErrorIsLoggedNotRaised(t *testing.T) {
	statsCh := mocks.NewCollectorChannel(t)
	statsCh.On("Send", mock.Anything).Return(assert.AnError).Once()
	s := newPostResolveServer(t, mocks.NewFilter(t), mocks.NewCache(t), map[string]channel.CollectorChannel{
		model.TYPE_STATISTICS: statsCh,
	})

	assert.NotPanics(t, func() {
		s.EmitServiceStatistics(newPostResolveReqCtx(model.StatusProcessed, nil), newPostResolveDNSContext("example.com", dns.TypeA))
	})
}
