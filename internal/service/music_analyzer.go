package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kyangconn/music-online-go/internal/config"
)

const maxAnalyzerResponseBytes = 1 << 20

type audioAnalyzerInput struct {
	MusicID         uint
	FileHash        string
	ContentRevision uint64
	MaxDuration     time.Duration
	ContentLength   int64
	Audio           io.Reader
}

type audioAnalyzerResult struct {
	AnalyzerID      string
	AnalyzerVersion string
	ModelVersion    string
	DurationMS      int64
	Features        map[string]any
	ModelLabels     map[string]float64
}

type audioAnalyzer interface {
	Analyze(ctx context.Context, input audioAnalyzerInput) (*audioAnalyzerResult, error)
}

type analyzerFailure struct {
	code      string
	retryable bool
	summary   string
	cause     error
}

func (failure *analyzerFailure) Error() string {
	if failure.cause != nil {
		return failure.summary + ": " + failure.cause.Error()
	}
	return failure.summary
}

func (failure *analyzerFailure) Unwrap() error { return failure.cause }

func newAnalyzerFailure(code string, retryable bool, summary string, cause error) error {
	return &analyzerFailure{code: code, retryable: retryable, summary: summary, cause: cause}
}

type httpAudioAnalyzer struct {
	config config.AnalyzerConfig
	client *http.Client
}

func newHTTPAudioAnalyzer(cfg config.AnalyzerConfig) audioAnalyzer {
	return newHTTPAudioAnalyzerWithClient(cfg, nil)
}

func newHTTPAudioAnalyzerWithClient(cfg config.AnalyzerConfig, client *http.Client) audioAnalyzer {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		// Analyzer traffic carries raw audio and a bearer secret. It is designed
		// for an explicitly configured local/container endpoint, so never route
		// it through inherited HTTP_PROXY settings.
		transport.Proxy = nil
		client = &http.Client{
			Timeout:   time.Duration(cfg.TimeoutSeconds) * time.Second,
			Transport: transport,
			// Never forward the bearer credential to a redirect target. The
			// administrator must configure the final analyzer endpoint directly.
			CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	return &httpAudioAnalyzer{config: cfg, client: client}
}

type analyzerHTTPResponse struct {
	AnalyzerID      string             `json:"analyzer_id"`
	AnalyzerVersion string             `json:"analyzer_version"`
	ModelVersion    string             `json:"model_version"`
	DurationMS      int64              `json:"duration_ms"`
	Features        map[string]any     `json:"features"`
	ModelLabels     map[string]float64 `json:"model_labels"`
}

func (analyzer *httpAudioAnalyzer) Analyze(ctx context.Context, input audioAnalyzerInput) (*audioAnalyzerResult, error) {
	if input.Audio == nil || input.ContentLength <= 0 {
		return nil, newAnalyzerFailure("invalid_audio_source", false, "audio source is empty", nil)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, analyzer.config.Endpoint, input.Audio)
	if err != nil {
		return nil, newAnalyzerFailure("analyzer_request_invalid", false, "analyzer request could not be created", err)
	}
	request.ContentLength = input.ContentLength
	request.Header.Set("Authorization", "Bearer "+analyzer.config.Token)
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("X-Music-Online-Music-ID", strconv.FormatUint(uint64(input.MusicID), 10))
	request.Header.Set("X-Music-Online-File-Hash", input.FileHash)
	request.Header.Set("X-Music-Online-Content-Revision", strconv.FormatUint(input.ContentRevision, 10))
	request.Header.Set("X-Music-Online-Max-Duration-Ms", strconv.FormatInt(input.MaxDuration.Milliseconds(), 10))

	response, err := analyzer.client.Do(request)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, newAnalyzerFailure("analyzer_timeout", true, "analyzer request timed out", err)
		}
		if errors.Is(err, context.Canceled) || errors.Is(ctx.Err(), context.Canceled) {
			return nil, newAnalyzerFailure("analyzer_cancelled", false, "analyzer request was cancelled", err)
		}
		return nil, newAnalyzerFailure("analyzer_unavailable", true, "analyzer could not be reached", err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxAnalyzerResponseBytes+1))
	if err != nil {
		return nil, newAnalyzerFailure("analyzer_response_read_failed", true, "analyzer response could not be read", err)
	}
	if len(body) > maxAnalyzerResponseBytes {
		return nil, newAnalyzerFailure("analyzer_response_too_large", false, "analyzer response exceeded the size limit", nil)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		retryable := response.StatusCode == http.StatusTooManyRequests || response.StatusCode == http.StatusRequestTimeout || response.StatusCode >= http.StatusInternalServerError
		return nil, newAnalyzerFailure(
			"analyzer_http_"+strconv.Itoa(response.StatusCode), retryable,
			"analyzer rejected the audio stream", nil,
		)
	}
	if contentType := strings.ToLower(response.Header.Get("Content-Type")); contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		return nil, newAnalyzerFailure("analyzer_protocol_error", false, "analyzer returned a non-JSON response", nil)
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var decoded analyzerHTTPResponse
	if err := decoder.Decode(&decoded); err != nil {
		return nil, newAnalyzerFailure("analyzer_protocol_error", false, "analyzer response was invalid", err)
	}
	if err := ensureJSONEOF(decoder); err != nil {
		return nil, newAnalyzerFailure("analyzer_protocol_error", false, "analyzer response contained trailing data", err)
	}
	if decoded.AnalyzerID != analyzer.config.ID || decoded.AnalyzerVersion != analyzer.config.Version || decoded.ModelVersion != analyzer.config.ModelVersion {
		return nil, newAnalyzerFailure("analyzer_version_mismatch", false, "analyzer response version did not match configuration", nil)
	}
	if decoded.DurationMS <= 0 || decoded.DurationMS > input.MaxDuration.Milliseconds() {
		return nil, newAnalyzerFailure("analyzer_duration_invalid", false, "analyzer reported an invalid duration", nil)
	}
	if err := validateAnalyzerFeatures(decoded.Features); err != nil {
		return nil, newAnalyzerFailure("analyzer_features_invalid", false, "analyzer returned invalid features", err)
	}
	if err := validateAnalyzerLabels(decoded.ModelLabels); err != nil {
		return nil, newAnalyzerFailure("analyzer_labels_invalid", false, "analyzer returned invalid model labels", err)
	}
	return &audioAnalyzerResult{
		AnalyzerID: decoded.AnalyzerID, AnalyzerVersion: decoded.AnalyzerVersion,
		ModelVersion: decoded.ModelVersion, DurationMS: decoded.DurationMS,
		Features: decoded.Features, ModelLabels: decoded.ModelLabels,
	}, nil
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("additional JSON value")
		}
		return err
	}
	return nil
}

func validateAnalyzerFeatures(features map[string]any) error {
	if features == nil {
		return errors.New("features object is required")
	}
	if len(features) > 128 {
		return errors.New("too many feature keys")
	}
	return validateAnalyzerJSONValue(features, 0)
}

func validateAnalyzerJSONValue(value any, depth int) error {
	if depth > 4 {
		return errors.New("feature nesting is too deep")
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) > 128 {
			return errors.New("feature object is too large")
		}
		for key, child := range typed {
			if strings.TrimSpace(key) == "" || len(key) > 100 {
				return errors.New("feature key is invalid")
			}
			if err := validateAnalyzerJSONValue(child, depth+1); err != nil {
				return fmt.Errorf("feature %q: %w", key, err)
			}
		}
	case []any:
		if len(typed) > 256 {
			return errors.New("feature array is too large")
		}
		for _, child := range typed {
			if err := validateAnalyzerJSONValue(child, depth+1); err != nil {
				return err
			}
		}
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return errors.New("feature number is not finite")
		}
	case string:
		if len(typed) > 200 {
			return errors.New("feature string is too long")
		}
	case bool, nil:
	default:
		return fmt.Errorf("unsupported feature value %T", value)
	}
	return nil
}

func validateAnalyzerLabels(labels map[string]float64) error {
	if labels == nil {
		return errors.New("model_labels object is required")
	}
	if len(labels) > 256 {
		return errors.New("too many model labels")
	}
	for label, score := range labels {
		if strings.TrimSpace(label) == "" || len(label) > 100 {
			return errors.New("model label is invalid")
		}
		if math.IsNaN(score) || math.IsInf(score, 0) || score < 0 || score > 1 {
			return fmt.Errorf("model label %q score must be between 0 and 1", label)
		}
	}
	return nil
}
