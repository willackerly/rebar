package repo

import "testing"

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"ssh short", "git@github.com:willackerly/rebar.git", "github.com/willackerly/rebar", false},
		{"ssh short no .git", "git@github.com:willackerly/rebar", "github.com/willackerly/rebar", false},
		{"https with .git", "https://github.com/willackerly/rebar.git", "github.com/willackerly/rebar", false},
		{"https no .git", "https://github.com/willackerly/rebar", "github.com/willackerly/rebar", false},
		{"ssh:// scheme", "ssh://git@gitlab.com/team/foo.git", "gitlab.com/team/foo", false},
		{"gitlab nested groups", "git@gitea.example.com:nested/group/repo.git", "gitea.example.com/nested/group/repo", false},
		{"uppercase host lowered", "https://GitHub.com/Foo/Bar.git", "github.com/Foo/Bar", false},
		{"trailing slash", "https://github.com/foo/bar/", "github.com/foo/bar", false},
		{"http scheme", "http://example.com/foo/bar.git", "example.com/foo/bar", false},
		{"https with port", "https://example.com:8443/foo/bar.git", "example.com/foo/bar", false},
		{"git scheme", "git://github.com/foo/bar.git", "github.com/foo/bar", false},

		{"empty", "", "", true},
		{"no path", "https://github.com", "", true},
		{"garbage", "not-a-url", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseRemoteURL(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got %q", tc.input, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("ParseRemoteURL(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}
