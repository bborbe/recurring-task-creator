// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg

import (
	"context"
	"time"

	"github.com/bborbe/errors"
	"github.com/golang/glog"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsv1typed "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
)

// crdSetupTimeout bounds the CRD install + reconcile sequence so an
// unreachable API server cannot wedge binary startup. 30s covers a slow
// kube-apiserver cold-cache; longer would block Kafka/HTTP boot for no
// gain.
const crdSetupTimeout = 30 * time.Second

// K8sConnector is shipped by Spec 008 (scaffolding) but is intentionally
// NOT yet wired into main.go / cmd/run-once/main.go. The Run() integration
// — and the informer that consumes Schedule CRs — lands in Spec B per
// 008's Non-goals ("DOES NOT introduce an informer / Listen wiring"). The
// type, schema, mock, and tests ship here so Spec B is a pure wiring change.

// ConfigBuilder is the test seam for loading the rest.Config. Production
// wiring passes rest.InClusterConfig; tests pass a closure that returns
// a zero-value *rest.Config so SetupCustomResourceDefinition runs without
// a real in-cluster service-account token mount.
type ConfigBuilder func() (*rest.Config, error)

// CRDClientBuilder is the test seam for constructing the apiextensions
// clientset. Production wiring passes apiextensionsclient.NewForConfig;
// tests wire it to a closure that returns the fake clientset.
type CRDClientBuilder func(*rest.Config) (apiextensionsclient.Interface, error)

//counterfeiter:generate -o ../mocks/k8s_connector.go --fake-name FakeK8sConnector . K8sConnector

// K8sConnector installs the Schedule CRD on a Kubernetes cluster.
type K8sConnector interface {
	SetupCustomResourceDefinition(ctx context.Context) error
}

// NewK8sConnector builds a connector that uses configBuilder to load
// the k8s config and clientBuilder to construct the apiextensions
// clientset. Production wiring passes rest.InClusterConfig +
// apiextensionsclient.NewForConfig; tests pass closures returning
// stubs/fakes.
func NewK8sConnector(configBuilder ConfigBuilder, clientBuilder CRDClientBuilder) K8sConnector {
	return &k8sConnector{configBuilder: configBuilder, clientBuilder: clientBuilder}
}

type k8sConnector struct {
	configBuilder ConfigBuilder
	clientBuilder CRDClientBuilder
}

func (k *k8sConnector) SetupCustomResourceDefinition(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, crdSetupTimeout)
	defer cancel()
	config, err := k.configBuilder()
	if err != nil {
		return errors.Wrap(ctx, err, "build k8s config")
	}
	clientset, err := k.clientBuilder(config)
	if err != nil {
		return errors.Wrap(ctx, err, "build apiextensions clientset")
	}

	crdClient := clientset.ApiextensionsV1().CustomResourceDefinitions()
	_, err = crdClient.Get(ctx, v1.Plural+"."+v1.GroupName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return k.createCrd(ctx, crdClient)
	}
	if err != nil {
		return errors.Wrap(ctx, err, "get CRD")
	}
	return k.updateCrd(ctx, crdClient)
}

func (k *k8sConnector) createCrd(
	ctx context.Context,
	crdClient apiextensionsv1typed.CustomResourceDefinitionInterface,
) error {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{Name: v1.Plural + "." + v1.GroupName},
		Spec:       k.desiredCRDSpec(),
	}
	if _, err := crdClient.Create(ctx, crd, metav1.CreateOptions{}); err != nil {
		if apierrors.IsAlreadyExists(err) {
			glog.V(2).Infof("k8s-connector: crd-already-exists: applying update")
			return k.updateCrd(ctx, crdClient)
		}
		return errors.Wrapf(ctx, err, "create CRD %s.%s", v1.Plural, v1.GroupName)
	}
	glog.V(2).Infof("k8s-connector: created CRD %s.%s", v1.Plural, v1.GroupName)
	return nil
}

func (k *k8sConnector) updateCrd(
	ctx context.Context,
	crdClient apiextensionsv1typed.CustomResourceDefinitionInterface,
) error {
	existing, err := crdClient.Get(ctx, v1.Plural+"."+v1.GroupName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrap(ctx, err, "get CRD")
	}
	// Replace the whole spec rather than deep-merging. This binary owns the
	// CRD's schema, scope, names, and versions: any divergence between the
	// running pod's desiredCRDSpec() and what's on the cluster is exactly
	// what this reconcile is supposed to flatten. Operator-added Spec fields
	// (e.g. extra Versions, additionalPrinterColumns) are out of scope for
	// v1 — operators that need them should fork desiredCRDSpec or use
	// server-side apply with a different field manager.
	existing.Spec = k.desiredCRDSpec()
	if _, err := crdClient.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return errors.Wrapf(ctx, err, "update CRD %s.%s", v1.Plural, v1.GroupName)
	}
	glog.V(2).Infof("k8s-connector: updated CRD %s.%s", v1.Plural, v1.GroupName)
	return nil
}

// desiredCRDSpec returns the Go-built CRD spec for the Schedule CRD.
// Every value (group, kind, plural, singular, short name, scope) is
// read from the v1 package's frozen constants — the connector does not
// hard-code any of the names. Renaming a constant in v1 is the only way
// the CRD spec changes; this is the single source of truth.
func (k *k8sConnector) desiredCRDSpec() apiextensionsv1.CustomResourceDefinitionSpec {
	return apiextensionsv1.CustomResourceDefinitionSpec{
		Group: v1.GroupName,
		Names: apiextensionsv1.CustomResourceDefinitionNames{
			Kind:       v1.Kind,
			ListKind:   v1.ListKind,
			Plural:     v1.Plural,
			Singular:   v1.Singular,
			ShortNames: v1.ShortNames,
		},
		Scope: apiextensionsv1.NamespaceScoped,
		Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
			Name:    v1.Version,
			Served:  true,
			Storage: true,
			Schema: &apiextensionsv1.CustomResourceValidation{
				OpenAPIV3Schema: scheduleCRSchemaPtr(),
			},
		}},
	}
}
