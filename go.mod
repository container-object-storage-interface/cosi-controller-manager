module github.com/container-object-storage-interface/cosi-controller-manager

go 1.14

require (
	github.com/container-object-storage-interface/api v0.0.0-20200924013804-9290d9c83ae2
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/spf13/cobra v0.0.5
	github.com/spf13/viper v1.3.2
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.4
	k8s.io/client-go v0.18.4
	k8s.io/klog v1.0.0
)

replace github.com/container-object-storage-interface/api => ../api
