// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This connector is wired into `main.go` and `cmd/run-once/main.go` in a future spec; this file is standalone.

package pkg

import (
	"context"

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

// ConfigBuilder is the test seam for loading the rest.Config. Production
// wiring passes rest.InClusterConfig; tests pass a closure that returns
// a zero-value *rest.Config so SetupCustomResourceDefinition runs without
// a real in-cluster service-account token mount.
type ConfigBuilder func() (*rest.Config, error)

// CRDClientBuilder is the test seam for constructing the apiextensions
// clientset. Production wiring passes apiextensionsclient.NewForConfig;
// tests wire it to a closure that returns the fake clientset.
type CRDClientBuilder func(*rest.Config) (apiextensionsclient.Interface, error)

// K8sConnector installs the Schedule CRD on a Kubernetes cluster.
//
//counterfeiter:generate -o ../mocks/k8s_connector.go --fake-name FakeK8sConnector . K8sConnector
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
	config, err := k.configBuilder()
	if err != nil {
		return errors.Wrap(ctx, err, "build k8s config")
	}
	clientset, err := k.clientBuilder(config)
	if err != nil {
		return errors.Wrap(ctx, err, "build apiextensions clientset")
	}

	crdClient := clientset.ApiextensionsV1().CustomResourceDefinitions()
	existing, err := crdClient.Get(ctx, v1.Plural+"."+v1.GroupName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return k.createCrd(ctx, crdClient)
	}
	if err != nil {
		return errors.Wrap(ctx, err, "get CRD")
	}
	existing.Spec = k.desiredCRDSpec()
	if _, err := crdClient.Update(ctx, existing, metav1.UpdateOptions{}); err != nil {
		return errors.Wrapf(ctx, err, "update CRD %s.%s", v1.Plural, v1.GroupName)
	}
	glog.V(2).Infof("k8s-connector: updated CRD %s.%s", v1.Plural, v1.GroupName)
	return nil
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
			// Race: another pod beat us to it. Fall through to update path.
			glog.V(2).Infof("k8s-connector: crd-already-exists: applying update")
			return nil
		}
		return errors.Wrapf(ctx, err, "create CRD %s.%s", v1.Plural, v1.GroupName)
	}
	glog.V(2).Infof("k8s-connector: created CRD %s.%s", v1.Plural, v1.GroupName)
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
				OpenAPIV3Schema: k.scheduleSpecSchemaPtr(),
			},
		}},
	}
}
