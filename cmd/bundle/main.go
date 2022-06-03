package main

import (
	"fmt"
	"os"

	"k8s.io/client-go/kubernetes/scheme"
	"kubegems.io/bundle-controller/cmd/bundle/apps"
	bundlev1 "kubegems.io/bundle-controller/pkg/apis/bundle/v1beta1"
)

const ErrExitCode = 1

func init() {
	bundlev1.AddToScheme(scheme.Scheme)
}

func main() {
	if err := apps.NewBundleControllerCmd().Execute(); err != nil {
		fmt.Println(err.Error())
		os.Exit(ErrExitCode)
	}
}
