package git

import "testing"

func Test_CheckForValidForkRemote(t *testing.T) {
	type args struct {
		upstreamURL string
		remoteURL   string
		repo        string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "#1 - Success",
			args: args{
				upstreamURL: "https://github.com/rancher/charts",
				remoteURL:   "https://github.com/user/charts",
				repo:        "charts",
			},
			want: true,
		},
		{
			name: "#2 - Fail",
			args: args{
				upstreamURL: "https://github.com/rancher/charts",
				remoteURL:   "https://github.com/user/chartz",
				repo:        "charts",
			},
			want: false,
		},
		{
			name: "#3 - Fail",
			args: args{
				upstreamURL: "https://github.com/rancher/charts",
				remoteURL:   "https://github.com/rancher/charts",
				repo:        "charts",
			},
			want: false,
		},
		{
			name: "#4 - Fail",
			args: args{
				upstreamURL: "https://git.mirror.io/rancher/charts",
				remoteURL:   "https://github.com/user/charts",
				repo:        "charts",
			},
			want: false,
		},
		{
			name: "#5 - Fail",
			args: args{
				upstreamURL: "https://github.com/rancher/charts",
				remoteURL:   "https://github.com/user/charts",
				repo:        "rancher",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckForValidForkRemote(tt.args.upstreamURL, tt.args.remoteURL, tt.args.repo); got != tt.want {
				t.Errorf("CheckForValidForkRemote() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_extractCommonParts(t *testing.T) {
	type args struct {
		url1 string
		url2 string
	}
	tests := []struct {
		name       string
		args       args
		wantPrefix string
		wantSuffix string
	}{
		{
			name: "#1 - Success",
			args: args{
				url1: "https://github.com/rancher/charts",
				url2: "https://github.com/user/charts",
			},
			wantPrefix: "https://github.com",
			wantSuffix: "charts",
		},
		{
			name: "#2 - Fail",
			args: args{
				url1: "https://github.com/rancher/charts",
				url2: "https://github.com/user/chartz",
			},
			wantPrefix: "https://github.com",
			wantSuffix: "",
		},
		{
			name: "#3 - Fail",
			args: args{
				url1: "https://gitlab.com/rancher/charts",
				url2: "https://github.com/user/charts",
			},
			wantPrefix: "https:/",
			wantSuffix: "charts",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPrefix, gotSuffix := extractCommonParts(tt.args.url1, tt.args.url2)
			if gotPrefix != tt.wantPrefix {
				t.Errorf("extractCommonParts() gotPrefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
			if gotSuffix != tt.wantSuffix {
				t.Errorf("extractCommonParts() gotSuffix = %v, want %v", gotSuffix, tt.wantSuffix)
			}
		})
	}
}
