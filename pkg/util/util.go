package util

import (
	"errors"

	"k8s.io/apimachinery/pkg/util/uuid"
	"github.com/container-object-storage-interface/api/apis/objectstorage.k8s.io/v1alpha1"
)

var (
	ErrStopProvision  = errors.New("stop provisioning")
	ErrBCUnavailable  = errors.New("BucketClass is not available")
	ErrNotImplemented = errors.New("Operation Not Implemented")
)

func CopySS(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	copy := make(map[string]string, len(m))
	for k, v := range m {
		copy[k] = v
	}
	return copy
}

func CopyStrings(s []string) []string {
	if s == nil {
		return nil
	}
	c := make([]string, len(s))
	copy(c, s)
	return c
}

func GetUUID() string {
	return string(uuid.NewUUID())
}

func ReadObject(o *v1alpha1.ObjectReference) string {
	return ""
}
