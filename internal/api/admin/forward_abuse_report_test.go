package admin_test

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForwardAbuseUserReport(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	// reportId 欠落は 204 (forwarder 未配線, abuseRepo 未配線 → no-op)
	assert.Equal(t, http.StatusNoContent, doPost(h.ForwardAbuseUserReport, `{}`, adminUser).Code)
}

type stubAbuseForwarder struct {
	calledWith string
	err        error
}

func (s *stubAbuseForwarder) ForwardReport(reportID string) error {
	s.calledWith = reportID
	return s.err
}

func TestForwardAbuseUserReport_UsesForwarderWhenWired(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	stub := &stubAbuseForwarder{}
	h.SetAbuseForwarder(stub)
	rec := doPost(h.ForwardAbuseUserReport, `{"reportId":"r1"}`, adminUser)
	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "r1", stub.calledWith)
}

func TestForwardAbuseUserReport_ForwarderError(t *testing.T) {
	h, _, _, _ := newTestHandler(t)
	h.SetAbuseForwarder(&stubAbuseForwarder{err: assertError{}})
	rec := doPost(h.ForwardAbuseUserReport, `{"reportId":"r1"}`, adminUser)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}
