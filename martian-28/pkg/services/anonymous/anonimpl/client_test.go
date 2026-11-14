package anonimpl

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/services/anonymous"
	"github.com/grafana/grafana/pkg/services/anonymous/anonimpl/anonstore"
	"github.com/grafana/grafana/pkg/services/anonymous/anontest"
	"github.com/grafana/grafana/pkg/services/authn"
	"github.com/grafana/grafana/pkg/services/org"
	"github.com/grafana/grafana/pkg/services/org/orgtest"
	"github.com/grafana/grafana/pkg/setting"
)

func TestAnonymous_Authenticate(t *testing.T) {
	type TestCase struct {
		desc string
		org  *org.Org
		cfg  *setting.Cfg
		err  error
	}

	tests := []TestCase{
		{
			desc: "should success with valid org configured",
			org:  &org.Org{ID: 1, Name: "some org"},
			cfg: &setting.Cfg{
				AnonymousOrgName: "some org",
				AnonymousOrgRole: "Viewer",
			},
		},
		{
			desc: "should return error if any error occurs during org lookup",
			err:  fmt.Errorf("some error"),
			cfg: &setting.Cfg{
				AnonymousOrgName: "some org",
				AnonymousOrgRole: "Viewer",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			c := Anonymous{
				cfg:               tt.cfg,
				log:               log.NewNopLogger(),
				orgService:        &orgtest.FakeOrgService{ExpectedOrg: tt.org, ExpectedError: tt.err},
				anonDeviceService: anontest.NewFakeService(),
			}

			identity, err := c.Authenticate(context.Background(), &authn.Request{})
			if err != nil {
				require.Error(t, err)
				require.Nil(t, identity)
			} else {
				require.Nil(t, err)

				assert.Equal(t, authn.AnonymousNamespaceID, identity.ID)
				assert.Equal(t, tt.org.ID, identity.OrgID)
				assert.Equal(t, tt.org.Name, identity.OrgName)
				assert.Equal(t, tt.cfg.AnonymousOrgRole, string(identity.GetOrgRole()))
			}
		})
	}
}

// Embed the interface so unexpected calls fail rather than silently succeeding.
type recordingDeviceService struct {
	anonymous.Service
	request *http.Request
	err     error
}

func (s *recordingDeviceService) TagDevice(_ context.Context, r *http.Request, _ anonymous.DeviceKind) error {
	s.request = r
	return s.err
}

func TestAnonymous_AuthenticateDeviceLimit(t *testing.T) {
	for _, tc := range []struct {
		name         string
		limit        int64
		request      *http.Request
		tagError     error
		wantID       string
		wantFallback bool
	}{
		{name: "nil HTTP request", limit: 1, wantFallback: true},
		{name: "nil headers", limit: 1, request: &http.Request{RemoteAddr: "192.0.2.1:1234"}, wantFallback: true},
		{name: "empty header", limit: 1, request: &http.Request{Header: http.Header{}, RemoteAddr: "192.0.2.1:1234"}, wantFallback: true},
		{name: "explicit device", limit: 1, request: &http.Request{Header: http.Header{http.CanonicalHeaderKey(deviceIDHeader): {"device"}}}, wantID: "device"},
		{name: "unlimited", request: &http.Request{}},
		{name: "limit reached", limit: 1, request: &http.Request{}, tagError: anonstore.ErrDeviceLimitReached, wantFallback: true},
		{name: "other tagging error", limit: 1, request: &http.Request{}, tagError: fmt.Errorf("store unavailable"), wantFallback: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &recordingDeviceService{err: tc.tagError}
			client := &Anonymous{
				cfg:               &setting.Cfg{AnonymousDeviceLimit: tc.limit},
				log:               log.NewNopLogger(),
				orgService:        &orgtest.FakeOrgService{ExpectedOrg: &org.Org{ID: 1}},
				anonDeviceService: service,
			}
			identity, err := client.Authenticate(context.Background(), &authn.Request{HTTPRequest: tc.request})
			if tc.tagError == anonstore.ErrDeviceLimitReached {
				require.ErrorIs(t, err, tc.tagError)
				require.Nil(t, identity)
			} else {
				require.NoError(t, err)
				require.NotNil(t, identity)
			}
			require.NotNil(t, service.request)
			if tc.wantFallback {
				assert.Contains(t, service.request.Header.Get(deviceIDHeader), "fallback-")
			} else {
				assert.Equal(t, tc.wantID, service.request.Header.Get(deviceIDHeader))
			}
			if tc.request != nil {
				assert.Equal(t, tc.wantID, tc.request.Header.Get(deviceIDHeader), "original headers must be preserved")
			}
		})
	}
}

func TestAnonymous_AuthenticateFallbackIdentity(t *testing.T) {
	service := &recordingDeviceService{}
	client := &Anonymous{
		cfg:               &setting.Cfg{AnonymousDeviceLimit: 1},
		log:               log.NewNopLogger(),
		orgService:        &orgtest.FakeOrgService{ExpectedOrg: &org.Org{ID: 1}},
		anonDeviceService: service,
	}
	tag := func(addr, agent string) string {
		_, err := client.Authenticate(context.Background(), &authn.Request{HTTPRequest: &http.Request{
			RemoteAddr: addr, Header: http.Header{"User-Agent": {agent}},
		}})
		require.NoError(t, err)
		return service.request.Header.Get(deviceIDHeader)
	}
	id := tag("192.0.2.1:1234", "browser")
	assert.Equal(t, id, tag("192.0.2.1:5678", "browser"))
	assert.NotEqual(t, id, tag("192.0.2.2:1234", "browser"))
	assert.NotEqual(t, id, tag("192.0.2.1:1234", "other browser"))
}
