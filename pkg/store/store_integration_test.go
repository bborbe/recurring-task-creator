// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package store_test

import (
	"context"
	stderrors "errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"

	v1 "github.com/bborbe/recurring-task-creator/k8s/apis/task.benjamin-borbe.de/v1"
	fake "github.com/bborbe/recurring-task-creator/k8s/client/clientset/versioned/fake"
	externalversions "github.com/bborbe/recurring-task-creator/k8s/client/informers/externalversions"
	listersv1 "github.com/bborbe/recurring-task-creator/k8s/client/listers/task.benjamin-borbe.de/v1"
	"github.com/bborbe/recurring-task-creator/pkg/schedule"
	"github.com/bborbe/recurring-task-creator/pkg/store"
)

const testNamespace = "test-ns"

var _ = Describe("ScheduleStore", func() {
	Describe("List via informer (integration)", func() {
		var (
			ctx    context.Context
			cancel context.CancelFunc
		)

		BeforeEach(func() {
			ctx, cancel = context.WithCancel(context.Background())
		})

		AfterEach(func() {
			cancel()
		})

		It("returns only the valid CR, dropping the invalid-recurrence CR", func() {
			validCR := &v1.Schedule{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1.GroupName + "/" + v1.Version,
					Kind:       v1.Kind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "valid-slug",
					Namespace: testNamespace,
				},
				Spec: v1.ScheduleSpec{
					Title: "Valid Task",
					Schedule: v1.ScheduleTrigger{
						Recurrence: "Weekday",
						Weekday:    "Friday",
					},
					Template: v1.ScheduleTemplate{Body: "some body"},
				},
			}
			invalidCR := &v1.Schedule{
				TypeMeta: metav1.TypeMeta{
					APIVersion: v1.GroupName + "/" + v1.Version,
					Kind:       v1.Kind,
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "invalid-slug",
					Namespace: testNamespace,
				},
				Spec: v1.ScheduleSpec{
					Title: "Bad Task",
					Schedule: v1.ScheduleTrigger{
						Recurrence: "Bogus",
					},
				},
			}

			client := fake.NewSimpleClientset(validCR, invalidCR)
			factory := externalversions.NewSharedInformerFactoryWithOptions(
				client, 0,
				externalversions.WithNamespace(testNamespace),
			)

			informer := factory.Task().V1().Schedules().Informer()
			lister := factory.Task().V1().Schedules().Lister()

			factory.StartWithContext(ctx)

			Eventually(informer.HasSynced, 5*time.Second, 50*time.Millisecond).Should(BeTrue())

			s := store.NewScheduleStore(lister, testNamespace)
			defs, err := s.List(ctx)

			Expect(err).NotTo(HaveOccurred())
			Expect(defs).To(HaveLen(1))
			Expect(defs[0].Slug).To(Equal("valid-slug"))
			Expect(defs[0].Recurrence).To(Equal(schedule.RecurrenceWeekday))
			Expect(defs[0].Weekdays).To(Equal([]time.Weekday{time.Friday}))
		})
	})

	Describe("List with lister error", func() {
		It("returns a wrapped error when the lister fails", func() {
			sentinel := stderrors.New("cache not ready")
			lister := &stubScheduleLister{
				namespaceLister: &stubScheduleNamespaceLister{err: sentinel},
			}
			s := store.NewScheduleStore(lister, testNamespace)

			_, err := s.List(context.Background())

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("list schedules from informer cache"))
			Expect(err.Error()).To(ContainSubstring("cache not ready"))
		})
	})
})

// stubScheduleLister is a minimal in-test lister that delegates to a
// namespace lister returning a fixed error. It satisfies the
// listersv1.ScheduleLister interface without requiring a full informer.
type stubScheduleLister struct {
	namespaceLister *stubScheduleNamespaceLister
}

func (s *stubScheduleLister) List(_ labels.Selector) ([]*v1.Schedule, error) {
	return nil, nil
}

func (s *stubScheduleLister) Schedules(_ string) listersv1.ScheduleNamespaceLister {
	return s.namespaceLister
}

// stubScheduleNamespaceLister always returns the configured error.
type stubScheduleNamespaceLister struct {
	err error
}

func (s *stubScheduleNamespaceLister) List(_ labels.Selector) ([]*v1.Schedule, error) {
	return nil, s.err
}

func (s *stubScheduleNamespaceLister) Get(_ string) (*v1.Schedule, error) {
	return nil, s.err
}
