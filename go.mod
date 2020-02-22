module gitlab.com/piersharding/dask-operator

go 1.13

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.5.0 // indirect
	github.com/Masterminds/sprig v2.22.0+incompatible
	github.com/appscode/go v0.0.0-20191025021232-311ac347b3ef
	github.com/cyphar/filepath-securejoin v0.2.2 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/huandu/xstrings v1.3.0 // indirect
	github.com/mitchellh/copystructure v1.0.0 // indirect
	github.com/onsi/ginkgo v1.6.0
	github.com/onsi/gomega v1.4.2
	github.com/sirupsen/logrus v1.2.0
	gopkg.in/yaml.v3 v3.0.0-20191106092431-e228e37189d3
	k8s.io/api v0.0.0-20190918195907-bd6ac527cfd2
	k8s.io/apimachinery v0.0.0-20190817020851-f2f3a405f61d
	k8s.io/client-go v0.0.0-20190918200256-06eb1244587a
	k8s.io/helm v2.16.0+incompatible
	sigs.k8s.io/controller-runtime v0.3.0
)

replace github.com/go-check/check v1.0.0-20180628173108-788fd7840127 => github.com/go-check/check v0.0.0-20180628173108-788fd7840127
