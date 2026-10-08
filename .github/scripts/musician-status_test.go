package main

import (
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/3899/ncmm/pkg/log"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise a real SDK request: compiling alone did not catch its dependency
// on the global logger, and task progress must stay separate from total plays.
func TestStatusRequestPreservesTaskProgress(t *testing.T) {
	log.Default = nil
	cookiePath := filepath.Join(t.TempDir(), "cookie.json")
	if err := os.WriteFile(cookiePath, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	client, err := newStatusClient(cookiePath)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close(context.Background())
	called := false
	client.GetClient().Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		called = true
		if r.URL.Path != "/eapi/nmusician/workbench/special/right/vip/info" || r.Method != http.MethodPost {
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body := `{"code":200,"data":{"isMusician":true,"recentPlayCount30":586,"furtherTask":{"children":[{"name":"有效播放达650次","missionCode":"mission_code_recently_play_effect_count_v2","missionStatus":50,"progressRate":4,"totalCompleteNum":100,"taskProgressText":"4%"}]}}}`
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
	})
	response, err := queryStatus(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if !called || response.Data.RecentPlayCount30 != 586 || len(response.Data.FurtherTask.Children) != 1 {
		t.Fatalf("invalid response: %+v", response)
	}
	child := response.Data.FurtherTask.Children[0]
	if child.ProgressRate != 4 || child.TotalCompleteNum != 100 || child.MissionStatus != 50 || child.TaskProgressText != "4%" {
		t.Fatalf("raw task progress was changed: %+v", child)
	}
}
