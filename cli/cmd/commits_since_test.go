package cmd

import (
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

// A commit made on the last-synced day itself counts as "since last sync".
// `--since=<date>` alone means that date at the current time of day, so
// commits made earlier that day were dropped (and the count depended on when
// the check ran).
func TestCommitsSinceIncludesTheWholeSyncDay(t *testing.T) {
	testutil.IsolateGitConfig(t)
	repo := testutil.InitRepo(t)
	for _, c := range []struct{ date, msg string }{
		{"2023-06-14 23:59:00", "day before"},
		{"2023-06-15 00:00:05", "sync day, just after midnight"},
		{"2023-06-16 12:00:00", "day after"},
	} {
		t.Setenv("GIT_AUTHOR_DATE", c.date)
		t.Setenv("GIT_COMMITTER_DATE", c.date)
		testutil.Git(t, repo, "commit", "-q", "--allow-empty", "-m", c.msg)
	}
	if got := commitsSince(repo, "2023-06-15"); got != "2" {
		t.Fatalf("commitsSince(2023-06-15) = %q, want 2 (the sync day's commit and the next day's)", got)
	}
}
