package store

import "testing"

func TestParseRank(t *testing.T) {
	cases := map[string]AllianceRank{
		"MEMBER":  RankMember,
		"OFFICER": RankOfficer,
		"LEADER":  RankLeader,
		"bogus":   RankMember,
		"":        RankMember,
	}
	for in, want := range cases {
		if got := ParseRank(in); got != want {
			t.Errorf("ParseRank(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAllianceRankRights(t *testing.T) {
	// Leader holds every right.
	if !RankLeader.CanManageAlliance() || !RankLeader.CanManageApplications() ||
		!RankLeader.CanModerateBoard() || !RankLeader.CanPost() {
		t.Error("leader should hold all rights")
	}
	// Officer can manage applications + moderate the board, but not the alliance.
	if RankOfficer.CanManageAlliance() {
		t.Error("officer must not manage alliance settings")
	}
	if !RankOfficer.CanManageApplications() || !RankOfficer.CanModerateBoard() || !RankOfficer.CanPost() {
		t.Error("officer should manage applications, moderate the board and post")
	}
	// Member may only post.
	if RankMember.CanManageAlliance() || RankMember.CanManageApplications() || RankMember.CanModerateBoard() {
		t.Error("member must not manage anything")
	}
	if !RankMember.CanPost() {
		t.Error("member should be allowed to post")
	}
}

func TestZeroRankHasNoRights(t *testing.T) {
	var zero AllianceRank
	if zero.CanPost() || zero.CanModerateBoard() || zero.CanManageApplications() || zero.CanManageAlliance() {
		t.Error("the zero rank must hold no rights")
	}
}

func TestParseDiplomacyKind(t *testing.T) {
	cases := map[string]string{
		"WAR":   DiplomacyWar,
		"war":   DiplomacyWar,
		" nap ": DiplomacyNAP,
		"NAP":   DiplomacyNAP,
	}
	for in, want := range cases {
		got, ok := ParseDiplomacyKind(in)
		if !ok || got != want {
			t.Errorf("ParseDiplomacyKind(%q) = (%q, %v), want %q", in, got, ok, want)
		}
	}
	if _, ok := ParseDiplomacyKind("trade"); ok {
		t.Error("unknown kind must not parse")
	}
}
