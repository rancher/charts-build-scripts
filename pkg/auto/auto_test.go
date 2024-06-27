package auto

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/rancher/charts-build-scripts/pkg/git"
	"github.com/rancher/charts-build-scripts/pkg/lifecycle"
)

func Test_createForwardPortCommands(t *testing.T) {
	type fields struct {
		git                     *git.Git
		VR                      *lifecycle.VersionRules
		assetsToBeForwardPorted map[string][]lifecycle.Asset
	}

	type args struct {
		chart string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []Command
		wantErr error
	}{
		// #1 Success Complex Test - SUCCESS
		{
			name: "#1 Success",
			fields: fields{
				git: &git.Git{
					Remotes: map[string]string{
						"https://github.com/rancher/charts": "upstream",
					},
				},
				VR: &lifecycle.VersionRules{
					DevBranch: "dev-v2.9",
				},
				assetsToBeForwardPorted: map[string][]lifecycle.Asset{
					"fleet": {
						{Version: "104.0.0+up.0.0.0"},
						{Version: "103.0.0+up.0.0.0"},
						{Version: "104.0.0+up.0.0.1"},
					},
					"fleet-crd": {
						{Version: "104.0.0+up.0.0.0"},
						{Version: "103.0.0+up.0.0.0"},
						{Version: "104.0.0+up.0.0.1"},
					},
					"fleet-agent": {
						{Version: "104.0.0+up.0.0.0"},
						{Version: "103.0.0+up.0.0.0"},
						{Version: "104.0.0+up.0.0.1"},
					},
					"rancher-turtles": {
						{Version: "103.0.0+up.0.0.0"},
						{Version: "104.0.0+up.0.0.1"},
					},
				},
			},
			args: args{
				chart: "",
			},
			want: []Command{
				{
					Chart:   "fleet",
					Version: "103.0.0+up.0.0.0",
				},
				{
					Chart:   "fleet",
					Version: "104.0.0+up.0.0.0",
				},
				{
					Chart:   "fleet",
					Version: "104.0.0+up.0.0.1",
				},
				{
					Chart:   "fleet-agent",
					Version: "103.0.0+up.0.0.0",
				},
				{
					Chart:   "fleet-agent",
					Version: "104.0.0+up.0.0.0",
				},
				{
					Chart:   "fleet-agent",
					Version: "104.0.0+up.0.0.1",
				},
				{
					Chart:   "fleet-crd",
					Version: "103.0.0+up.0.0.0",
				},
				{
					Chart:   "fleet-crd",
					Version: "104.0.0+up.0.0.0",
				},
				{
					Chart:   "fleet-crd",
					Version: "104.0.0+up.0.0.1",
				},
				{
					Chart:   "rancher-turtles",
					Version: "103.0.0+up.0.0.0",
				},
				{
					Chart:   "rancher-turtles",
					Version: "104.0.0+up.0.0.1",
				},
			},
			wantErr: nil,
		},
		// #2 No Version Test - SUCCESS
		{
			name: "#1 Success",
			fields: fields{
				git: &git.Git{
					Remotes: map[string]string{
						"https://github.com/rancher/charts": "upstream",
					},
				},
				VR: &lifecycle.VersionRules{
					DevBranch: "dev-v2.9",
				},
				assetsToBeForwardPorted: map[string][]lifecycle.Asset{},
			},
			args: args{
				chart: "",
			},
			want:    []Command{},
			wantErr: nil,
		},
		// #3 Only 1 asset version Test - SUCCESS
		{
			name: "#1 Success",
			fields: fields{
				git: &git.Git{
					Remotes: map[string]string{
						"https://github.com/rancher/charts": "upstream",
					},
				},
				VR: &lifecycle.VersionRules{
					DevBranch: "dev-v2.9",
				},
				assetsToBeForwardPorted: map[string][]lifecycle.Asset{
					"rancher-turtles": {
						{Version: "103.0.0+up.0.0.0"},
					},
				},
			},
			args: args{
				chart: "",
			},
			want: []Command{
				{
					Chart:   "rancher-turtles",
					Version: "103.0.0+up.0.0.0",
				},
			},
			wantErr: nil,
		},
		// #4 Filter chart Test - SUCCESS
		{
			name: "#1 Success",
			fields: fields{
				git: &git.Git{
					Remotes: map[string]string{
						"https://github.com/rancher/charts": "upstream",
					},
				},
				VR: &lifecycle.VersionRules{
					DevBranch: "dev-v2.9",
				},
				assetsToBeForwardPorted: map[string][]lifecycle.Asset{
					"rancher-foxes": {
						{Version: "103.0.0+up.0.0.0"},
						{Version: "102.0.0+up.0.0.0"},
					},
					"rancher-turtles": {
						{Version: "103.0.0+up.0.0.0"},
						{Version: "102.0.0+up.0.0.0"},
					},
				},
			},
			args: args{
				chart: "rancher-turtles",
			},
			want: []Command{
				{
					Chart:   "rancher-turtles",
					Version: "102.0.0+up.0.0.0",
				},
				{
					Chart:   "rancher-turtles",
					Version: "103.0.0+up.0.0.0",
				},
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := &ForwardPort{
				git:                     tt.fields.git,
				VR:                      tt.fields.VR,
				assetsToBeForwardPorted: tt.fields.assetsToBeForwardPorted,
			}

			got, err := fp.createForwardPortCommands(tt.args.chart)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("createForwardPortCommands() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if len(got) != len(tt.want) {
				t.Errorf("createForwardPortCommands() got %d commands, want %d commands", len(got), len(tt.want))
			}
			for i, gotCmd := range got {
				if i >= len(tt.want) {
					t.Errorf("Extra command in got: %v", gotCmd)
					break
				}
				wantCmd := tt.want[i]
				if gotCmd.Chart != wantCmd.Chart || gotCmd.Version != wantCmd.Version {
					t.Errorf("createForwardPortCommands() command at index %d = %v, want %v", i, gotCmd, wantCmd)
				}
			}
		})
	}
}

func Test_writeMakeCommands(t *testing.T) {
	type fields struct {
		git *git.Git
		VR  *lifecycle.VersionRules
	}

	type args struct {
		asset   string
		version string
	}

	tests := []struct {
		name    string
		fields  fields
		args    args
		want    Command
		wantErr error
	}{
		// #1 Success Test
		{
			name: "#1 make forward-port rancher-istio 103.0.0+up0.0.1 BRANCH=dev-v2.9 UPSTREAM=upstream",
			fields: fields{
				git: &git.Git{
					Remotes: map[string]string{
						"https://github.com/rancher/charts": "upstream",
					},
				},
				VR: &lifecycle.VersionRules{
					DevBranch: "dev-v2.9",
				},
			},
			args: args{
				asset: "rancher-istio", version: "103.0.0+up0.0.1",
			},
			want: Command{
				Chart:   "rancher-istio",
				Version: "103.0.0+up0.0.1",
				Command: []string{
					"make", "forward-port",
					"CHART=rancher-istio", "VERSION=103.0.0+up0.0.1",
					"BRANCH=dev-v2.9", "UPSTREAM=upstream",
				},
			},
			wantErr: nil,
		},
		// #2 Success Test
		{
			name: "#1 make forward-port rancher-istio 103.0.0+up0.0.1 BRANCH=dev-v2.9 UPSTREAM=origin",
			fields: fields{
				git: &git.Git{
					Remotes: map[string]string{
						"https://github.com/rancher/charts": "origin",
					},
				},
				VR: &lifecycle.VersionRules{
					DevBranch: "dev-v2.9",
				},
			},
			args: args{
				asset: "rancher-istio", version: "103.0.0+up0.0.1",
			},
			want: Command{
				Chart:   "rancher-istio",
				Version: "103.0.0+up0.0.1",
				Command: []string{
					"make", "forward-port",
					"CHART=rancher-istio", "VERSION=103.0.0+up0.0.1",
					"BRANCH=dev-v2.9", "UPSTREAM=origin",
				},
			},
			wantErr: nil,
		},
		// #3 Failure Test
		{
			name: "#1 make forward-port rancher-istio 103.0.0+up0.0.1 BRANCH=dev-v2.9 UPSTREAM=origin",
			fields: fields{
				git: &git.Git{
					Remotes: map[string]string{
						"https://github.com/someUser/charts": "origin",
					},
				},
				VR: &lifecycle.VersionRules{
					DevBranch: "dev-v2.9",
				},
			},
			args: args{
				asset: "rancher-istio", version: "103.0.0+up0.0.1",
			},
			want: Command{
				Chart:   "rancher-istio",
				Version: "103.0.0+up0.0.1",
				Command: []string{
					"make", "forward-port",
					"CHART=rancher-istio", "VERSION=103.0.0+up0.0.1",
					"BRANCH=dev-v2.9", "UPSTREAM=origin",
				},
			},
			wantErr: fmt.Errorf("upstream remote not found; you need to have the upstream remote configured in your git repository (https://github.com/rancher/charts)"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fp := &ForwardPort{
				git: tt.fields.git,
				VR:  tt.fields.VR,
			}

			got, err := fp.writeMakeCommand(tt.args.asset, tt.args.version)
			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("writeMakeCommand() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if got.Chart != tt.want.Chart {
				t.Errorf("writeMakeCommand() = %v, want %v", got, tt.want)
			}
			if got.Version != tt.want.Version {
				t.Errorf("writeMakeCommand() = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got.Command, tt.want.Command) {
				t.Errorf("writeMakeCommand() = %v, want %v", got.Command, tt.want.Command)
			}
		})
	}
}

func Test_checkIfChartChanged(t *testing.T) {

	type args struct {
		lastChart string
		chart     string
	}

	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "#1 Same Charts",
			args: args{
				lastChart: "rancher-istio",
				chart:     "rancher-istio",
			},
			want: false,
		},
		{
			name: "#1.1 Same Charts but CRD",
			args: args{
				lastChart: "rancher-istio",
				chart:     "rancher-istio-crd",
			},
			want: false,
		},
		{
			name: "#2.1 Different Charts",
			args: args{
				lastChart: "rancher-istio",
				chart:     "rancher-kiali",
			},
			want: true,
		},
		{
			name: "#2.2 Different Charts but CRD",
			args: args{
				lastChart: "rancher-istio",
				chart:     "rancher-kiali-crd",
			},
			want: true,
		},
		{
			name: "#3.1 Special Case: fleet - same charts",
			args: args{
				lastChart: "fleet",
				chart:     "fleet",
			},
			want: false,
		},
		{
			name: "#3.2 Special Case: fleet - same charts but CRD",
			args: args{
				lastChart: "fleet",
				chart:     "fleet-crd",
			},
			want: false,
		},
		{
			name: "#3.3 Special Case: fleet - same charts but agent",
			args: args{
				lastChart: "fleet",
				chart:     "fleet-agent",
			},
			want: false,
		},
		{
			name: "#3.4 Special Case: fleet - CRD and agent",
			args: args{
				lastChart: "fleet",
				chart:     "fleet-agent",
			},
			want: false,
		},
		{
			name: "#4.1 Special Case: neuvector - same charts",
			args: args{
				lastChart: "neuvector",
				chart:     "neuvector",
			},
			want: false,
		},
		{
			name: "#4.2 Special Case: neuvector - same charts but CRD",
			args: args{
				lastChart: "neuvector",
				chart:     "neuvector-crd",
			},
			want: false,
		},
		{
			name: "#4.3 Special Case: neuvector - same charts but monitor",
			args: args{
				lastChart: "neuvector",
				chart:     "neuvector-monitor",
			},
			want: false,
		},
		{
			name: "#4.4 Special Case: neuvector - CRD and monitor",
			args: args{
				lastChart: "neuvector",
				chart:     "neuvector-monitor",
			},
			want: false,
		},
		{
			name: "#5.1 Special Edge Cases: fleet and neuvector",
			args: args{
				lastChart: "fleet",
				chart:     "neuvector",
			},
			want: true,
		},
		{
			name: "#5.2 Special Edge Cases: fleet-crd and neuvector",
			args: args{
				lastChart: "fleet-crd",
				chart:     "neuvector",
			},
			want: true,
		},
		{
			name: "#5.3 Special Edge Cases: fleet-crd and neuvector-monitor",
			args: args{
				lastChart: "fleet-crd",
				chart:     "neuvector-monitor",
			},
			want: true,
		},
	}

	// Execute tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkIfChartChanged(tt.args.lastChart, tt.args.chart); got != tt.want {
				t.Errorf("checkForChangeType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_checkSpecialChartNamingCases(t *testing.T) {

	type args struct {
		lastChart string
		chart     string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "#1.0 Special Edge Cases",
			args: args{
				lastChart: "fleet-crd",
				chart:     "fleet",
			},
			want: "fleet",
		},
		{
			name: "#1.1 Special Edge Cases",
			args: args{
				lastChart: "fleet",
				chart:     "fleet-crd",
			},
			want: "fleet",
		},
		{
			name: "#1.2 Special Edge Cases",
			args: args{
				lastChart: "fleet-agent",
				chart:     "fleet-crd",
			},
			want: "fleet",
		},
		{
			name: "#1.3 Special Edge Cases",
			args: args{
				lastChart: "fleet-crd",
				chart:     "fleet-agent",
			},
			want: "fleet",
		},
		{
			name: "#2.0 Special Edge Cases",
			args: args{
				lastChart: "neuvector",
				chart:     "neuvector-crd",
			},
			want: "neuvector",
		},
		{
			name: "#2.1 Special Edge Cases",
			args: args{
				lastChart: "neuvector-monitor",
				chart:     "neuvector",
			},
			want: "neuvector",
		},
		{
			name: "#2.2 Special Edge Cases",
			args: args{
				lastChart: "neuvector-monitor",
				chart:     "neuvector-crd",
			},
			want: "neuvector",
		},
		{
			name: "#2.3 Special Edge Cases",
			args: args{
				lastChart: "neuvector-crd",
				chart:     "neuvector-monitor",
			},
			want: "neuvector",
		},
		{
			name: "#3.0 Special Edge Cases",
			args: args{
				lastChart: "rancher-aks-operator",
				chart:     "rancher-eks-operator",
			},
			want: "",
		},
		{
			name: "#3.1 Special Edge Cases",
			args: args{
				lastChart: "rancher-aks-operator",
				chart:     "rancher-aks-operator-crd",
			},
			want: "rancher-aks-operator",
		},
		{
			name: "#3.2 Special Edge Cases",
			args: args{
				lastChart: "rancher-gke-operator-crd",
				chart:     "rancher-gke-operator",
			},
			want: "rancher-gke-operator",
		},
		{
			name: "#4.0 Special Edge Cases",
			args: args{
				lastChart: "rancher-neuvector",
				chart:     "neuvector-rancher",
			},
			want: "",
		},
	}

	// Execute tests
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := checkChartCommonPrefixes(tt.args.lastChart, tt.args.chart)
			if got != tt.want {
				t.Errorf("checkChartCommonPrefixes() = %v, want %v", got, tt.want)
			}
		})
	}
}
