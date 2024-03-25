package applicationset

import (
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argocd/argocdcommon"
	"github.com/argoproj-labs/argocd-operator/pkg/argoutil"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// getHost will return the host for the given ArgoCD.
func (asr *ApplicationSetReconciler) getHost() string {
	host := asr.Instance.Name
	if len(asr.Instance.Spec.ApplicationSet.WebhookServer.Host) > 0 {
		tmpHost, err := argocdcommon.ShortenHostname(asr.Instance.Spec.ApplicationSet.WebhookServer.Host)
		if err != nil {
			asr.Logger.Error(err, "getHost: failed to shorten hostname")
		} else {
			host = tmpHost
		}
	}
	return host
}

// getResources will return the ResourceRequirements for the Application Sets container.
func (asr *ApplicationSetReconciler) getResources() corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if asr.Instance.Spec.ApplicationSet.Resources != nil {
		resources = *asr.Instance.Spec.ApplicationSet.Resources
	}

	return resources
}

// getSCMRootCAConfigMapName will return the SCMRootCA ConfigMap name for the given ArgoCD ApplicationSet Controller.
func (asr *ApplicationSetReconciler) getSCMRootCAConfigMapName() string {
	if asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap != "" && len(asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap) > 0 {
		return asr.Instance.Spec.ApplicationSet.SCMRootCAConfigMap
	}
	return ""
}

func (asr *ApplicationSetReconciler) getAppsetSourceNsRBAC() ([]types.NamespacedName, []types.NamespacedName, error) {
	roles := []types.NamespacedName{}
	rbs := []types.NamespacedName{}

	compReq, err := argocdcommon.GetComponentLabelRequirement(component)
	if err != nil {
		return nil, nil, err
	}

	rbacReq, err := argocdcommon.GetRbacTypeLabelRequirement(common.ArgoCDRBACTypeAppSetManagement)
	if err != nil {
		return nil, nil, err
	}

	ls := argocdcommon.GetLabelSelector(*compReq, *rbacReq)

	for ns := range asr.AppsetSourceNamespaces {
		nsRoles, nsRbs := argocdcommon.GetRBACToBeDeleted(ns, ls, asr.Client, asr.Logger)
		roles = append(roles, nsRoles...)
		rbs = append(rbs, nsRbs...)
	}

	return roles, rbs, nil
}

func (asr *ApplicationSetReconciler) getCmd() []string {
	cmd := make([]string, 0)

	cmd = append(cmd, EntryPointSh)
	cmd = append(cmd, AppSetController)

	cmd = append(cmd, ArgoCDRepoServer)
	cmd = append(cmd, asr.RepoServer.GetServerAddress())

	cmd = append(cmd, common.LogLevelCmd)
	cmd = append(cmd, argoutil.GetLogLevel(asr.Instance.Spec.ApplicationSet.LogLevel))

	// ApplicationSet command arguments provided by the user
	extraArgs := asr.Instance.Spec.ApplicationSet.ExtraCommandArgs
	err := argocdcommon.IsMergable(extraArgs, cmd)
	if err != nil {
		return cmd
	}

	cmd = append(cmd, extraArgs...)

	return cmd
}
