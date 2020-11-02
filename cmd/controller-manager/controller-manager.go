package main

import (
	"context"
	"flag"
	"fmt"
	"log"
        "os"
        "os/signal"
        "syscall"


	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	bucketcontroller "github.com/container-object-storage-interface/api/controller"
	"github.com/container-object-storage-interface/cosi-controller-manager/pkg/bucketrequest"
	"github.com/container-object-storage-interface/cosi-controller-manager/pkg/bucketaccessrequest"

	"github.com/golang/glog"
)

var ctx context.Context
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
	viper.AutomaticEnv()

	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)
	flag.Set("logtostderr", "true")

	strFlag := func(c *cobra.Command, ptr *string, name string, short string, dfault string, desc string) {
		c.PersistentFlags().
			StringVarP(ptr, name, short, dfault, desc)
	}
	strFlag(cmd, &kubeConfig, "kube-config", "", kubeConfig, "path to kubeconfig file")

	hideFlag := func(name string) {
		cmd.PersistentFlags().MarkHidden(name)
	}
	hideFlag("alsologtostderr")
	hideFlag("log_backtrace_at")
	hideFlag("log_dir")
	hideFlag("logtostderr")
	hideFlag("master")
	hideFlag("stderrthreshold")
	hideFlag("vmodule")

	// suppress the incorrect prefix in glog output
	flag.CommandLine.Parse([]string{})
	viper.BindPFlags(cmd.PersistentFlags())

	var cancel context.CancelFunc

	ctx, cancel = context.WithCancel(context.Background())
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGSEGV)

	go func() {
		s := <-sigs
		cancel()
		panic(fmt.Sprintf("%s %s", s.String(), "Signal received. Exiting"))
	}()
}

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err.Error())
	}
}

func run(args []string) error {
	ctrl, err := bucketcontroller.NewDefaultObjectStorageController("controller-manager", "leader-lock", 40)
	if err != nil {
		glog.Error(err)
		return err
	}
	ctrl.AddBucketRequestListener(bucketrequest.NewListener())
	ctrl.AddBucketAccessRequestListener(bucketaccessrequest.NewListener())
	return ctrl.Run(ctx)
}
