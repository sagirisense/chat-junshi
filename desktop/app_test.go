package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomModelsDoesNotRequirePythonService(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`{"data":[{"id":"synthetic-model"}]}`)) }))
	defer server.Close()
	app := NewApp()
	got, err := app.Call("models", map[string]interface{}{"provider": "custom", "api": map[string]interface{}{"base_url": server.URL}})
	if err != nil {
		t.Fatal(err)
	}
	if got.(map[string]interface{})["models"].([]string)[0] != "synthetic-model" {
		t.Fatal(got)
	}
	if app.service != nil {
		t.Fatal("started Python service")
	}
}
