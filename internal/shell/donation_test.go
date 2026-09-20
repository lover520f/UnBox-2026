package shell

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unbox/unbox/internal/store"
)

type donationRoundTripFunc func(*http.Request) (*http.Response, error)

func (f donationRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func donationTestResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func donationCachedJSON(name string, amount float64, cachedAt int64) string {
	return fmt.Sprintf(
		`{"updatedAt":"cache-time","donors":[{"id":"cache-id","name":%q,"avatar":"cache-avatar","amount":%g,"anonymous":false}],"cachedAt":%d}`,
		name, amount, cachedAt,
	)
}

func TestDonationCacheHitDoesNotFetch(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/donation.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := time.Unix(10_000, 0)
	if err := st.SetKV(donationCacheKey, donationCachedJSON("缓存用户", 12, now.Unix())); err != nil {
		t.Fatal(err)
	}

	var requests atomic.Int32
	svc := newShellService(nil, nil, st)
	svc.now = func() time.Time { return now }
	svc.httpClient = &http.Client{Transport: donationRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("缓存命中时不应发起网络请求")
	})}

	got := svc.GetDonationLeaderboard()
	if requests.Load() != 0 {
		t.Fatalf("cache hit issued %d requests, want 0", requests.Load())
	}
	if got.UpdatedAt != "cache-time" || len(got.Donors) != 1 || got.Donors[0].Name != "缓存用户" {
		t.Fatalf("GetDonationLeaderboard() = %+v, want cached leaderboard", got)
	}
}

func TestDonationRemoteSuccessWritesCache(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/donation.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := time.Unix(20_000, 0)
	remote := `{"updatedAt":"remote-time","donors":[{"id":"u-1","name":"远端用户","avatar":"https://example.com/avatar.png","amount":88,"anonymous":false}]}`
	var requests atomic.Int32
	svc := newShellService(nil, nil, st)
	svc.now = func() time.Time { return now }
	svc.httpClient = &http.Client{Transport: donationRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		requests.Add(1)
		if req.URL.String() != donationURL {
			t.Fatalf("request URL = %q, want %q", req.URL.String(), donationURL)
		}
		return donationTestResponse(remote), nil
	})}

	got := svc.GetDonationLeaderboard()
	if requests.Load() != 1 {
		t.Fatalf("remote fetch issued %d requests, want 1", requests.Load())
	}
	if got.UpdatedAt != "remote-time" || len(got.Donors) != 1 || got.Donors[0].Name != "远端用户" {
		t.Fatalf("GetDonationLeaderboard() = %+v, want remote leaderboard", got)
	}

	raw, ok, err := st.GetKV(donationCacheKey)
	if err != nil || !ok {
		t.Fatalf("cache missing: ok=%v err=%v", ok, err)
	}
	var cached struct {
		UpdatedAt string `json:"updatedAt"`
		Donors    []struct {
			Name string `json:"name"`
		} `json:"donors"`
		CachedAt int64 `json:"cachedAt"`
	}
	if err := jsonUnmarshal(raw, &cached); err != nil {
		t.Fatalf("cache JSON invalid: %v\n%s", err, raw)
	}
	if cached.UpdatedAt != "remote-time" || len(cached.Donors) != 1 || cached.Donors[0].Name != "远端用户" {
		t.Fatalf("cached leaderboard = %+v, want remote data", cached)
	}
	if cached.CachedAt != now.Unix() {
		t.Fatalf("cachedAt = %d, want %d", cached.CachedAt, now.Unix())
	}
}

func TestDonationRemoteFailureFallsBackToCache(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/donation.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := time.Unix(30_000, 0)
	if err := st.SetKV(donationCacheKey, donationCachedJSON("过期缓存", 23, now.Add(-donationCacheTTL-time.Minute).Unix())); err != nil {
		t.Fatal(err)
	}

	var requests atomic.Int32
	svc := newShellService(nil, nil, st)
	svc.now = func() time.Time { return now }
	svc.httpClient = &http.Client{Transport: donationRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("网络不可用")
	})}

	got := svc.GetDonationLeaderboard()
	if requests.Load() != 1 {
		t.Fatalf("expired cache issued %d requests, want 1", requests.Load())
	}
	if got.UpdatedAt != "cache-time" || len(got.Donors) != 1 || got.Donors[0].Name != "过期缓存" {
		t.Fatalf("GetDonationLeaderboard() = %+v, want expired cache fallback", got)
	}
}

func TestDonationNoCacheFallsBackToSnapshot(t *testing.T) {
	originalSnapshot := donorsSnapshot
	donorsSnapshot = []byte(`{
		"updatedAt":"snapshot-time",
		"donors":[
			{"id":"snapshot-id","name":"内置用户","avatar":"snapshot.png","amount":66,"anonymous":false}
		]
	}`)
	t.Cleanup(func() { donorsSnapshot = originalSnapshot })

	var requests atomic.Int32
	svc := newShellService(nil, nil, nil)
	svc.httpClient = &http.Client{Transport: donationRoundTripFunc(func(*http.Request) (*http.Response, error) {
		requests.Add(1)
		return nil, errors.New("网络不可用")
	})}

	got := svc.GetDonationLeaderboard()
	if requests.Load() != 1 {
		t.Fatalf("no cache issued %d requests, want 1", requests.Load())
	}
	if got.UpdatedAt != "snapshot-time" || len(got.Donors) != 1 || got.Donors[0].Name != "内置用户" {
		t.Fatalf("GetDonationLeaderboard() = %+v, want decoded built-in snapshot", got)
	}
}

func TestDonationInvalidJSONFallsBackWithoutPanic(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/donation.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := time.Unix(40_000, 0)
	if err := st.SetKV(donationCacheKey, donationCachedJSON("缓存兜底", 34, now.Add(-2*donationCacheTTL).Unix())); err != nil {
		t.Fatal(err)
	}

	svc := newShellService(nil, nil, st)
	svc.now = func() time.Time { return now }
	svc.httpClient = &http.Client{Transport: donationRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return donationTestResponse(`{"updatedAt":"broken","donors":[`), nil
	})}

	got := svc.GetDonationLeaderboard()
	if got.UpdatedAt != "cache-time" || len(got.Donors) != 1 || got.Donors[0].Name != "缓存兜底" {
		t.Fatalf("GetDonationLeaderboard() = %+v, want cache fallback after invalid JSON", got)
	}
}

func TestDonationMissingFieldsFallBackWithoutPanic(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/donation.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	now := time.Unix(50_000, 0)
	if err := st.SetKV(donationCacheKey, donationCachedJSON("字段兜底", 45, now.Add(-2*donationCacheTTL).Unix())); err != nil {
		t.Fatal(err)
	}

	svc := newShellService(nil, nil, st)
	svc.now = func() time.Time { return now }
	svc.httpClient = &http.Client{Transport: donationRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return donationTestResponse(`{"updatedAt":"broken"}`), nil
	})}

	got := svc.GetDonationLeaderboard()
	if got.UpdatedAt != "cache-time" || len(got.Donors) != 1 || got.Donors[0].Name != "字段兜底" {
		t.Fatalf("GetDonationLeaderboard() = %+v, want cache fallback after missing fields", got)
	}
}

func TestDonationSortsAndNormalizesAnonymous(t *testing.T) {
	remote := `{
		"updatedAt":"remote-time",
		"donors":[
			{"id":"z","name":"Zed","avatar":"z.png","amount":10,"anonymous":false},
			{"id":"a","name":"Amy","avatar":"a.png","amount":10,"anonymous":false},
			{"id":"secret","name":"不该显示","avatar":"secret.png","amount":20,"anonymous":true},
			{"id":"c","name":"Cara","avatar":"c.png","amount":5,"anonymous":false}
		]
	}`
	svc := newShellService(nil, nil, nil)
	svc.httpClient = &http.Client{Transport: donationRoundTripFunc(func(*http.Request) (*http.Response, error) {
		return donationTestResponse(remote), nil
	})}

	got := svc.GetDonationLeaderboard()
	if len(got.Donors) != 4 {
		t.Fatalf("donor count = %d, want 4: %+v", len(got.Donors), got.Donors)
	}
	wantNames := []string{"热心网友", "Amy", "Zed", "Cara"}
	for i, want := range wantNames {
		if got.Donors[i].Name != want {
			t.Fatalf("donor[%d].Name = %q, want %q; all donors = %+v", i, got.Donors[i].Name, want, got.Donors)
		}
	}
	if got.Donors[0].ID != "" || got.Donors[0].Avatar != "" || !got.Donors[0].Anonymous {
		t.Fatalf("anonymous donor not normalized: %+v", got.Donors[0])
	}
}

// jsonUnmarshal 让测试只依赖标准库的解析结果，同时保持断言处的错误信息清晰。
func jsonUnmarshal(raw string, v any) error {
	return json.Unmarshal([]byte(raw), v)
}
