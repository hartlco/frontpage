package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fakeSyncer struct {
	err    error
	called int
}

func (s *fakeSyncer) Sync() error {
	s.called++
	return s.err
}

func TestSyncHandlerRedirectsOnSuccess(t *testing.T) {
	syncer := &fakeSyncer{}
	handler := NewSyncHandler(syncer)
	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if syncer.called != 1 {
		t.Fatalf("Sync called %d times, want 1", syncer.called)
	}
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/?refresh=success" {
		t.Fatalf("Location = %q, want %q", location, "/?refresh=success")
	}
}

func TestSyncHandlerRedirectsOnError(t *testing.T) {
	syncer := &fakeSyncer{err: errors.New("pull failed")}
	handler := NewSyncHandler(syncer)
	req := httptest.NewRequest(http.MethodPost, "/sync", nil)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, req)

	if syncer.called != 1 {
		t.Fatalf("Sync called %d times, want 1", syncer.called)
	}
	if recorder.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusSeeOther)
	}
	if location := recorder.Header().Get("Location"); location != "/?refresh=error" {
		t.Fatalf("Location = %q, want %q", location, "/?refresh=error")
	}
}
