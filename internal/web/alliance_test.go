package web

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

// TestAllianceTemplateRenders exercises every alliance tab so a bad field or
// method call fails the test rather than only showing at runtime.
func TestAllianceTemplateRenders(t *testing.T) {
	fm := template.FuncMap{"n": fmtInt, "nf": fmtFloat, "coords": coords}
	tmpl := template.Must(template.New("").Funcs(fm).ParseFS(templatesFS, "templates/*.html"))

	detail := &allianceDetail{
		ID: 3, Tag: "nWo", Name: "new World order", Description: "We rule.",
		FounderID: 7, FounderName: "capy", IsOpen: true, MinPoints: 0,
		Points: 118952, Members: 2, CreatedAt: "2026-10-12",
	}
	members := []allianceMemberRow{
		{UserID: 7, Username: "capy", Rank: "LEADER", Joined: "2026-10-12", IsFounder: true, IsMe: true},
		{UserID: 9, Username: "bandit", Rank: "MEMBER", Joined: "2026-10-12"},
	}
	posts := []alliancePostRow{
		{ID: 1, AuthorID: 7, AuthorName: "capy", Title: "Welcome", Body: "hi", IsPinned: true, When: "2026-10-12 09:00", CanDelete: true},
	}
	apps := []allianceApplicationRow{
		{ID: 4, AllianceID: 3, Tag: "nWo", Name: "new World order", Username: "bandit", UserID: 9, Message: "let me in", When: "2026-10-12 09:00"},
	}

	cases := []allianceData{
		{
			baseData: baseData{Title: "Alliance"},
			Tab:      "browse",
			Page:     1, Pages: 1, Total: 1,
			Alliances: []allianceListRow{{ID: 3, Tag: "nWo", Name: "new World order", Points: 118952, Members: 2, IsOpen: true}},
			MyApps:    apps,
		},
		{baseData: baseData{Title: "Alliance"}, Tab: "create"},
		{baseData: baseData{Title: "Alliance"}, Tab: "myapps", MyApps: apps},
		{
			baseData: baseData{Title: "Alliance"}, Tab: "profile",
			Alliance: detail, Members: members, Applied: true, AppID: 4,
		},
		{
			baseData: baseData{Title: "Alliance"}, Tab: "dashboard",
			Alliance: detail, Members: members, InAlliance: true, MyRank: "LEADER", CanManageAlliance: true,
		},
		{
			baseData: baseData{Title: "Alliance"}, Tab: "members",
			Alliance: detail, Members: members, InAlliance: true, CanManageAlliance: true,
		},
		{
			baseData: baseData{Title: "Alliance"}, Tab: "board",
			Alliance: detail, Posts: posts, InAlliance: true, CanModerateBoard: true,
		},
		{
			baseData: baseData{Title: "Alliance"}, Tab: "applications",
			Alliance: detail, Pending: apps, InAlliance: true, CanManageApplications: true,
		},
		{
			baseData: baseData{Title: "Alliance"}, Tab: "settings",
			Alliance: detail, InAlliance: true, CanManageAlliance: true,
		},
	}
	for _, d := range cases {
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, "alliance", d); err != nil {
			t.Errorf("tab %s: %v", d.Tab, err)
			continue
		}
		if !strings.Contains(buf.String(), "Alliance") {
			t.Errorf("tab %s: page did not render", d.Tab)
		}
	}
}

func TestAllianceSortKeys(t *testing.T) {
	if got := allianceSort("name"); got != "name" {
		t.Errorf("allianceSort(name) = %q", got)
	}
	if got := allianceSort("members"); got != "members" {
		t.Errorf("allianceSort(members) = %q", got)
	}
	if got := allianceSort("bogus"); got != "points" {
		t.Errorf("allianceSort(bogus) = %q, want points", got)
	}
}

func TestValidateAlliance(t *testing.T) {
	if m := validateAllianceTag("nWo"); m != "" {
		t.Errorf("tag nWo rejected: %q", m)
	}
	if m := validateAllianceTag("ThF-"); m != "" {
		t.Errorf("tag ThF- rejected: %q", m)
	}
	for _, bad := range []string{"", "a", "toolongtag", "bad tag", "bad!"} {
		if validateAllianceTag(bad) == "" {
			t.Errorf("tag %q unexpectedly valid", bad)
		}
	}
	if validateAllianceName("ab") == "" {
		t.Error("2-char name should be rejected")
	}
	if m := validateAllianceName("new World order"); m != "" {
		t.Errorf("name rejected: %q", m)
	}
	if validatePost("ab", "body") == "" {
		t.Error("short title should be rejected")
	}
	if validatePost("Welcome", "  ") == "" {
		t.Error("empty body should be rejected")
	}
	if m := validatePost("Welcome", "hi"); m != "" {
		t.Errorf("valid post rejected: %q", m)
	}
}
