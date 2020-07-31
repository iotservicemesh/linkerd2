package k8s

import (
	"errors"
	"fmt"

	authV1 "k8s.io/api/authorization/v1"
	discovery "k8s.io/api/discovery/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
)

// ResourceAuthz checks whether a given Kubernetes client is authorized to
// perform a given action.
func ResourceAuthz(
	k8sClient kubernetes.Interface,
	namespace, verb, group, version, resource, name string,
) error {
	ssar := &authV1.SelfSubjectAccessReview{
		Spec: authV1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authV1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     group,
				Version:   version,
				Resource:  resource,
				Name:      name,
			},
		},
	}

	result, err := k8sClient.
		AuthorizationV1().
		SelfSubjectAccessReviews().
		Create(ssar)
	if err != nil {
		return err
	}

	return evaluateAccessReviewStatus(group, resource, result.Status)
}

// ResourceAuthzForUser checks whether a given user is authorized to perform a
// given action.
func ResourceAuthzForUser(
	client kubernetes.Interface,
	namespace, verb, group, version, resource, subresource, name, user string, userGroups []string) error {
	sar := &authV1.SubjectAccessReview{
		Spec: authV1.SubjectAccessReviewSpec{
			User:   user,
			Groups: userGroups,
			ResourceAttributes: &authV1.ResourceAttributes{
				Namespace:   namespace,
				Verb:        verb,
				Group:       group,
				Version:     version,
				Resource:    resource,
				Subresource: subresource,
				Name:        name,
			},
		},
	}

	result, err := client.
		AuthorizationV1().
		SubjectAccessReviews().
		Create(sar)
	if err != nil {
		return err
	}

	return evaluateAccessReviewStatus(group, resource, result.Status)
}

func evaluateAccessReviewStatus(group, resource string, status authV1.SubjectAccessReviewStatus) error {
	if status.Allowed {
		return nil
	}

	gk := schema.GroupKind{
		Group: group,
		Kind:  resource,
	}
	if len(status.Reason) > 0 {
		return fmt.Errorf("not authorized to access %s: %s", gk, status.Reason)
	}
	return fmt.Errorf("not authorized to access %s", gk)
}

// ServiceProfilesAccess checks whether the ServiceProfile CRD is installed
// on the cluster and the client is authorized to access ServiceProfiles.
func ServiceProfilesAccess(k8sClient kubernetes.Interface) error {
	res, err := k8sClient.Discovery().ServerResourcesForGroupVersion(ServiceProfileAPIVersion)
	if err != nil {
		return err
	}

	if res.GroupVersion == ServiceProfileAPIVersion {
		for _, apiRes := range res.APIResources {
			if apiRes.Kind == ServiceProfileKind {
				return ResourceAuthz(k8sClient, "", "list", "linkerd.io", "", "serviceprofiles", "")
			}
		}
	}

	return errors.New("ServiceProfile CRD not found")
}

// EndpointSliceAccess verifies whether the K8s cluster has
// access to EndpointSlice resources.
func EndpointSliceAccess(k8sClient kubernetes.Interface) error {
	gv := discovery.SchemeGroupVersion.String()
	res, err := k8sClient.Discovery().ServerResourcesForGroupVersion(gv)
	if err != nil {
		return err
	}

	if res.GroupVersion == gv {
		for _, apiRes := range res.APIResources {
			if apiRes.Kind == "EndpointSlice" {
				return checkEndpointSlicesExist(k8sClient)
			}
		}
	}

	return errors.New("EndpointSlice resource not found")
}

func checkEndpointSlicesExist(k8sClient kubernetes.Interface) error {
	sliceList, err := k8sClient.DiscoveryV1beta1().EndpointSlices("").List(metav1.ListOptions{})
	if err != nil {
		return err
	}

	if len(sliceList.Items) > 0 {
		return nil
	}

	return errors.New("no EndpointSlice resources exist in the cluster")
}

// LinkAccess checks whether the Link CRD is installed on the cluster and the
// client is authorized to access Links.
func LinkAccess(k8sClient kubernetes.Interface) error {
	res, err := k8sClient.Discovery().ServerResourcesForGroupVersion(LinkAPIGroupVersion)
	if err != nil {
		return err
	}

	if res.GroupVersion == LinkAPIGroupVersion {
		for _, apiRes := range res.APIResources {
			if apiRes.Kind == LinkKind {
				return ResourceAuthz(k8sClient, "", "list", LinkAPIGroup, LinkAPIVersion, "links", "")
			}
		}
	}

	return errors.New("Link CRD not found")
}

// ClusterAccess verifies whether k8sClient is authorized to access all pods in
// all namespaces in the cluster.
func ClusterAccess(k8sClient kubernetes.Interface) error {
	return ResourceAuthz(k8sClient, "", "list", "", "", "pods", "")
}
