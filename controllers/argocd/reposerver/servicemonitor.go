package reposerver

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	"github.com/argoproj-labs/argocd-operator/pkg/monitoring"
	"github.com/argoproj-labs/argocd-operator/pkg/mutation"
	"github.com/argoproj-labs/argocd-operator/pkg/util"
	monitoringv1 "github.com/coreos/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (rsr *RepoServerReconciler) reconcileMetriscServiceMonitor() error {
	req := monitoring.ServiceMonitorRequest{
		ObjectMeta: argoutil.GetObjMeta(metricsResourceName, rsr.Instance.Namespace, rsr.Instance.Name, rsr.Instance.Namespace, component, argocdcommon.GetSvcMonitorLabel(), util.EmptyMap()),
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{
					common.AppK8sKeyName: resourceName,
				},
			},
			Endpoints: []monitoringv1.Endpoint{
				{
					Port: common.ArgoCDMetrics,
				},
			},
		},
		Client:    rsr.Client,
		Mutations: []mutation.MutateFunc{mutation.ApplyReconcilerMutation},
	}

	ignoreDrift := false
	updateFn := func(existing, desired *monitoringv1.ServiceMonitor, changed *bool) error {
		fieldsToCompare := []argocdcommon.FieldToCompare{
			{Existing: &existing.Labels, Desired: &desired.Labels, ExtraAction: nil},
			{Existing: &existing.Spec, Desired: &desired.Spec, ExtraAction: nil},
		}
		argocdcommon.UpdateIfChanged(fieldsToCompare, changed)
		return nil
	}
	return rsr.reconServiceMonitor(req, argocdcommon.UpdateFnSM(updateFn), ignoreDrift)
}

func (rsr *RepoServerReconciler) reconServiceMonitor(req monitoring.ServiceMonitorRequest, updateFn interface{}, ignoreDrift bool) error {
	desired, err := monitoring.RequestServiceMonitor(req)
	if err != nil {
		rsr.Logger.Debug("reconcileServiceMonitor: one or more mutations could not be applied")
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to request ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
	}

	if err = controllerutil.SetControllerReference(rsr.Instance, desired, rsr.Scheme); err != nil {
		rsr.Logger.Error(err, "reconcileServiceMonitor: failed to set owner reference for ServiceMonitor", "name", desired.Name, "namespace", desired.Namespace)
	}

	existing, err := monitoring.GetServiceMonitor(desired.Name, desired.Namespace, rsr.Client)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to retrieve ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
		}

		if err = monitoring.CreateServiceMonitor(desired, rsr.Client); err != nil {
			return errors.Wrapf(err, "reconcileServiceMonitor: failed to create ServiceMonitor %s in namespace %s", desired.Name, desired.Namespace)
		}
		rsr.Logger.Info("ServiceMonitor created", "name", desired.Name, "namespace", desired.Namespace)
		return nil
	}

	// ServiceMonitor found, no update required - nothing to do
	if ignoreDrift {
		return nil
	}

	changed := false

	// execute supplied update function
	if updateFn != nil {
		if fn, ok := updateFn.(argocdcommon.UpdateFnSM); ok {
			if err := fn(existing, desired, &changed); err != nil {
				return errors.Wrapf(err, "reconcileServiceMonitor: failed to execute update function for %s in namespace %s", existing.Name, existing.Namespace)
			}
		}
	}

	if !changed {
		return nil
	}

	if err = monitoring.UpdateServiceMonitor(existing, rsr.Client); err != nil {
		return errors.Wrapf(err, "reconcileServiceMonitor: failed to update ServiceMonitor %s", existing.Name)
	}

	rsr.Logger.Info("ServiceMonitor updated", "name", existing.Name, "namespace", existing.Namespace)
	return nil
}

func (rsr *RepoServerReconciler) deleteServiceMonitor(name, namespace string) error {
	// return if prometheus API is not present on cluster
	if !monitoring.IsPrometheusAPIAvailable() {
		rsr.Logger.Debug("prometheus API unavailable, skip service monitor deletion")
		return nil
	}

	if err := monitoring.DeleteServiceMonitor(name, namespace, rsr.Client); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return errors.Wrapf(err, "deleteServiceMonitor: failed to delete service monitor %s in namespace %s", name, namespace)
	}
	rsr.Logger.Info("service monitor deleted", "name", name, "namespace", namespace)
	return nil
}
