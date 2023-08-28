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
	"testing"

	appsv1 "github.com/openshift/api/apps/v1"
	oappsv1 "github.com/openshift/api/apps/v1"
	routev1 "github.com/openshift/api/route/v1"
	templatev1 "github.com/openshift/api/template/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoappv1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	argoprojv1alpha1 "github.com/argoproj-labs/argocd-operator/api/v1alpha1"
	"github.com/argoproj-labs/argocd-operator/common"
	"github.com/argoproj-labs/argocd-operator/controllers/argoutil"
)

var (
	fakeNs             = "foo"
	fakeReplicas int32 = 1
	fakeVolumes        = []corev1.Volume{
		{
			Name: "sso-x509-https-volume",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: servingCertSecretName,
				},
			},
		},
		{
			Name: "service-ca",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "${APPLICATION_NAME}-service-ca",
					},
				},
			},
		},
		{
			Name: "sso-probe-netrc-volume",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					Medium: "Memory",
				},
			},
		},
	}
)

func getFakeKeycloakResources() corev1.ResourceRequirements {
	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("1Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("1m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resourcev1.MustParse("128Mi"),
			corev1.ResourceCPU:    resourcev1.MustParse("128m"),
		},
	}
}

func TestKeycloakContainerImage(t *testing.T) {

	defer removeTemplateAPI()
	tests := []struct {
		name               string
		setEnvVarFunc      func(*testing.T, string)
		envVar             string
		argoCD             *argoprojv1alpha1.ArgoCD
		updateCrFunc       func(cr *argoprojv1alpha1.ArgoCD)
		templateAPIFound   bool
		wantContainerImage string
	}{
		{
			name:          "no .spec.sso, no ArgoCDKeycloakImageEnvName env var set",
			setEnvVarFunc: nil,
			envVar:        "",
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:       nil,
			templateAPIFound:   false,
			wantContainerImage: "quay.io/keycloak/keycloak@sha256:64fb81886fde61dee55091e6033481fa5ccdac62ae30a4fd29b54eb5e97df6a9",
		},
		{
			name:          "no .spec.sso, no ArgoCDKeycloakImageEnvName env var set - for OCP",
			setEnvVarFunc: nil,
			envVar:        "",
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:       nil,
			templateAPIFound:   true,
			wantContainerImage: "registry.redhat.io/rh-sso-7/sso76-openshift-rhel8@sha256:bb6dc12a49370ba6baa40cfa064238cddcfd1edb22c37dcdf53d331c0f7ee15d",
		},
		{
			name: "ArgoCDKeycloakImageEnvName env var set",
			setEnvVarFunc: func(t *testing.T, s string) {
				t.Setenv(common.ArgoCDKeycloakImageEnvName, s)
			},
			envVar: "envImage:latest",
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:       nil,
			templateAPIFound:   true,
			wantContainerImage: "envImage:latest",
		},
		{
			name: "both cr.spec.sso.Image and ArgoCDKeycloakImageEnvName are set.",
			setEnvVarFunc: func(t *testing.T, s string) {
				t.Setenv(common.ArgoCDKeycloakImageEnvName, s)
			},
			envVar: "envImage:latest",
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
					Image:    "crImage",
					Version:  "crVersion",
				}
			},
			templateAPIFound:   true,
			wantContainerImage: "crImage:crVersion",
		},
		{
			name: "both cr.spec.sso.keycloak.Image and ArgoCDKeycloakImageEnvName are set",
			setEnvVarFunc: func(t *testing.T, s string) {
				t.Setenv(common.ArgoCDKeycloakImageEnvName, s)
			},
			envVar: "envImage:latest",
			argoCD: makeArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Image:   "crImage",
						Version: "crVersion",
					},
				}
			},
			templateAPIFound:   true,
			wantContainerImage: "crImage:crVersion",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			templateAPIFound = test.templateAPIFound

			if test.setEnvVarFunc != nil {
				test.setEnvVarFunc(t, test.envVar)
			}
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			testImage := getKeycloakContainerImage(test.argoCD)
			assert.Equal(t, test.wantContainerImage, testImage)

		})
	}
}

func TestNewKeycloakTemplateInstance(t *testing.T) {
	// For OpenShift Container Platform.
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	tmplInstance, err := newKeycloakTemplateInstance(a)
	assert.NoError(t, err)

	assert.Equal(t, tmplInstance.Name, "rhsso")
	assert.Equal(t, tmplInstance.Namespace, a.Namespace)
}

func TestNewKeycloakTemplate(t *testing.T) {
	// For OpenShift Container Platform.
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	tmpl, err := newKeycloakTemplate(a)
	assert.NoError(t, err)

	assert.Equal(t, tmpl.Name, "rhsso")
	assert.Equal(t, tmpl.Namespace, a.Namespace)
}

func TestNewKeycloakTemplate_testDeploymentConfig(t *testing.T) {
	// For OpenShift Container Platform.
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	dc := getKeycloakDeploymentConfigTemplate(a)

	assert.Equal(t, dc.Spec.Replicas, fakeReplicas)

	strategy := appsv1.DeploymentStrategy{
		Type: "Recreate",
		Resources: corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceMemory: resourcev1.MustParse("256Mi"),
				corev1.ResourceCPU:    resourcev1.MustParse("250m"),
			},
			Limits: corev1.ResourceList{
				corev1.ResourceMemory: resourcev1.MustParse("512Mi"),
				corev1.ResourceCPU:    resourcev1.MustParse("500m"),
			},
		},
	}
	assert.Equal(t, dc.Spec.Strategy, strategy)

	assert.Equal(t, dc.Spec.Template.ObjectMeta.Name, "${APPLICATION_NAME}")
	assert.Equal(t, dc.Spec.Template.Spec.Volumes, fakeVolumes)
}

func TestNewKeycloakTemplate_testKeycloakContainer(t *testing.T) {
	// For OpenShift Container Platform.
	t.Setenv(common.ArgoCDKeycloakImageEnvName, "")
	templateAPIFound = true
	defer removeTemplateAPI()

	a := makeTestArgoCD()
	a.Spec.SSO = &argoappv1.ArgoCDSSOSpec{
		Provider: "keycloak",
	}
	kc := getKeycloakContainer(a)
	assert.Equal(t,
		"registry.redhat.io/rh-sso-7/sso76-openshift-rhel8@sha256:bb6dc12a49370ba6baa40cfa064238cddcfd1edb22c37dcdf53d331c0f7ee15d", kc.Image)
	assert.Equal(t, corev1.PullAlways, kc.ImagePullPolicy)
	assert.Equal(t, "${APPLICATION_NAME}", kc.Name)
}

func TestKeycloakResources(t *testing.T) {
	defer removeTemplateAPI()
	fR := getFakeKeycloakResources()

	tests := []struct {
		name          string
		argoCD        *argoprojv1alpha1.ArgoCD
		updateCrFunc  func(cr *argoprojv1alpha1.ArgoCD)
		wantResources corev1.ResourceRequirements
	}{
		{
			name: "default",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc:  nil,
			wantResources: defaultKeycloakResources(),
		},
		{
			name: "override with .spec.sso",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Resources: &fR,
				}
			},
			wantResources: getFakeKeycloakResources(),
		},
		{
			name: "override with .spec.sso.keycloak",
			argoCD: makeTestArgoCD(func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			updateCrFunc: func(cr *argoprojv1alpha1.ArgoCD) {
				cr.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						Resources: &fR,
					},
				}
			},
			wantResources: getFakeKeycloakResources(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.updateCrFunc != nil {
				test.updateCrFunc(test.argoCD)
			}

			testResources := getKeycloakContainer(test.argoCD).Resources
			assert.Equal(t, test.wantResources, testResources)

		})
	}
}

func TestNewKeycloakTemplate_testConfigmap(t *testing.T) {
	cm := getKeycloakConfigMapTemplate(fakeNs)
	assert.Equal(t, cm.Name, "${APPLICATION_NAME}-service-ca")
	assert.Equal(t, cm.Namespace, fakeNs)
}

func TestNewKeycloakTemplate_testService(t *testing.T) {
	svc := getKeycloakServiceTemplate(fakeNs)
	assert.Equal(t, svc.Name, "${APPLICATION_NAME}")
	assert.Equal(t, svc.Namespace, fakeNs)
	assert.Equal(t, svc.Spec.Selector, map[string]string{
		"deploymentConfig": "${APPLICATION_NAME}"})
}

func TestNewKeycloakTemplate_testRoute(t *testing.T) {
	route := getKeycloakRouteTemplate(fakeNs)
	assert.Equal(t, route.Name, "${APPLICATION_NAME}")
	assert.Equal(t, route.Namespace, fakeNs)
	assert.Equal(t, route.Spec.To,
		routev1.RouteTargetReference{Name: "${APPLICATION_NAME}"})
	assert.Equal(t, route.Spec.TLS,
		&routev1.TLSConfig{Termination: "reencrypt"})
}

func TestKeycloak_testRealmConfigCreation(t *testing.T) {
	cfg := &keycloakConfig{
		ArgoName:      "foo-argocd",
		ArgoNamespace: "foo",
		Username:      "test-user",
		Password:      "test",
		KeycloakURL:   "https://foo.keycloak.com",
		ArgoCDURL:     "https://bar.argocd.com",
	}

	_, err := createRealmConfig(cfg)
	assert.NoError(t, err)
}

func TestKeycloak_testServerCert(t *testing.T) {

	a := makeTestArgoCDForKeycloak()

	resObjs := []client.Object{a}
	subresObjs := []client.Object{a}
	runtimeObjs := []runtime.Object{}
	sch := makeTestReconcilerScheme(argoprojv1alpha1.AddToScheme, templatev1.AddToScheme, oappsv1.AddToScheme, routev1.AddToScheme)
	cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
	r := makeTestReconciler(cl, sch)

	sslCertsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      servingCertSecretName,
			Namespace: a.Namespace,
		},
		Data: map[string][]byte{
			"tls.crt": []byte("asdasfsff"),
		},
	}
	r.Client.Create(context.TODO(), sslCertsSecret)

	_, err := r.getKCServerCert(a)
	assert.NoError(t, err)

	sslCertsSecret.Data["tls.crt"] = nil
	assert.NoError(t, r.Client.Update(context.TODO(), sslCertsSecret))

	_, err = r.getKCServerCert(a)
	assert.NoError(t, err)
}

func TestKeycloakConfigVerifyTLSForOpenShift(t *testing.T) {
	tests := []struct {
		name             string
		argoCD           *v1alpha1.ArgoCD
		desiredVerifyTLS bool
	}{
		{
			name: ".spec.sso.verifyTLS & .spec.sso.keycloak.verifyTLS both nil",
			argoCD: makeTestArgoCD(func(ac *argoprojv1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
				}
			}),
			desiredVerifyTLS: true,
		},
		{
			name: ".spec.sso.verifyTLS nil, .spec.sso.keycloak.verifyTLS false",
			argoCD: makeTestArgoCD(func(ac *argoprojv1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						VerifyTLS: boolPtr(false),
					},
				}
			}),
			desiredVerifyTLS: false,
		},
		{
			name: ".spec.sso.verifyTLS false, .spec.sso.keycloak.verifyTLS nil",
			argoCD: makeTestArgoCD(func(ac *argoprojv1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider:  argoappv1.SSOProviderTypeKeycloak,
					VerifyTLS: boolPtr(false),
				}
			}),
			desiredVerifyTLS: false,
		},
		{
			name: ".spec.sso.verifyTLS nil, .spec.sso.keycloak.verifyTLS true",
			argoCD: makeTestArgoCD(func(ac *argoprojv1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider: argoappv1.SSOProviderTypeKeycloak,
					Keycloak: &v1alpha1.ArgoCDKeycloakSpec{
						VerifyTLS: boolPtr(true),
					},
				}
			}),
			desiredVerifyTLS: true,
		},
		{
			name: ".spec.sso.verifyTLS true, .spec.sso.keycloak.verifyTLS nil",
			argoCD: makeTestArgoCD(func(ac *argoprojv1alpha1.ArgoCD) {
				ac.Spec.SSO = &v1alpha1.ArgoCDSSOSpec{
					Provider:  argoappv1.SSOProviderTypeKeycloak,
					VerifyTLS: boolPtr(true),
				}
			}),
			desiredVerifyTLS: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			resObjs := []client.Object{test.argoCD}
			subresObjs := []client.Object{test.argoCD}
			runtimeObjs := []runtime.Object{}
			sch := makeTestReconcilerScheme(argoprojv1alpha1.AddToScheme, templatev1.AddToScheme, oappsv1.AddToScheme, routev1.AddToScheme)
			cl := makeTestReconcilerClient(sch, resObjs, subresObjs, runtimeObjs)
			r := makeTestReconciler(cl, sch)

			keycloakRoute := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      defaultKeycloakIdentifier,
					Namespace: test.argoCD.Namespace,
				},
				Spec: routev1.RouteSpec{
					Host: "test-host",
				},
			}
			r.Client.Create(context.TODO(), keycloakRoute)

			argoCDRoute := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", test.argoCD.Name, "server"),
					Namespace: test.argoCD.Namespace,
				},
				Spec: routev1.RouteSpec{
					Host: "test-argocd-host",
				},
			}
			r.Client.Create(context.TODO(), argoCDRoute)

			keycloakSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-%s", defaultKeycloakIdentifier, "secret"),
					Namespace: test.argoCD.Namespace,
				},
				Data: map[string][]byte{"SSO_USERNAME": []byte("username"), "SSO_PASSWORD": []byte("password")},
			}
			r.Client.Create(context.TODO(), keycloakSecret)

			sslCertsSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      servingCertSecretName,
					Namespace: test.argoCD.Namespace,
				},
				Data: map[string][]byte{
					"tls.crt": []byte("asdasfsff"),
				},
			}
			r.Client.Create(context.TODO(), sslCertsSecret)

			keyCloakConfig, err := r.prepareKeycloakConfig(test.argoCD)
			assert.NoError(t, err)

			assert.Equal(t, test.desiredVerifyTLS, keyCloakConfig.VerifyTLS)
		})
	}
}

func TestKeycloak_NodeLabelSelector(t *testing.T) {
	a := makeTestArgoCDForKeycloak()
	a.Spec.NodePlacement = &argoappv1.ArgoCDNodePlacementSpec{
		NodeSelector: deploymentDefaultNodeSelector(),
		Tolerations:  deploymentDefaultTolerations(),
	}

	dc := getKeycloakDeploymentConfigTemplate(a)

	nSelectors := deploymentDefaultNodeSelector()
	nSelectors = argoutil.AppendStringMap(nSelectors, common.DefaultNodeSelector())
	assert.Equal(t, dc.Spec.Template.Spec.NodeSelector, nSelectors)
	assert.Equal(t, dc.Spec.Template.Spec.Tolerations, a.Spec.NodePlacement.Tolerations)
}

func removeTemplateAPI() {
	templateAPIFound = false
}
