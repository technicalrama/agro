// Copyright 2019 ArgoCD Operator Developers
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package argocd

import (
	"fmt"
	"reflect"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	argoproj "github.com/argoproj-labs/argocd-operator/api/v1beta1"
	"github.com/argoproj-labs/argocd-operator/common"

	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	v1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

const (
	grafanaDeprecatedWarning = "Warning: grafana field is deprecated from ArgoCD: field will be ignored."
)

// reconcileResources will reconcile common ArgoCD resources.
func (r *ReconcileArgoCD) reconcileResources(cr *argoproj.ArgoCD) error {

	// we reconcile SSO first so that we can catch and throw errors for any illegal SSO configurations right away, and return control from here
	// preventing dex resources from getting created anyway through the other function calls, effectively bypassing the SSO checks
	log.Info("reconciling SSO")
	if err := r.reconcileSSO(cr); err != nil {
		log.Info(err.Error())
	}

	log.Info("reconciling status")
	if err := r.reconcileStatus(cr); err != nil {
		log.Info(err.Error())
	}

	log.Info("reconciling roles")
	if err := r.reconcileRoles(cr); err != nil {
		log.Info(err.Error())
		return err
	}

	log.Info("reconciling rolebindings")
	if err := r.reconcileRoleBindings(cr); err != nil {
		log.Info(err.Error())
		return err
	}

	log.Info("reconciling service accounts")
	if err := r.reconcileServiceAccounts(cr); err != nil {
		log.Info(err.Error())
		return err
	}

	log.Info("reconciling certificate authority")
	if err := r.reconcileCertificateAuthority(cr); err != nil {
		return err
	}

	log.Info("reconciling secrets")
	if err := r.reconcileSecrets(cr); err != nil {
		return err
	}

	useTLSForRedis := r.redisShouldUseTLS(cr)

	log.Info("reconciling config maps")
	if err := r.reconcileConfigMaps(cr, useTLSForRedis); err != nil {
		return err
	}

	log.Info("reconciling services")
	if err := r.reconcileServices(cr); err != nil {
		return err
	}

	log.Info("reconciling deployments")
	if err := r.reconcileDeployments(cr, useTLSForRedis); err != nil {
		return err
	}

	log.Info("reconciling statefulsets")
	if err := r.reconcileStatefulSets(cr, useTLSForRedis); err != nil {
		return err
	}

	log.Info("reconciling autoscalers")
	if err := r.reconcileAutoscalers(cr); err != nil {
		return err
	}

	log.Info("reconciling ingresses")
	if err := r.reconcileIngresses(cr); err != nil {
		return err
	}

	if IsRouteAPIAvailable() {
		log.Info("reconciling routes")
		if err := r.reconcileRoutes(cr); err != nil {
			return err
		}
	}

	if IsPrometheusAPIAvailable() {
		log.Info("reconciling prometheus")
		if err := r.reconcilePrometheus(cr); err != nil {
			return err
		}

		// Reconciles prometheusRule created to alert based on argo-cd workload status
		if err := r.reconcilePrometheusRule(cr); err != nil {
			return err
		}

		if err := r.reconcileMetricsServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileRepoServerServiceMonitor(cr); err != nil {
			return err
		}

		if err := r.reconcileServerMetricsServiceMonitor(cr); err != nil {
			return err
		}
	}

	if cr.Spec.ApplicationSet != nil {
		log.Info("reconciling ApplicationSet controller")
		if err := r.reconcileApplicationSetController(cr); err != nil {
			return err
		}
	}

	if cr.Spec.Notifications.Enabled {
		log.Info("reconciling Notifications controller")
		if err := r.reconcileNotificationsController(cr); err != nil {
			return err
		}
	}

	if err := r.reconcileRepoServerTLSSecret(cr); err != nil {
		return err
	}

	if err := r.reconcileRedisTLSSecret(cr, useTLSForRedis); err != nil {
		return err
	}

	return nil
}

// setResourceWatches will register Watches for each of the supported Resources.
func (r *ReconcileArgoCD) setResourceWatches(bldr *builder.Builder, clusterResourceMapper, tlsSecretMapper, namespaceResourceMapper, clusterSecretResourceMapper, applicationSetGitlabSCMTLSConfigMapMapper handler.MapFunc) *builder.Builder {

	deploymentConfigPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			// Ignore updates to CR status in which case metadata.Generation does not change
			var count int32 = 1
			newDC, ok := e.ObjectNew.(*oappsv1.DeploymentConfig)
			if !ok {
				return false
			}
			oldDC, ok := e.ObjectOld.(*oappsv1.DeploymentConfig)
			if !ok {
				return false
			}
			if newDC.Name == defaultKeycloakIdentifier {
				if newDC.Status.AvailableReplicas == count {
					return true
				}
				if newDC.Status.AvailableReplicas == int32(0) &&
					!reflect.DeepEqual(oldDC.Status.AvailableReplicas, newDC.Status.AvailableReplicas) {
					// Handle the deletion of keycloak pod.
					log.Info(fmt.Sprintf("Handle the pod deletion event for keycloak deployment config %s in namespace %s",
						newDC.Name, newDC.Namespace))
					err := handleKeycloakPodDeletion(newDC)
					if err != nil {
						log.Error(err, fmt.Sprintf("Failed to update Deployment Config %s for keycloak pod deletion in namespace %s",
							newDC.Name, newDC.Namespace))
					}
				}
			}
			return false
		},
	}

	deleteSSOPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newCR, ok := e.ObjectNew.(*argoproj.ArgoCD)
			if !ok {
				return false
			}
			oldCR, ok := e.ObjectOld.(*argoproj.ArgoCD)
			if !ok {
				return false
			}

			// Handle deletion of SSO from Argo CD custom resource
			if !reflect.DeepEqual(oldCR.Spec.SSO, newCR.Spec.SSO) && newCR.Spec.SSO == nil {
				err := r.deleteSSOConfiguration(newCR, oldCR)
				if err != nil {
					log.Error(err, fmt.Sprintf("Failed to delete SSO Configuration for ArgoCD %s in namespace %s",
						newCR.Name, newCR.Namespace))
				}
			}

			// Trigger reconciliation of SSO on update event
			if !reflect.DeepEqual(oldCR.Spec.SSO, newCR.Spec.SSO) && newCR.Spec.SSO != nil && oldCR.Spec.SSO != nil {
				err := r.reconcileSSO(newCR)
				if err != nil {
					log.Error(err, fmt.Sprintf("Failed to update existing SSO Configuration for ArgoCD %s in namespace %s",
						newCR.Name, newCR.Namespace))
				}
			}
			return true
		},
	}

	// Add new predicate to delete Notifications Resources. The predicate watches the Argo CD CR for changes to the `.spec.Notifications.Enabled`
	// field. When a change is detected that results in notifications being disabled, we trigger deletion of notifications resources
	deleteNotificationsPred := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			newCR, ok := e.ObjectNew.(*argoproj.ArgoCD)
			if !ok {
				return false
			}
			oldCR, ok := e.ObjectOld.(*argoproj.ArgoCD)
			if !ok {
				return false
			}
			if oldCR.Spec.Notifications.Enabled && !newCR.Spec.Notifications.Enabled {
				err := r.deleteNotificationsResources(newCR)
				if err != nil {
					log.Error(err, fmt.Sprintf("Failed to delete notifications controller resources for ArgoCD %s in namespace %s",
						newCR.Name, newCR.Namespace))
				}
			}
			return true
		},
	}

	// Watch for changes to primary resource ArgoCD
	bldr.For(&argoproj.ArgoCD{}, builder.WithPredicates(deleteSSOPred, deleteNotificationsPred))

	// Watch for changes to ConfigMap sub-resources owned by ArgoCD instances.
	bldr.Owns(&corev1.ConfigMap{})

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	bldr.Owns(&corev1.Secret{})

	// Watch for changes to Service sub-resources owned by ArgoCD instances.
	bldr.Owns(&corev1.Service{})

	// Watch for changes to Deployment sub-resources owned by ArgoCD instances.
	bldr.Owns(&appsv1.Deployment{})

	// Watch for changes to Ingress sub-resources owned by ArgoCD instances.
	bldr.Owns(&networkingv1.Ingress{})

	bldr.Owns(&v1.Role{})

	bldr.Owns(&v1.RoleBinding{})

	clusterResourceHandler := handler.EnqueueRequestsFromMapFunc(clusterResourceMapper)

	clusterSecretResourceHandler := handler.EnqueueRequestsFromMapFunc(clusterSecretResourceMapper)

	appSetGitlabSCMTLSConfigMapHandler := handler.EnqueueRequestsFromMapFunc(applicationSetGitlabSCMTLSConfigMapMapper)

	tlsSecretHandler := handler.EnqueueRequestsFromMapFunc(tlsSecretMapper)

	bldr.Watches(&v1.ClusterRoleBinding{}, clusterResourceHandler)

	bldr.Watches(&v1.ClusterRole{}, clusterResourceHandler)

	bldr.Watches(&corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{
		Name: common.ArgoCDAppSetGitlabSCMTLSCertsConfigMapName,
	}}, appSetGitlabSCMTLSConfigMapHandler)

	// Watch for secrets of type TLS that might be created by external processes
	bldr.Watches(&corev1.Secret{Type: corev1.SecretTypeTLS}, tlsSecretHandler)

	// Watch for cluster secrets added to the argocd instance
	bldr.Watches(&corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Labels: map[string]string{
			common.ArgoCDManagedByClusterArgoCDLabel: "cluster",
		}}}, clusterSecretResourceHandler)

	// Watch for changes to Secret sub-resources owned by ArgoCD instances.
	bldr.Owns(&appsv1.StatefulSet{})

	// Inspect cluster to verify availability of extra features
	// This sets the flags that are used in subsequent checks
	if err := InspectCluster(); err != nil {
		log.Info("unable to inspect cluster")
	}

	if IsRouteAPIAvailable() {
		// Watch OpenShift Route sub-resources owned by ArgoCD instances.
		bldr.Owns(&routev1.Route{})
	}

	if IsPrometheusAPIAvailable() {
		// Watch Prometheus sub-resources owned by ArgoCD instances.
		bldr.Owns(&monitoringv1.Prometheus{})

		// Watch Prometheus ServiceMonitor sub-resources owned by ArgoCD instances.
		bldr.Owns(&monitoringv1.ServiceMonitor{})
	}

	if IsTemplateAPIAvailable() {
		// Watch for the changes to Deployment Config
		bldr.Owns(&oappsv1.DeploymentConfig{}, builder.WithPredicates(deploymentConfigPred))

	}

	namespaceHandler := handler.EnqueueRequestsFromMapFunc(namespaceResourceMapper)

	bldr.Watches(&corev1.Namespace{}, namespaceHandler, builder.WithPredicates(namespaceFilterPredicate()))

	return bldr
}
