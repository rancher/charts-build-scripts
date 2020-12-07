module github.com/rancher/charts-build-scripts

go 1.13

replace (
	// Needed to resolve issue on Darwin machines
	// https://github.com/ory/dockertest/issues/208#issuecomment-686820414
	golang.org/x/sys => golang.org/x/sys v0.0.0-20200826173525-f9321e4c35a6
	k8s.io/client-go => k8s.io/client-go v0.19.0
)

require (
	github.com/go-git/go-billy/v5 v5.0.0
	github.com/go-git/go-git/v5 v5.2.0
	github.com/google/go-github v17.0.0+incompatible
	github.com/google/go-querystring v1.0.0 // indirect
	github.com/sirupsen/logrus v1.7.0
	github.com/urfave/cli v1.22.2
	golang.org/x/oauth2 v0.0.0-20201109201403-9fd604954f58
	golang.org/x/sys v0.0.0-20201015000850-e3ed0017c211 // indirect
	gopkg.in/yaml.v2 v2.3.0
	helm.sh/helm/v3 v3.4.1
	rsc.io/letsencrypt v0.0.3 // indirect
)
