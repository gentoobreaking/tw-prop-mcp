package gis

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLandSurveyAdapter_FetchParcelGeometry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("request") != "GetFeature" {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"type": "FeatureCollection",
			"features": [{
				"type": "Feature",
				"geometry": {
					"type": "Polygon",
					"coordinates": [[[121.5, 25.0], [121.51, 25.0], [121.51, 25.01], [121.5, 25.01], [121.5, 25.0]]]
				},
				"properties": {"COUNTY": "台北市", "TOWN": "大安區"}
			}]
		}`))
	}))
	defer server.Close()

	adapter := NewLandSurveyAdapter(server.URL, server.Client())
	ctx := context.Background()
	wkt, err := adapter.FetchParcelGeometry(ctx, "台北市", "大安區", "001", "0001")
	if err != nil {
		t.Fatalf("FetchParcelGeometry error: %v", err)
	}
	if wkt == "" {
		t.Error("expected non-empty WKT")
	}
	if len(wkt) < 10 {
		t.Errorf("WKT too short: %s", wkt)
	}
}

func TestLandSurveyAdapter_FetchParcelGeometry_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type":"FeatureCollection","features":[]}`))
	}))
	defer server.Close()

	adapter := NewLandSurveyAdapter(server.URL, server.Client())
	ctx := context.Background()
	_, err := adapter.FetchParcelGeometry(ctx, "台北市", "大安區", "001", "0001")
	if err == nil {
		t.Error("expected error for empty feature collection")
	}
}

func TestLandSurveyAdapter_FetchRoadNetwork_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("typeNames") != "TWIS_NLSC_Road" {
			http.Error(w, "invalid typeNames", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"type": "FeatureCollection",
			"features": [{
				"type": "Feature",
				"id": "road_1",
				"geometry": {
					"type": "LineString",
					"coordinates": [[121.5, 25.0], [121.51, 25.01]]
				},
				"properties": {"Name": "仁愛路"}
			}]
		}`))
	}))
	defer server.Close()

	adapter := NewLandSurveyAdapter(server.URL, server.Client())
	ctx := context.Background()
	bbox := "POLYGON((121.5 25.0, 121.51 25.0, 121.51 25.01, 121.5 25.01, 121.5 25.0))"
	segments, err := adapter.FetchRoadNetwork(ctx, bbox)
	if err != nil {
		t.Fatalf("FetchRoadNetwork error: %v", err)
	}
	if len(segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Name != "仁愛路" {
		t.Errorf("expected name 仁愛路, got %q", segments[0].Name)
	}
}

func TestLandRegistryAdapter_FetchParcelGeometry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"type": "Feature",
			"geometry": {
				"type": "Polygon",
				"coordinates": [[[121.5, 25.0], [121.51, 25.0], [121.51, 25.01], [121.5, 25.01], [121.5, 25.0]]]
			}
		}`))
	}))
	defer server.Close()

	adapter := NewLandRegistryAdapter(server.URL, server.Client())
	ctx := context.Background()
	wkt, err := adapter.FetchParcelGeometry(ctx, "台北市", "大安區", "001", "0001")
	if err != nil {
		t.Fatalf("FetchParcelGeometry error: %v", err)
	}
	if wkt == "" {
		t.Error("expected non-empty WKT")
	}
}

func TestLandRegistryAdapter_FetchRoadNetwork_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/roads" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"type": "FeatureCollection",
			"features": [{
				"type": "Feature",
				"id": "road_1",
				"geometry": {
					"type": "LineString",
					"coordinates": [[121.5, 25.0], [121.51, 25.01]]
				},
				"properties": {"name": "忠孝東路"}
			}]
		}`))
	}))
	defer server.Close()

	adapter := NewLandRegistryAdapter(server.URL, server.Client())
	ctx := context.Background()
	bbox := "POLYGON((121.5 25.0, 121.51 25.0, 121.51 25.01, 121.5 25.01, 121.5 25.0))"
	segments, err := adapter.FetchRoadNetwork(ctx, bbox)
	if err != nil {
		t.Fatalf("FetchRoadNetwork error: %v", err)
	}
	if len(segments) != 1 {
		t.Errorf("expected 1 segment, got %d", len(segments))
	}
	if segments[0].Name != "忠孝東路" {
		t.Errorf("expected name 忠孝東路, got %q", segments[0].Name)
	}
}

func TestGISService_FallbackOrder(t *testing.T) {
	firstFail := true
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if firstFail {
			http.Error(w, "server 1 error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type":"Feature","geometry":{"type":"Point","coordinates":[121.5,25.0]}}`))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"type":"Feature","geometry":{"type":"Point","coordinates":[121.51,25.01]}}`))
	}))
	defer server2.Close()

	adapter1 := NewLandSurveyAdapter(server1.URL, server1.Client())
	adapter2 := NewLandSurveyAdapter(server2.URL, server2.Client())

	service := NewGISService([]GISSource{adapter1, adapter2}, true)
	ctx := context.Background()

	// First call - server1 fails, server2 succeeds
	wkt, err := service.FetchParcelGeometry(ctx, "台北市", "大安區", "001", "0001")
	if err != nil {
		t.Fatalf("first call error: %v", err)
	}
	if wkt != "POINT(121.51 25.01)" {
		t.Errorf("expected server2 result, got %q", wkt)
	}

	// Second call - server1 now succeeds
	firstFail = false
	wkt, err = service.FetchParcelGeometry(ctx, "台北市", "大安區", "001", "0001")
	if err != nil {
		t.Fatalf("second call error: %v", err)
	}
	if wkt != "POINT(121.5 25)" {
		t.Errorf("expected server1 result, got %q", wkt)
}
}
func TestGISService_NoFallback(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server error", http.StatusInternalServerError)
	}))
	defer server.Close()

	adapter := NewLandSurveyAdapter(server.URL, server.Client())
	service := NewGISService([]GISSource{adapter}, false)
	ctx := context.Background()

	_, err := service.FetchParcelGeometry(ctx, "台北市", "大安區", "001", "0001")
	if err == nil {
		t.Error("expected error when fallback disabled")
	}
}