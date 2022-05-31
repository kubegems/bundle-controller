package utils

import (
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetReference(obj client.Object) corev1.ObjectReference {
	return corev1.ObjectReference{
		APIVersion: obj.GetObjectKind().GroupVersionKind().GroupVersion().String(),
		Kind:       obj.GetObjectKind().GroupVersionKind().Kind,
		Namespace:  obj.GetNamespace(),
		Name:       obj.GetName(),
	}
}
