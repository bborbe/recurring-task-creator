// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pkg_test

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsfake "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/testing"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
	"github.com/bborbe/recurring-task-creator/pkg"
)

var _ = Describe("SetupCustomResourceDefinition", func() {
	var (
		ctx           context.Context
		clientset     *apiextensionsfake.Clientset
		clientBuilder pkg.CRDClientBuilder
		configBuilder pkg.ConfigBuilder
		connector     pkg.K8sConnector
	)

	BeforeEach(func() {
		ctx = context.Background()
		configBuilder = func() (*rest.Config, error) { return &rest.Config{}, nil }
		clientset = apiextensionsfake.NewSimpleClientset()
		clientBuilder = func(_ *rest.Config) (apiextensionsclient.Interface, error) {
			return clientset, nil
		}
		connector = pkg.NewK8sConnector(configBuilder, clientBuilder)
	})

	It("creates the CRD when none exists", func() {
		Expect(connector.SetupCustomResourceDefinition(ctx)).To(Succeed())

		crdList, err := clientset.ApiextensionsV1().
			CustomResourceDefinitions().
			List(ctx, metav1.ListOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(crdList.Items).To(HaveLen(1))
		crd := crdList.Items[0]
		Expect(crd.Spec.Group).To(Equal(v1.GroupName))
		Expect(crd.Spec.Names.Kind).To(Equal(v1.Kind))
		Expect(crd.Spec.Names.Plural).To(Equal(v1.Plural))
	})

	It("updates the CRD when an older spec exists", func() {
		// Pre-load the fake with a CRD whose spec has the same group/kind
		// but a deliberately wrong Versions[0].Name — the test asserts the
		// connector overwrites it.
		old := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: v1.Plural + "." + v1.GroupName},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: v1.GroupName,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:     v1.Kind,
					Plural:   v1.Plural,
					Singular: v1.Singular,
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
					Name:    "WRONG-VERSION",
					Served:  true,
					Storage: true,
				}},
			},
		}
		clientset = apiextensionsfake.NewSimpleClientset(old)
		clientBuilder = func(_ *rest.Config) (apiextensionsclient.Interface, error) { return clientset, nil }
		connector = pkg.NewK8sConnector(configBuilder, clientBuilder)

		Expect(connector.SetupCustomResourceDefinition(ctx)).To(Succeed())

		// After update, the spec's Versions[0].Name must be v1.Version.
		updated, err := clientset.ApiextensionsV1().
			CustomResourceDefinitions().
			Get(ctx, old.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(updated.Spec.Versions).To(HaveLen(1))
		Expect(updated.Spec.Versions[0].Name).To(Equal(v1.Version))
	})

	It("wraps an error when the clientset builder fails", func() {
		clientBuilder = func(_ *rest.Config) (apiextensionsclient.Interface, error) {
			return nil, errors.New("boom")
		}
		connector = pkg.NewK8sConnector(configBuilder, clientBuilder)

		err := connector.SetupCustomResourceDefinition(ctx)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("build apiextensions clientset"))
	})

	It("reconciles via update when create sees an AlreadyExists race", func() {
		// Simulate the real race: another pod already wrote the CRD with
		// a different version slot. The local Get races and returns NotFound
		// (its read predates Pod A's write — the reactor below makes the
		// first Get return NotFound, then injects Pod A's CRD before the
		// second Get fires from updateCrd's re-fetch). On Create the API
		// returns AlreadyExists; the connector must fall through to update
		// so Pod B's desiredCRDSpec wins, not Pod A's stale slot.
		raceWinner := &apiextensionsv1.CustomResourceDefinition{
			ObjectMeta: metav1.ObjectMeta{Name: v1.Plural + "." + v1.GroupName},
			Spec: apiextensionsv1.CustomResourceDefinitionSpec{
				Group: v1.GroupName,
				Names: apiextensionsv1.CustomResourceDefinitionNames{
					Kind:     v1.Kind,
					Plural:   v1.Plural,
					Singular: v1.Singular,
				},
				Scope: apiextensionsv1.NamespaceScoped,
				Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
					Name:    "STALE-RACE-WINNER-VERSION",
					Served:  true,
					Storage: true,
				}},
			},
		}
		clientset = apiextensionsfake.NewSimpleClientset()
		clientset.PrependReactor(
			"create",
			"customresourcedefinitions",
			func(_ testing.Action) (bool, runtime.Object, error) {
				// Pod A finished its write between our Get and our Create.
				_ = clientset.Tracker().Add(raceWinner)
				return true, nil, &apierrors.StatusError{ErrStatus: metav1.Status{
					Status:  metav1.StatusFailure,
					Code:    409,
					Reason:  metav1.StatusReasonAlreadyExists,
					Message: "race: another pod created the CRD first",
				}}
			},
		)
		clientBuilder = func(_ *rest.Config) (apiextensionsclient.Interface, error) {
			return clientset, nil
		}
		connector = pkg.NewK8sConnector(configBuilder, clientBuilder)

		Expect(connector.SetupCustomResourceDefinition(ctx)).To(Succeed())

		updated, err := clientset.ApiextensionsV1().CustomResourceDefinitions().Get(
			ctx, raceWinner.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(updated.Spec.Versions).To(HaveLen(1))
		Expect(updated.Spec.Versions[0].Name).To(Equal(v1.Version),
			"AlreadyExists fall-through must reconcile to desired spec")
	})
})
