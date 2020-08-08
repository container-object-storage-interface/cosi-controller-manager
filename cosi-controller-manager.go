package main

import (
	"fmt"
	"log"
	"os"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/spf13/cobra"

	"github.com/container-object-storage-interface/api/clientset"
	cosiinformers "github.com/container-object-storage-interface/api/informers/externalversions"
	"github.com/container-object-storage-interface/cosi-controller-manager/pkg/bucketrequest"
	"github.com/container-object-storage-interface/cosi-controller-manager/pkg/bucket"
	"github.com/container-object-storage-interface/cosi-controller-manager/pkg/bucketclass"
)

var cmd = &cobra.Command{
	Use:           "cosi-controller-manager",
	Short:         "central controller for managing bucket* and bucketAccess* API objects",
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(c *cobra.Command, args []string) error {
		return run(args)
	},
	DisableFlagsInUseLine: true,
}

var kubeConfig string

func init() {
	cmd.PersistentFlags().StringVarP(&kubeConfig, "kubeconfig", "k", kubeConfig, "absolute path of the kubernetes config file")
}

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}

func run(args) erorr {
	if kubeConfig == "" {
		return fmt.Errorf("kubeConfig parameter cannot be empty")
	}

	config, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		log.Panic(err.Error())
	}

	cs, err := clientset.NewForConfig(config)
	if err != nil {
		log.Panic(err.Error())
	}

	factory := cosiinformers.NewSharedInformerFactory(cs, 0)

	bucketRequestInformer := factory.Cosi().V1alpha1().BucketRequests().Informer()
	bucketInformer := factory.Cosi().V1alpha1().Buckets().Informer()
	bucketClassInformer := factory.Cosi().V1alpha1().BucketClasses().Informer()

	stopper := make(chan struct{})
	defer close(stopper)

	bucketRequestInformer.AddEventHandler(bucketrequest.EventHandlerFuncs())
	bucketInformer.AddEventHandler(bucket.EventHandlerFuncs())
	bucketClassInformer.AddEventHandler(bucketClass.EventHandlerFunc())

	factory.Run(stopper)
	<-stopper
}
