package bucketrequest

import (
	"context"
	"fmt"

	"k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/container-object-storage-interface/api/apis/objectstorage.k8s.io/v1alpha1"
	bucketclientset "github.com/container-object-storage-interface/api/clientset"
	bucketcontroller "github.com/container-object-storage-interface/api/controller"
	"github.com/container-object-storage-interface/cosi-controller-manager/pkg/util"
	kubeclientset "k8s.io/client-go/kubernetes"

	"github.com/golang/glog"
)

type bucketRequestListener struct {
	kubeClient   kubeclientset.Interface
	bucketClient bucketclientset.Interface
}

func NewListener() bucketcontroller.BucketRequestListener {
	return &bucketRequestListener{}
}

func (b *bucketRequestListener) InitializeKubeClient(k kubeclientset.Interface) {
	b.kubeClient = k
}

func (b *bucketRequestListener) InitializeBucketClient(bc bucketclientset.Interface) {
	b.bucketClient = bc
}

func (b *bucketRequestListener) Add(ctx context.Context, obj *v1alpha1.BucketRequest) error {
	glog.V(1).Infof("add called for bucket %s", obj.Name)
	bucketRequest := obj
	status, err := b.provisionBucketRequestOperation(ctx, bucketRequest)
	if err == nil || status == "Finished" {
		// Provisioning is 100% finished / not in progress.
		switch err {
		case nil:
			glog.V(5).Infof("BucketRequest processing succeeded, removing bucketRequest %s from bucketRequests in progress", bucketRequest.UID)
		case util.ErrStopProvision:
			glog.V(5).Infof("Stop provisioning, removing bucketRequest %s from bucketRequests in progress", bucketRequest.UID)
			// Our caller would requeue if we pass on this special error; return nil instead.
			err = nil
		default:
			glog.V(2).Infof("Final error received, removing buckerRequest %s from bucketRequests in progress", bucketRequest.UID)
		}
		return err
	}
	if status == "InBackground" {
		//nothing for now
	} else {
		// status == ProvisioningNoChange.
		// Don't change bucketRequestsInProgress:
		// - the bucketRequest is already there if previous status was ProvisioningInBackground.
		// - the bucketRequest is not there if if previous status was "Finished".
	}
	return nil
}

func (b *bucketRequestListener) Update(ctx context.Context, old, new *v1alpha1.BucketRequest) error {
	glog.V(1).Infof("add called for bucket %s", old)
	return nil
}

func (b *bucketRequestListener) Delete(ctx context.Context, obj *v1alpha1.BucketRequest) error {
	return nil
}

// provisionBucketRequestOperation attempts to provision a bucket for the given bucketRequest.
// Returns nil error only when the bucket was provisioned (in which case it also returns "Finished"),
// a normal error when the bucket was not provisioned and provisioning should be retried (requeue the bucketRequest),
// or the special errStopProvision when provisioning was impossible and no further attempts to provision should be tried.
func (b *bucketRequestListener) provisionBucketRequestOperation(ctx context.Context, bucketRequest *v1alpha1.BucketRequest) (string, error) {
	// Most code here is identical to that found in controller.go of kube's  controller...
	bucketClassName := b.GetBucketClass(bucketRequest)

	//  A previous doProvisionBucketRequest may just have finished while we were waiting for
	//  the locks. Check that bucket (with deterministic name) hasn't been provisioned
	//  yet.
	bucketName := bucketRequest.Name

	bucket, err := b.bucketClient.ObjectstorageV1alpha1().Buckets().Get(ctx, bucketName, metav1.GetOptions{})
	//bucket, err := cosiclientset.NewForConfigOrDie(config).CosiV1alpha1().Buckets().Get(ctx, bucketName, metav1.GetOptions{})
	if err == nil && bucket != nil {
		// bucket has been already provisioned, nothing to do.
		return "Finished", util.ErrStopProvision
	}

	// Prepare a bucketRequestRef to the bucketRequest early (to fail before a bucket is
	// provisioned)
	/*bucketRequestRef, err := ref.GetReference(scheme.Scheme, bucketRequest)
	  if err != nil {
	          glog.Error(logOperation(operation, "unexpected error getting bucketRequest reference: %v", err))
	          return ProvisioningNoChange, err
	  }
	*/

	bucketClass, err := b.bucketClient.ObjectstorageV1alpha1().BucketClasses().Get(ctx, bucketClassName, metav1.GetOptions{})
	if bucketClass == nil {
		// bucket has been already provisioned, nothing to do.
		return "InvalidBucketClass", util.ErrStopProvision
	}

	//ctrl.eventRecorder.Event(bucketRequest, v1.EventTypeNormal, "Provisioning", fmt.Sprintf("External provisioner is provisioning bucket for bucketRequest %q", bucketRequestToBucketRequestKey(bucketRequest)))

	glog.Info(logOperation("created", "bucket %q provisioned", bucket))

	// Set bucketRequestRef and the bucket controller will bind and set annBoundByController for us
	// bucket.Spec.bucketRequestRef = bucketRequestRef

	bucket = &v1alpha1.Bucket{}
	bucket.Name = util.GetUUID()
	bucket.Spec.Provisioner = "testProvisioner"
	bucket.Spec.ReleasePolicy = bucketClass.ReleasePolicy
	bucket.Spec.AnonymousAccessMode = v1alpha1.AnonymousAccessMode{PublicReadWrite: true}
	bucket.Spec.BucketClassName = bucketClass.Name
	bucket.Spec.AllowedNamespaces = util.CopyStrings(bucketClass.AllowedNamespaces) //could use k8s util/slice
	bucket.Spec.BucketAccessBindings = []string{}
	// TODO have a switch statement to populate appropriate protocol based on BR.Protocol
	bucket.Spec.Protocol = v1alpha1.Protocol{ProtocolSignature: v1alpha1.ProtocolSignatureS3, S3: &v1alpha1.S3Protocol{Endpoint: "aws.com/s3", BucketName: "testbucket", Region: "US", SignatureVersion: "s3v2"}}
	bucket.Spec.Parameters = util.CopySS(bucketClass.Parameters) //could use k8s util/maps

	bucket, err = b.bucketClient.ObjectstorageV1alpha1().Buckets().Create(context.Background(), bucket, metav1.CreateOptions{})
	if err != nil {
	}
	if err == nil || apierrs.IsAlreadyExists(err) {
		glog.V(5).Infof("Bucket %s saved", bucket.Name)
		return "exists", nil
	}

	glog.Info(logOperation("Finished", "succeeded"))
	return "Finished", nil
}

// GetBuckerRequestClass returns StorageClassName. If no storage class was
// requested, it returns "".
func (b *bucketRequestListener) GetBucketClass(bucketRequest *v1alpha1.BucketRequest) string {
	// Use beta annotation first
	if class, found := bucketRequest.Annotations[v1.BetaStorageClassAnnotation]; found {
		return class
	}

	if bucketRequest.Spec.BucketClassName != "" {
		return bucketRequest.Spec.BucketClassName
	}

	return ""
}

func (b *bucketRequestListener) cloneTheBucket(bucketRequest *v1alpha1.BucketRequest) error {
	glog.V(1).Infof("clone called for bucket %s", bucketRequest.Spec.BucketInstanceName)
	return util.ErrNotImplemented
}

func logOperation(operation, format string, a ...interface{}) string {
	return fmt.Sprintf(fmt.Sprintf("%s: %s", operation, format), a...)
}

func bucketRequestToBucketRequestKey(bucketRequest *v1alpha1.BucketRequest) string {
	return fmt.Sprintf("%s/%s", bucketRequest.Namespace, bucketRequest.Name)
}
