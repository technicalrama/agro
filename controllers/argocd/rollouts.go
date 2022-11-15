// Copyright 2021 ArgoCD Operator Developers
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
	"context"
	"fmt"
	"os"
	"reflect"

	argoprojv1a1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// The following are the rollouts controller resources reconciled by the operator.
// kind: ServiceAccount # name: argo-rollouts
// kind: ClusterRole # name: argo-rollouts
// kind: ClusterRole # name: argo-rollouts-aggregate-to-admin
// kind: ClusterRole # name: argo-rollouts-aggregate-to-edit
// kind: ClusterRole # name: argo-rollouts-aggregate-to-view
// kind: ClusterRoleBinding # name: argo-rollouts
// kind: Secret # name: argo-rollouts-notification-secret
// kind: Service # name: argo-rollouts-metrics
// kind: Deployment # name: argo-rollouts

func (r *ReconcileArgoCD) reconcileRolloutsController(cr *argoprojv1a1.ArgoCD) error {

	log.Info("reconciling rollouts serviceaccount")
	sa, err := r.reconcileRolloutsServiceAccount(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling rollouts roles")
	role, err := r.reconcileRolloutsRole(cr)
	if err != nil {
		return err
	}

	log.Info("reconciling rollouts role bindings")
	if err := r.reconcileRolloutsRoleBinding(cr, role, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts secret")
	if err := r.reconcileRolloutsSecrets(cr); err != nil {
		return err
	}

	log.Info("reconciling rollouts deployment")
	if err := r.reconcileRolloutsDeployment(cr, sa); err != nil {
		return err
	}

	log.Info("reconciling rollouts service")
	if err := r.reconcileRolloutsService(cr); err != nil {
		return err
	}

	return nil
}

// Reconciles rollouts serviceaccount.
func (r *ReconcileArgoCD) reconcileRolloutsServiceAccount(cr *argoprojv1a1.ArgoCD) (*corev1.ServiceAccount, error) {

	sa := newServiceAccountWithName("argo-rollouts", cr)
	setRolloutsLabels(&sa.ObjectMeta)

	exists := true
	if err := argoutil.FetchObject(r.Client, cr.Namespace, sa.Name, sa); err != nil {
		if !errors.IsNotFound(err) {
			return nil, err
		}
		exists = false
	}

	if exists {
		return sa, nil
	}

	if err := controllerutil.SetControllerReference(cr, sa, r.Scheme); err != nil {
		return nil, err
	}

	err := r.Client.Create(context.TODO(), sa)
	if err != nil {
		return nil, err
	}

	return sa, err
}

// Reconciles rollouts role.
func (r *ReconcileArgoCD) reconcileRolloutsRole(cr *argoprojv1a1.ArgoCD) (*v1.Role, error) {

	policyRules := []v1.PolicyRule{

		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"rollouts",
				"rollouts/status",
				"rollouts/finalizers",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"analysisruns",
				"analysisruns/finalizers",
				"experiments",
				"experiments/finalizers",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"argoproj.io",
			},
			Resources: []string{
				"analysistemplates",
				"clusteranalysistemplates",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"apps",
			},
			Resources: []string{
				"replicasets",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
				"apps",
			},
			Resources: []string{
				"deployments",
				"podtemplates",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"services",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
				"patch",
				"create",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"coordination.k8s.io",
			},
			Resources: []string{
				"leases",
			},
			Verbs: []string{
				"create",
				"get",
				"update",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"secrets",
				"configmaps",
			},
			Verbs: []string{
				"get",
				"list",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods",
			},
			Verbs: []string{
				"list",
				"update",
				"watch",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"pods/eviction",
			},
			Verbs: []string{
				"create",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"events",
			},
			Verbs: []string{
				"create",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"networking.k8s.io",
				"extensions",
			},
			Resources: []string{
				"ingresses",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"batch",
			},
			Resources: []string{
				"jobs",
			},
			Verbs: []string{
				"create",
				"get",
				"list",
				"watch",
				"update",
				"patch",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"networking.istio.io",
			},
			Resources: []string{
				"virtualservices",
				"destinationrules",
			},
			Verbs: []string{
				"watch",
				"get",
				"update",
				"patch",
				"list",
			},
		},
		{
			APIGroups: []string{
				"split.smi-spec.io",
			},
			Resources: []string{
				"trafficsplits",
			},
			Verbs: []string{
				"create",
				"watch",
				"get",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"getambassador.io",
				"x.getambassador.io",
			},
			Resources: []string{
				"mappings",
				"ambassadormappings",
			},
			Verbs: []string{
				"create",
				"watch",
				"get",
				"update",
				"list",
				"delete",
			},
		},
		{
			APIGroups: []string{
				"",
			},
			Resources: []string{
				"endpoints",
			},
			Verbs: []string{
				"get",
			},
		},
		{
			APIGroups: []string{
				"elbv2.k8s.aws",
			},
			Resources: []string{
				"targetgroupbindings",
			},
			Verbs: []string{
				"list",
				"get",
			},
		},
		{
			APIGroups: []string{
				"appmesh.k8s.aws",
			},
			Resources: []string{
				"virtualservices",
			},
			Verbs: []string{
				"watch",
				"get",
				"list",
			},
		},
		{
			APIGroups: []string{
				"appmesh.k8s.aws",
			},
			Resources: []string{
				"virtualnodes",
				"virtualrouters",
			},
			Verbs: []string{
				"watch",
				"get",
				"list",
				"update",
				"patch",
			},
		},
		{
			APIGroups: []string{
				"traefik.containo.us",
			},
			Resources: []string{
				"traefikservices",
			},
			Verbs: []string{
				"watch",
				"get",
				"update",
			},
		},
	}

	role := newRole("argo-rollouts", policyRules, cr)
	setRolloutsLabels(&role.ObjectMeta)

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: role.Name, Namespace: cr.Namespace}, role)
	if err != nil {
		if !errors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to reconcile the role for the service account associated with %s : %s", role.Name, err)
		}
		if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
			return nil, err
		}
		return role, r.Client.Create(context.TODO(), role)
	}

	role.Rules = policyRules
	if err = controllerutil.SetControllerReference(cr, role, r.Scheme); err != nil {
		return nil, err
	}
	return role, r.Client.Update(context.TODO(), role)
}

// Reconcile rollouts rolebinding.
func (r *ReconcileArgoCD) reconcileRolloutsRoleBinding(cr *argoprojv1a1.ArgoCD, role *v1.Role, sa *corev1.ServiceAccount) error {

	name := "argo-rollouts"

	// get expected name
	roleBinding := newRoleBindingWithname(name, cr)

	// fetch existing rolebinding by name
	roleBindingExists := true
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: roleBinding.Name, Namespace: cr.Namespace}, roleBinding); err != nil {
		if !errors.IsNotFound(err) {
			return fmt.Errorf("failed to get the rolebinding associated with %s : %s", name, err)
		}
		roleBindingExists = false
	}

	setRolloutsLabels(&roleBinding.ObjectMeta)

	roleBinding.RoleRef = v1.RoleRef{
		APIGroup: v1.GroupName,
		Kind:     "Role",
		Name:     role.Name,
	}

	roleBinding.Subjects = []v1.Subject{
		{
			Kind:      v1.ServiceAccountKind,
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
	}

	if err := controllerutil.SetControllerReference(cr, roleBinding, r.Scheme); err != nil {
		return err
	}

	if roleBindingExists {
		return r.Client.Update(context.TODO(), roleBinding)
	}

	return r.Client.Create(context.TODO(), roleBinding)
}

// reconcileRolloutsDeployment will ensure the Deployment resource is present for the Rollouts controller component.
func (r *ReconcileArgoCD) reconcileRolloutsDeployment(cr *argoprojv1a1.ArgoCD, sa *corev1.ServiceAccount) error {
	deploy := newDeploymentWithSuffix("argo-rollouts", "controller", cr)

	setRolloutsLabels(&deploy.ObjectMeta)

	podSpec := &deploy.Spec.Template.Spec

	runAsNonRoot := true
	podSpec.SecurityContext = &corev1.PodSecurityContext{
		RunAsNonRoot: &runAsNonRoot,
	}

	podSpec.ServiceAccountName = sa.ObjectMeta.Name

	podSpec.Containers = []corev1.Container{
		rolloutsContainer(cr),
	}
	AddSeccompProfileForOpenShift(r.Client, podSpec)

	if existing := newDeploymentWithSuffix("argo-rollouts", "controller", cr); argoutil.IsObjectFound(r.Client, cr.Namespace, existing.Name, existing) {

		existingSpec := existing.Spec.Template.Spec

		deploymentsDifferent := !reflect.DeepEqual(existingSpec.Containers[0], podSpec.Containers) ||
			existingSpec.ServiceAccountName != podSpec.ServiceAccountName ||
			!reflect.DeepEqual(existing.Labels, deploy.Labels) ||
			!reflect.DeepEqual(existing.Spec.Template.Labels, deploy.Spec.Template.Labels) ||
			!reflect.DeepEqual(existing.Spec.Selector, deploy.Spec.Selector) ||
			!reflect.DeepEqual(existing.Spec.Template.Spec.NodeSelector, deploy.Spec.Template.Spec.NodeSelector) ||
			!reflect.DeepEqual(existing.Spec.Template.Spec.Tolerations, deploy.Spec.Template.Spec.Tolerations) ||
			!reflect.DeepEqual(existing.Spec.Template.Spec.SecurityContext, podSpec.SecurityContext)

		// If the Deployment already exists, make sure the values we care about are up-to-date
		if deploymentsDifferent {
			existing.Spec.Template.Spec.Containers = podSpec.Containers
			existing.Spec.Template.Spec.ServiceAccountName = podSpec.ServiceAccountName
			existing.Labels = deploy.Labels
			existing.Spec.Template.Labels = deploy.Spec.Template.Labels
			existing.Spec.Selector = deploy.Spec.Selector
			existing.Spec.Template.Spec.NodeSelector = deploy.Spec.Template.Spec.NodeSelector
			existing.Spec.Template.Spec.Tolerations = deploy.Spec.Template.Spec.Tolerations
			existing.Spec.Template.Spec.SecurityContext = podSpec.SecurityContext
			return r.Client.Update(context.TODO(), existing)
		}
		return nil
	}

	if err := controllerutil.SetControllerReference(cr, deploy, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), deploy)

}

func rolloutsContainer(cr *argoprojv1a1.ArgoCD) corev1.Container {

	// Global proxy env vars go first
	rolloutsEnv := cr.Spec.Rollouts.Env

	// Environment specified in the CR take precedence over everything else
	rolloutsEnv = argoutil.EnvMerge(rolloutsEnv, proxyEnvVars(), false)

	return corev1.Container{
		Command:         cr.Spec.Rollouts.ExtraCommandArgs,
		Env:             rolloutsEnv,
		Image:           getRolloutsContainerImage(cr),
		ImagePullPolicy: corev1.PullAlways,
		LivenessProbe: &corev1.Probe{
			FailureThreshold: 3,
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/healthz",
					Port: intstr.FromString("healthz"),
				},
			},
			InitialDelaySeconds: int32(30),
			PeriodSeconds:       int32(20),
			SuccessThreshold:    int32(1),
			TimeoutSeconds:      int32(10),
		},
		Name:      "argo-rollouts",
		Resources: getRolloutsControllerResources(cr),
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 8080,
				Name:          "healthz",
			},
			{
				ContainerPort: 8090,
				Name:          "metrics",
			},
		},
		ReadinessProbe: &corev1.Probe{
			FailureThreshold: int32(5),
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/metrics",
					Port: intstr.FromString("metrics"),
				},
			},
			InitialDelaySeconds: int32(10),
			PeriodSeconds:       int32(5),
			SuccessThreshold:    int32(1),
			TimeoutSeconds:      int32(4),
		},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
			AllowPrivilegeEscalation: boolPtr(false),
			ReadOnlyRootFilesystem:   boolPtr(true),
			RunAsNonRoot:             boolPtr(true),
		},
	}
}

// Returns the container image for rollouts controller.
func getRolloutsContainerImage(cr *argoprojv1a1.ArgoCD) string {
	defaultImg, defaultTag := false, false

	img := ""
	tag := ""

	// First pull from spec, if it exists
	if cr.Spec.Rollouts != nil {
		img = cr.Spec.Rollouts.Image
		tag = cr.Spec.Rollouts.Version
	}

	// If spec is empty, use the defaults
	if img == "" {
		img = common.ArgoRolloutsDefaultImage
		defaultImg = true
	}
	if tag == "" {
		tag = common.ArgoRolloutsDefaultVersion
		defaultTag = true
	}

	// If an env var is specified then use that, but don't override the spec values (if they are present)
	if e := os.Getenv(common.ArgoCDImageEnvName); e != "" && (defaultTag && defaultImg) {
		return e
	}
	return argoutil.CombineImageTag(img, tag)
}

// getRolloutsControllerResources will return the ResourceRequirements for the rollouts container.
func getRolloutsControllerResources(cr *argoprojv1a1.ArgoCD) corev1.ResourceRequirements {
	resources := corev1.ResourceRequirements{}

	// Allow override of resource requirements from CR
	if cr.Spec.ApplicationSet.Resources != nil {
		resources = *cr.Spec.ApplicationSet.Resources
	}

	return resources
}

func setRolloutsLabels(obj *metav1.ObjectMeta) {
	obj.Labels["app.kubernetes.io/name"] = "argo-rollouts"
	obj.Labels["app.kubernetes.io/part-of"] = "argo-rollouts"
	obj.Labels["app.kubernetes.io/component"] = "rollouts-controller"
}

// reconcileRolloutsService will ensure that the Service is present for the Rollouts controller.
func (r *ReconcileArgoCD) reconcileRolloutsService(cr *argoprojv1a1.ArgoCD) error {

	svc := newServiceWithSuffix("argo-rollouts", "controller", cr)
	if cr.Spec.Rollouts == nil {

		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			err := argoutil.FetchObject(r.Client, cr.Namespace, svc.Name, svc)
			if err != nil {
				return err
			}
			log.Info(fmt.Sprintf("Deleting rollouts controller service %s as rollouts is disabled", svc.Name))
			err = r.Delete(context.TODO(), svc)
			if err != nil {
				return err
			}
		}
	} else {
		if argoutil.IsObjectFound(r.Client, cr.Namespace, svc.Name, svc) {
			return nil // Service found, do nothing
		}
	}
	svc.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "metrics",
			Port:       8090,
			Protocol:   corev1.ProtocolTCP,
			TargetPort: intstr.FromInt(8090),
		},
	}

	svc.Spec.Selector = map[string]string{
		common.ArgoCDKeyName: nameWithSuffix("argo-rollouts", cr),
	}

	if err := controllerutil.SetControllerReference(cr, svc, r.Scheme); err != nil {
		return err
	}
	return r.Client.Create(context.TODO(), svc)
}

//
func (r *ReconcileArgoCD) reconcileRolloutsSecrets(cr *argoprojv1a1.ArgoCD) error {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "argo-rollouts-notification-secret",
			Namespace: cr.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
	}

	if argoutil.IsObjectFound(r.Client, cr.Namespace, secret.Name, secret) {
		return nil // secret found, do nothing
	}

	err := r.Client.Create(context.TODO(), secret)
	if err != nil {
		return err
	}

	return nil
}
