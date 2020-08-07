package main

import (
	"log"
	"os"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/container-object-storage-interface/api/clientset"
	cosiinformers "github.com/container-object-storage-interface/api/informers/externalversions"
)

func main() {
	kubeConfig := ""
	if len(os.Args) > 1 {
		kubeConfig = os.Args[1]
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
	informer := factory.Cosi().V1alpha1().Buckets().Informer()

	stopper := make(chan struct{})
	defer close(stopper)

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		//AddFunc:    onAdd,
		//		DeleteFunc: onDelete,
		//UpdateFunc: onUpdate,
	})

	informer.Run(stopper)
}
