package shellexec

import "testing"

func TestPosixCwdExprNoWshRemote(t *testing.T) {
	tests := []struct {
		name    string
		cwd     string
		sshUser string
		want    string
	}{
		{
			name:    "tilde-dir-uses-username-home",
			cwd:     "~/.ssh",
			sshUser: "root",
			want:    "~root/.ssh",
		},
		{
			name:    "tilde-root-uses-username-home",
			cwd:     "~",
			sshUser: "root",
			want:    "~root",
		},
		{
			name:    "tilde-slash-uses-username-home",
			cwd:     "~/",
			sshUser: "root",
			want:    "~root/",
		},
		{
			name:    "non-tilde-falls-back",
			cwd:     "/var/log",
			sshUser: "root",
			want:    "/var/log",
		},
		{
			name:    "missing-user-falls-back-to-home-var",
			cwd:     "~/.ssh",
			sshUser: "",
			want:    "\"$HOME/.ssh\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := posixCwdExprNoWshRemote(tt.cwd, tt.sshUser)
			if got != tt.want {
				t.Fatalf("posixCwdExprNoWshRemote(%q, %q)=%q, want %q", tt.cwd, tt.sshUser, got, tt.want)
			}
		})
	}
}

