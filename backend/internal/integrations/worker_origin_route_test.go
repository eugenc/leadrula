package integrations

import (
	"os"
	"strings"
	"testing"
)

func TestWorkerExecuteJob_doesNotApplyOriginRoute(t *testing.T) {
	t.Helper()
	src, err := os.ReadFile("worker.go")
	if err != nil {
		t.Fatalf("read worker.go: %v", err)
	}
	if strings.Contains(string(src), "TryApplyConnectionOriginRoute") {
		t.Fatal("outbound worker must not call TryApplyConnectionOriginRoute; origin routes belong on inbound webhooks only")
	}
}
