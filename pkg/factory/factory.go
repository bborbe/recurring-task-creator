// Copyright (c) 2025 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package factory

import (
	"net/http"

	"github.com/bborbe/agent/lib/command/task"
	libsentry "github.com/bborbe/sentry"

	"github.com/bborbe/recurring-task-creator/pkg/handler"
	"github.com/bborbe/recurring-task-creator/pkg/publisher"
)

// CreateTestLoglevelHandler creates an HTTP handler that tests different glog verbosity levels.
func CreateTestLoglevelHandler() http.Handler {
	return handler.NewTestLoglevelHandler()
}

// CreateSentryAlertHandler creates an HTTP handler that sends test alerts to Sentry.
func CreateSentryAlertHandler(sentryClient libsentry.Client) http.Handler {
	return handler.NewSentryAlertHandler(sentryClient)
}

// CreatePublisher builds a publisher.Publisher that sends through the
// given task.CreateCommandSender. Pure plumbing: no business logic.
func CreatePublisher(sender task.CreateCommandSender) publisher.Publisher {
	return publisher.NewPublisher(sender)
}
