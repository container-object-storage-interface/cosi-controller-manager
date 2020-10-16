package bucketaccessrequest

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

type bucketAccessRequestListener struct {
	kubeClient   kubeclientset.Interface
	bucketClient bucketclientset.Interface
}

func NewListener() bucketcontroller.BucketAccessRequestListener {
	return &bucketAccessRequestListener{}
}

func (b *bucketAccessRequestListener) InitializeKubeClient(k kubeclientset.Interface) {
	b.kubeClient = k
}

func (b *bucketAccessRequestListener) InitializeBucketClient(bc bucketclientset.Interface) {
	b.bucketClient = bc
}

func (b *bucketAccessRequestListener) Add(ctx context.Context, obj *v1alpha1.BucketAccessRequest) error {
	glog.V(1).Infof("add called for bucketaccess %s", obj.Name)
	bucketAccessRequest := obj

	status, err := b.provisionBucketAccess(ctx, bucketAccessRequest)
	if err == nil || status == "Finished" {
		// Provisioning is 100% finished / not in progress.
		switch err {
		case nil:
			glog.V(5).Infof("BucketAccessRequest processing succeeded, removing bucketAccessRequest %s from bucketAccessRequests in progress", bucketAccessRequest.UID)
		case util.ErrStopProvision:
			glog.V(5).Infof("Stop provisioning, removing bucketAccessRequest %s from bucketAccessRequests in progress", bucketAccessRequest.UID)
			// Our caller would requeue if we pass on this special error; return nil instead.
			err = nil
		default:
			glog.V(2).Infof("Final error received, removing buckerRequest %s from bucketAccessRequests in progress", bucketAccessRequest.UID)
		}
		return err
	}
	if status == "InBackground" {
		//nothing for now
	} else {
		// status == ProvisioningNoChange.
		// Don't change bucketAccessRequestsInProgress:
		// - the bucketAccessRequest is already there if previous status was ProvisioningInBackground.
		// - the bucketAccessRequest is not there if if previous status was "Finished".
	}
	return nil
}

func (b *bucketAccessRequestListener) Update(ctx context.Context, old, new *v1alpha1.BucketAccessRequest) error {
	glog.V(1).Infof("update called for bucket %s", old)
	return nil
}

func (b *bucketAccessRequestListener) Delete(ctx context.Context, obj *v1alpha1.BucketAccessRequest) error {
	glog.V(1).Infof("delete called for bucket %s", obj)
	return nil
}

// provisionBucketAccess  attempts to provision a BucketAccess for the given bucketAccessRequest.
// Returns nil error only when the bucket was provisioned (in which case it also returns "Finished"),
// a normal error when the bucket was not provisioned and provisioning should be retried (requeue the bucketAccessRequest),
// or the special errStopProvision when provisioning was impossible and no further attempts to provision should be tried.
func (b *bucketAccessRequestListener) provisionBucketAccess(ctx context.Context, bucketAccessRequest *v1alpha1.BucketAccessRequest) (string, error) {
	// Most code here is identical to that found in controller.go of kube's  controller...
	bucketAccessClassName := b.GetBucketAccessClass(bucketAccessRequest)

	//  A previous doProvisionBucketAccessRequest may just have finished while we were waiting for
	//  the locks. Check that bucket (with deterministic name) hasn't been provisioned
	//  yet.
	bucketAccessName := bucketAccessRequest.Name

	bucketaccess, err := b.bucketClient.ObjectstorageV1alpha1().BucketAccesses().Get(ctx, bucketAccessName, metav1.GetOptions{})
	if err == nil && bucketaccess != nil {
		// bucketaccess has been already provisioned, nothing to do.
		return "Finished", util.ErrStopProvision
	}

	// Prepare a bucketAccessRequestRef to the bucketAccessRequest early (to fail before a bucket is
	// provisioned)
	/*bucketAccessRequestRef, err := ref.GetReference(scheme.Scheme, bucketAccessRequest)
	  if err != nil {
	          glog.Error(logOperation(operation, "unexpected error getting bucketAccessRequest reference: %v", err))
	          return ProvisioningNoChange, err
	  }
	*/

	bucketAccessClass, err := b.bucketClient.ObjectstorageV1alpha1().BucketAccessClasses().Get(ctx, bucketAccessClassName, metav1.GetOptions{})
	if bucketAccessClass == nil {
		// bucket has been already provisioned, nothing to do.
		return "InvalidBucketAccessClass", util.ErrBCUnavailable
	}

	bucketRequest, err := b.bucketClient.ObjectstorageV1alpha1().BucketRequests(bucketAccessRequest.Namespace).Get(ctx, bucketAccessRequest.Spec.BucketRequestName, metav1.GetOptions{})
	if bucketRequest == nil {
		// bucket has been already provisioned, nothing to do.
		return "InvalidBucketRequest", util.ErrStopProvision
	}
	if err != nil {
		return "InvalidBucketRequest", err
	}

	//ctrl.eventRecorder.Event(bucketAccessRequest, v1.EventTypeNormal, "Provisioning", fmt.Sprintf("External provisioner is provisioning bucket for bucketAccessRequest %q", bucketAccessRequestToBucketAccessRequestKey(bucketAccessRequest)))

	glog.Info(logOperation("create bucketaccess %q", bucketaccess.Name))

	// Set bucketAccessRequestRef and the bucket controller will bind and set annBoundByController for us
	// bucket.Spec.bucketAccessRequestRef = bucketAccessRequestRef

	bucketaccess = &v1alpha1.BucketAccess{}
	bucketaccess.Name = util.GetUUID()

        bucketaccess.Spec.BucketInstanceName  = bucketRequest.Spec.BucketInstanceName
	bucketaccess.Spec.BucketAccessRequest = bucketAccessRequest.Name
	bucketaccess.Spec.ServiceAccount   = bucketAccessRequest.Spec.ServiceAccountName
	//bucketaccess.Spec.MintedSecretName - set by the driver
	bucketaccess.Spec.PolicyActionsConfigMapData = util.ReadObject(bucketAccessClass.PolicyActionsConfigMap)
	bucketaccess.Spec.Principal = bucketAccessRequest.Namespace

	bucketaccess.Spec.Provisioner = bucketAccessClass.Provisioner
	bucketaccess.Spec.Parameters = util.CopySS(bucketAccessClass.Parameters) //could use k8s util/maps


	bucketaccess, err = b.bucketClient.ObjectstorageV1alpha1().BucketAccesses().Create(context.Background(), bucketaccess, metav1.CreateOptions{})
	if err == nil || apierrs.IsAlreadyExists(err) {
		glog.V(5).Infof("BucketAccess %s saved", bucketaccess.Name)
		return "exists", nil
	}

	glog.Info(logOperation("Finished", "succeeded"))
	return "Finished", nil
}

// GetBuckerRequestClass returns StorageClassName. If no storage class was
// requested, it returns "".
func (b *bucketAccessRequestListener) GetBucketAccessClass(bucketAccessRequest *v1alpha1.BucketAccessRequest) string {
	// Use beta annotation first
	if class, found := bucketAccessRequest.Annotations[v1.BetaStorageClassAnnotation]; found {
		return class
	}

	if bucketAccessRequest.Spec.BucketAccessClassName != "" {
		return bucketAccessRequest.Spec.BucketAccessClassName
	}

	return ""
}

func logOperation(operation, format string, a ...interface{}) string {
	return fmt.Sprintf(fmt.Sprintf("%s: %s", operation, format), a...)
}

func bucketAccessRequestToBucketAccessRequestKey(bucketAccessRequest *v1alpha1.BucketAccessRequest) string {
	return fmt.Sprintf("%s/%s", bucketAccessRequest.Namespace, bucketAccessRequest.Name)
}
