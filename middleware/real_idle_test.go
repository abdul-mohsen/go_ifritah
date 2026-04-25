package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// TestRealIdle_DoesNotLogOut is a real end-to-end test that:
//  1. Logs into the running app at http://localhost:8000 with ssda/Qwerty123
//  2. Verifies an authenticated page loads
//  3. Sleeps for 16 minutes (longer than the 15-min access token lifetime)
//  4. Clicks a page and verifies the user is NOT redirected to login
//
// Gated behind RUN_REAL_IDLE_TEST=1 so CI doesn't wait 16 min.
//
// Run with:
//
//	$env:RUN_REAL_IDLE_TEST="1"; go test ./middleware/ -run TestRealIdle_DoesNotLogOut -v -timeout 30m
func TestRealIdle_DoesNotLogOut(t *testing.T) {
	if os.Getenv("RUN_REAL_IDLE_TEST") != "1" {
		t.Skip("set RUN_REAL_IDLE_TEST=1 to run this 16-minute test")
	}

	const base = "http://localhost:8000"
	const idleDuration = 16 * time.Minute

	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't auto-follow, we want to see redirects
		},
	}

	// 1) login
	t.Log("==> logging in")
	form := url.Values{}
	form.Set("username", "ssda")
	form.Set("password", "Qwerty123")
	resp, err := client.PostForm(base+"/login", form)
	if err != nil {
		t.Fatalf("login request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	t.Logf("login status=%d hxRedirect=%q body=%s", resp.StatusCode, resp.Header.Get("HX-Redirect"), truncate(string(body), 200))

	u, _ := url.Parse(base)
	cookies := jar.Cookies(u)
	var sid string
	for _, c := range cookies {
		if c.Name == "session_id" {
			sid = c.Value
		}
	}
	if sid == "" {
		t.Fatalf("no session_id cookie after login (got cookies: %v)", cookies)
	}
	t.Logf("session_id=%s...", truncate(sid, 16))

	// 2) baseline authenticated request
	t.Log("==> baseline /dashboard")
	if !pageOK(t, client, base+"/dashboard") {
		t.Fatal("baseline dashboard request failed — login flow is broken before we even start the idle test")
	}

	// 3) idle
	t.Logf("==> sleeping %v to let access token expire", idleDuration)
	startedIdle := time.Now()
	for elapsed := time.Duration(0); elapsed < idleDuration; elapsed = time.Since(startedIdle) {
		time.Sleep(60 * time.Second)
		t.Logf("    idle %v / %v", elapsed.Round(time.Second), idleDuration)
	}

	// 4) "click" — request another authenticated page
	t.Log("==> clicking /dashboard/purchase-bills after idle")
	resp, err = client.Get(base + "/dashboard/purchase-bills")
	if err != nil {
		t.Fatalf("post-idle request failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(resp.Body)
	t.Logf("status=%d location=%q hxRedirect=%q",
		resp.StatusCode, resp.Header.Get("Location"), resp.Header.Get("HX-Redirect"))

	// Check the cookie survived
	cookiesAfter := jar.Cookies(u)
	var sidAfter string
	for _, c := range cookiesAfter {
		if c.Name == "session_id" {
			sidAfter = c.Value
		}
	}
	if sidAfter == "" {
		t.Errorf("session_id cookie was deleted after idle — user got logged out")
	} else {
		t.Logf("session_id after idle=%s...", truncate(sidAfter, 16))
	}

	// 303 redirects to /dashboard/purchase-bills (refresh path) are OK; /login or / means logged out.
	loc := resp.Header.Get("Location")
	if resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusFound {
		if loc == "/" || loc == "/login" {
			t.Errorf("redirected to login page (Location=%s) — user is logged out", loc)
		}
		t.Logf("redirect destination=%s", loc)
		// Follow once and verify
		if loc != "" && !strings.HasPrefix(loc, "/login") && loc != "/" {
			resp2, err := client.Get(base + loc)
			if err == nil {
				resp2.Body.Close()
				t.Logf("follow redirect: status=%d", resp2.StatusCode)
				if resp2.StatusCode == http.StatusSeeOther {
					if l2 := resp2.Header.Get("Location"); l2 == "/" || l2 == "/login" {
						t.Errorf("second redirect went to login (Location=%s)", l2)
					}
				}
			}
		}
	} else if resp.StatusCode == http.StatusUnauthorized {
		t.Errorf("got 401 after idle — user is logged out")
	} else if resp.StatusCode != http.StatusOK {
		t.Errorf("unexpected status %d after idle", resp.StatusCode)
	}

	// Final check: HX-Redirect to login also means logged out
	if hx := resp.Header.Get("HX-Redirect"); hx == "/" || hx == "/login" {
		t.Errorf("HX-Redirect=%s sends to login", hx)
	}

	// Verify by hitting another protected URL with the (possibly refreshed) cookie
	t.Log("==> follow-up /dashboard request to confirm session is alive")
	if !pageOK(t, client, base+"/dashboard") {
		t.Errorf("follow-up /dashboard failed — session is dead after idle")
	}
}

func pageOK(t *testing.T, client *http.Client, url string) bool {
	t.Helper()
	resp, err := client.Get(url)
	if err != nil {
		t.Logf("    request error: %v", err)
		return false
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	loc := resp.Header.Get("Location")
	t.Logf("    %s -> %d location=%q snippet=%s", url, resp.StatusCode, loc, truncate(htmlTitle(string(body)), 80))

	// Redirects to / or /login mean logged out
	if (resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusFound) &&
		(loc == "/" || loc == "/login") {
		return false
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return false
	}
	return resp.StatusCode < 400
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func htmlTitle(s string) string {
	i := strings.Index(s, "<title>")
	if i < 0 {
		return ""
	}
	j := strings.Index(s[i:], "</title>")
	if j < 0 {
		return ""
	}
	return s[i+7 : i+j]
}

// keep json import used (in case we later parse responses)
var _ = json.Marshal
var _ = fmt.Sprintf
