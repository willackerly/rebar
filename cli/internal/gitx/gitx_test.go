package gitx

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/willackerly/rebar/cli/internal/testutil"
)

func TestLines(t *testing.T) {
	for _, tc := range []struct {
		out  string
		want []string
	}{
		{"", nil},
		{"a\n", []string{"a"}},
		{"a\n\nb\n", []string{"a", "", "b"}},
		{"a\nb", []string{"a"}}, // read never returns the unterminated b
		{"\n", []string{""}},
	} {
		if got := Lines(tc.out); !slices.Equal(got, tc.want) {
			t.Errorf("Lines(%q) = %q, want %q", tc.out, got, tc.want)
		}
	}
}

func TestRunAndMessage(t *testing.T) {
	testutil.IsolateGitConfig(t)
	repo := testutil.InitRepo(t)
	if _, code := Run(repo, nil, "rev-parse", "HEAD"); code == 0 {
		t.Fatal("rev-parse HEAD succeeded in an empty repository")
	}
	var errw bytes.Buffer
	if _, code := Run(repo, &errw, "log", "-1", "HEAD"); code != 128 || errw.Len() == 0 {
		t.Errorf("git log in an empty repository: exit %d, stderr %q", code, errw.String())
	}
	testutil.Git(t, repo, "commit", "-q", "--allow-empty", "--cleanup=verbatim", "-m", "fix: x\n\nbody\n\n")
	subject, body := Message(repo, nil, "HEAD")
	if subject != "fix: x" || body != "fix: x\n\nbody" {
		t.Errorf("Message = %q, %q", subject, body)
	}
	if top := Subst(repo, "rev-parse", "--show-toplevel"); top == "" || strings.HasSuffix(top, "\n") {
		t.Errorf("Subst kept the newline or lost the output: %q", top)
	}
	if !Available() {
		t.Error("git is not available")
	}
}
