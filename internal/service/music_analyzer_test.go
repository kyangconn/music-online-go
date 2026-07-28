package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kyangconn/music-online-go/internal/config"
)

func TestHTTPAudioAnalyzerStreamsAuthenticatedVersionedRequest(t *testing.T) {
	audio := []byte("fixture audio bytes")
	var received atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost || request.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 32) {
			t.Errorf("unexpected analyzer request method/auth")
		}
		if request.Header.Get("X-Music-Online-Music-ID") != "7" ||
			request.Header.Get("X-Music-Online-File-Hash") != strings.Repeat("a", 64) ||
			request.Header.Get("X-Music-Online-Content-Revision") != "3" ||
			request.Header.Get("X-Music-Online-Max-Duration-Ms") != "60000" {
			t.Errorf("analyzer request headers = %#v", request.Header)
		}
		body, err := io.ReadAll(request.Body)
		if err != nil || string(body) != string(audio) {
			t.Errorf("analyzer body = %q, err=%v", body, err)
		}
		received.Store(true)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"analyzer_id":"fixture","analyzer_version":"1.0","model_version":"m1","duration_ms":1500,"features":{"bpm":128,"bpm_candidates":[64,128],"instrumental":true},"model_labels":{"trance":0.8}}`)
	}))
	defer server.Close()

	cfg := testAnalyzerConfig(server.URL)
	analyzer := newHTTPAudioAnalyzer(cfg)
	result, err := analyzer.Analyze(context.Background(), audioAnalyzerInput{
		MusicID: 7, FileHash: strings.Repeat("a", 64), ContentRevision: 3,
		MaxDuration: time.Minute, ContentLength: int64(len(audio)), Audio: strings.NewReader(string(audio)),
	})
	if err != nil {
		t.Fatalf("analyze fixture: %v", err)
	}
	if !received.Load() || result.DurationMS != 1500 || result.Features["bpm"] != float64(128) || result.ModelLabels["trance"] != 0.8 {
		t.Fatalf("unexpected analyzer result: %+v", result)
	}
}

func TestHTTPAudioAnalyzerDoesNotInheritProxy(t *testing.T) {
	analyzer, ok := newHTTPAudioAnalyzer(testAnalyzerConfig("http://analyzer.internal/v1/analyze")).(*httpAudioAnalyzer)
	if !ok {
		t.Fatal("default analyzer has an unexpected implementation")
	}
	transport, ok := analyzer.client.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatalf("analyzer transport must disable inherited proxies: %#v", analyzer.client.Transport)
	}
}

func TestHTTPAudioAnalyzerDoesNotForwardCredentialOnRedirect(t *testing.T) {
	var targetCalls atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetCalls.Add(1)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		http.Redirect(writer, request, target.URL, http.StatusTemporaryRedirect)
	}))
	defer source.Close()

	analyzer := newHTTPAudioAnalyzer(testAnalyzerConfig(source.URL))
	_, err := analyzer.Analyze(context.Background(), audioAnalyzerInput{
		MusicID: 1, FileHash: strings.Repeat("a", 64), ContentRevision: 1,
		MaxDuration: time.Minute, ContentLength: 1, Audio: strings.NewReader("x"),
	})
	if err == nil {
		t.Fatal("redirect response should not be accepted")
	}
	if targetCalls.Load() != 0 {
		t.Fatal("analyzer redirect target was called; bearer token could have leaked")
	}
}

func TestHTTPAudioAnalyzerRejectsInvalidBoundedOutput(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = io.Copy(io.Discard, request.Body)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(writer, `{"analyzer_id":"fixture","analyzer_version":"1.0","model_version":"m1","duration_ms":1500,"features":{},"model_labels":{"trance":1.2}}`)
	}))
	defer server.Close()
	analyzer := newHTTPAudioAnalyzer(testAnalyzerConfig(server.URL))
	_, err := analyzer.Analyze(context.Background(), audioAnalyzerInput{
		MusicID: 1, FileHash: strings.Repeat("a", 64), ContentRevision: 1,
		MaxDuration: time.Minute, ContentLength: 1, Audio: strings.NewReader("x"),
	})
	var failure *analyzerFailure
	if !errors.As(err, &failure) || failure.code != "analyzer_labels_invalid" || failure.retryable {
		t.Fatalf("invalid labels error = %#v", err)
	}
}

func testAnalyzerConfig(endpoint string) config.AnalyzerConfig {
	return config.AnalyzerConfig{
		Mode: "http", Endpoint: endpoint, Token: strings.Repeat("t", 32),
		ID: "fixture", Version: "1.0", ModelVersion: "m1", TimeoutSeconds: 5,
	}
}
