package server

import (
	"context"
	"testing"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	networkingv1 "k8s.io/api/networking/v1"
)

func TestServerReconciler_createUpdateAndDeleteServerIngress(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	setTestResourceNameAndLabels(sr)

	nginx := "nginx"

	// configure ingress resource in ArgoCD
	sr.Instance.Spec.Server = argoproj.ArgoCDServerSpec{
		Ingress: argoproj.ArgoCDIngressSpec{
			Enabled:          true,
			IngressClassName: &nginx,
		},
	}

	err := sr.reconcileServerIngress()
	assert.NoError(t, err)

	// ingress resource should be created
	ingress := &networkingv1.Ingress{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: "argocd"}, ingress)
	assert.NoError(t, err)
	assert.Equal(t, &nginx, ingress.Spec.IngressClassName)

	// modify ingress resource in ArgoCD
	var nilClass *string = nil
	ann := map[string]string{"example.com": "test"}
	sr.Instance.Spec.Server.Ingress.IngressClassName = nilClass
	sr.Instance.Spec.Server.Ingress.Annotations = ann

	err = sr.reconcileServerIngress()
	assert.NoError(t, err)

	// ingress resource should be updated
	ingress = &networkingv1.Ingress{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: "argocd"}, ingress)
	assert.NoError(t, err)
	assert.Equal(t, nilClass, ingress.Spec.IngressClassName)
	assert.Equal(t, ann, ingress.ObjectMeta.Annotations)

	// disable ingress in ArgoCD
	sr.Instance.Spec.Server.Ingress.Enabled = false
	err = sr.reconcileServerIngress()
	assert.NoError(t, err)

	// ingress resource should be deleted
	ingress = &networkingv1.Ingress{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server", Namespace: "argocd"}, ingress)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestServerReconciler_createUpdateAndDeleteServerGRPCIngress(t *testing.T) {
	ns := argocdcommon.MakeTestNamespace()
	sr := makeTestServerReconciler(t, ns)
	setTestResourceNameAndLabels(sr)

	nginx := "nginx"

	// configure grpc ingress resource in ArgoCD
	sr.Instance.Spec.Server.GRPC.Ingress = argoproj.ArgoCDIngressSpec{
		Enabled:          true,
		IngressClassName: &nginx,
	}

	err := sr.reconcileServerGRPCIngress()
	assert.NoError(t, err)

	// ingress resource should be created
	ingress := &networkingv1.Ingress{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server-grpc", Namespace: "argocd"}, ingress)
	assert.NoError(t, err)
	assert.Equal(t, &nginx, ingress.Spec.IngressClassName)

	// modify grpc ingress resource in ArgoCD
	var nilClass *string = nil
	ann := map[string]string{"example.com": "test"}
	sr.Instance.Spec.Server.GRPC.Ingress.IngressClassName = nilClass
	sr.Instance.Spec.Server.GRPC.Ingress.Annotations = ann

	err = sr.reconcileServerGRPCIngress()
	assert.NoError(t, err)

	// ingress resource should be updated
	ingress = &networkingv1.Ingress{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server-grpc", Namespace: "argocd"}, ingress)
	assert.NoError(t, err)
	assert.Equal(t, nilClass, ingress.Spec.IngressClassName)
	assert.Equal(t, ann, ingress.ObjectMeta.Annotations)

	// disable grpc ingress in ArgoCD
	sr.Instance.Spec.Server.GRPC.Ingress.Enabled = false
	err = sr.reconcileServerGRPCIngress()
	assert.NoError(t, err)

	// ingress resource should be deleted
	ingress = &networkingv1.Ingress{}
	err = sr.Client.Get(context.TODO(), types.NamespacedName{Name: "argocd-argocd-server-grpc", Namespace: "argocd"}, ingress)
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}
