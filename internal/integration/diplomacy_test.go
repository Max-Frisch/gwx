package integration

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// TestAllianceDiplomacy drives NAP/war declarations through the real HTTP
// surface: two founders declare war (unilateral), end it, then exchange and
// accept a NAP.
func TestAllianceDiplomacy(t *testing.T) {
	h := newHarness(t)
	a := h.register(t, "diplo-alpha")
	b := h.register(t, "diplo-beta")

	h.action(t, a.client, "/alliance/create", url.Values{
		"tag": {"ALPHA"}, "name": {"Alpha Alliance"}, "description": {"first"},
	})
	h.action(t, b.client, "/alliance/create", url.Values{
		"tag": {"BETA"}, "name": {"Beta Alliance"}, "description": {"second"},
	})

	alphaID := h.queryInt(t, `SELECT id FROM alliances WHERE tag = 'ALPHA'`)
	betaID := h.queryInt(t, `SELECT id FROM alliances WHERE tag = 'BETA'`)

	// The diplomacy tab renders with the war option for the founder.
	status, body := h.get(t, a.client, "/alliance/internal/diplomacy")
	if status != 200 {
		t.Fatalf("GET /alliance/internal/diplomacy -> %d", status)
	}
	if !strings.Contains(body, "War declaration") || !strings.Contains(body, `action="/alliance/internal/diplomacy/propose"`) {
		t.Fatalf("diplomacy page missing the declaration form:\n%s", snippet([]byte(body)))
	}

	// War is unilateral: immediately ACTIVE.
	h.action(t, a.client, "/alliance/internal/diplomacy/propose", url.Values{
		"targetID": {i64(betaID)}, "kind": {"WAR"}, "message": {"prepare"},
	})
	if got := h.queryInt(t, `
		SELECT COUNT(*) FROM alliance_diplomacy
		WHERE proposer_id = $1 AND target_id = $2 AND kind = 'WAR' AND status = 'ACTIVE'
	`, alphaID, betaID); got != 1 {
		t.Fatalf("war declaration not active (count=%d)", got)
	}

	warID := h.queryInt(t, `SELECT id FROM alliance_diplomacy WHERE proposer_id = $1 AND target_id = $2`, alphaID, betaID)

	// The defending side may seek peace.
	h.action(t, b.client, "/alliance/internal/diplomacy/"+i64(warID)+"/end", nil)
	if got := h.queryInt(t, `SELECT COUNT(*) FROM alliance_diplomacy WHERE id = $1 AND status = 'ENDED'`, warID); got != 1 {
		t.Fatalf("war not ended (count=%d)", got)
	}

	// A NAP is bilateral: proposal starts PROPOSED, the peer accepts.
	h.action(t, a.client, "/alliance/internal/diplomacy/propose", url.Values{
		"targetID": {i64(betaID)}, "kind": {"NAP"}, "message": {"truce?"},
	})
	napID := h.queryInt(t, `
		SELECT id FROM alliance_diplomacy
		WHERE proposer_id = $1 AND target_id = $2 AND kind = 'NAP' AND status = 'PROPOSED'
	`, alphaID, betaID)
	if napID == 0 {
		t.Fatal("NAP was not proposed")
	}

	// A duplicate live relationship is refused.
	if dup := h.postFlashErr(t, b.client, "/alliance/internal/diplomacy/propose", url.Values{
		"targetID": {i64(alphaID)}, "kind": {"WAR"},
	}); dup == "" {
		t.Fatal("a second live relationship should have been refused")
	}

	// Target accepts the NAP.
	h.action(t, b.client, "/alliance/internal/diplomacy/"+i64(napID)+"/respond", url.Values{
		"decision": {"accept"},
	})
	if got := h.queryInt(t, `SELECT COUNT(*) FROM alliance_diplomacy WHERE id = $1 AND status = 'ACTIVE'`, napID); got != 1 {
		t.Fatalf("NAP not accepted (count=%d)", got)
	}

	// A non-member may not declare.
	outsider := h.register(t, "diplo-outsider")
	if denied := h.postFlashErr(t, outsider.client, "/alliance/internal/diplomacy/propose", url.Values{
		"targetID": {i64(alphaID)}, "kind": {"WAR"},
	}); denied == "" {
		t.Fatal("a non-member should not be able to declare diplomacy")
	}

	// The public profile shows the active NAP.
	_, profile := h.get(t, outsider.client, "/alliance/"+i64(alphaID))
	if !strings.Contains(profile, "NAP") || !strings.Contains(profile, "BETA") {
		t.Errorf("public profile did not show the active NAP:\n%s", snippet([]byte(profile)))
	}
}

func i64(n int64) string { return strconv.FormatInt(n, 10) }

// postFlashErr POSTs a form and returns the rejection flash, following the
// redirect so the error query parameter is visible.
func (h *harness) postFlashErr(t *testing.T, c *http.Client, path string, form url.Values) string {
	t.Helper()
	resp, err := c.PostForm(h.server.URL+path, form)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
	return resp.Request.URL.Query().Get("err")
}
