// Copyright (c) 2026 Benjamin Borbe All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package publisher_test

import (
	"context"

	lib "github.com/bborbe/agent"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bborbe/recurring-task-creator/pkg/publisher"
)

var _ = Describe("ModeRoleRule", func() {
	DescribeTable(
		"accepts or refuses the mode/role pairing matrix",
		func(frontmatter lib.TaskFrontmatter, wantErr bool) {
			err := (publisher.ModeRoleRule{
				Slug:        "shutdown-k3s",
				Frontmatter: frontmatter,
			}).Validate(context.Background())
			if wantErr {
				Expect(err).To(HaveOccurred())
				return
			}
			Expect(err).NotTo(HaveOccurred())
		},
		Entry(
			"interactive with role absent is refused",
			lib.TaskFrontmatter{"mode": "interactive"},
			true,
		),
		Entry(
			"interactive with role agent is refused",
			lib.TaskFrontmatter{"mode": "interactive", "role": "agent"},
			true,
		),
		Entry(
			"interactive with a placeholder-looking role is refused literally",
			lib.TaskFrontmatter{"mode": "interactive", "role": "{{iso-week}}"},
			true,
		),
		Entry(
			"interactive with role human is accepted",
			lib.TaskFrontmatter{"mode": "interactive", "role": "human"},
			false,
		),
		Entry(
			"interactive with role manager is accepted",
			lib.TaskFrontmatter{"mode": "interactive", "role": "manager"},
			false,
		),
		Entry(
			"no mode with role agent is accepted",
			lib.TaskFrontmatter{"role": "agent"},
			false,
		),
		Entry(
			"headless with role agent is accepted",
			lib.TaskFrontmatter{"mode": "headless", "role": "agent"},
			false,
		),
		Entry("nil frontmatter is accepted", nil, false),
		Entry(
			"non-string mode (bool) is accepted",
			lib.TaskFrontmatter{"mode": true},
			false,
		),
		Entry(
			"non-string mode (int) is accepted",
			lib.TaskFrontmatter{"mode": 1},
			false,
		),
		Entry(
			"non-string mode (list) is accepted",
			lib.TaskFrontmatter{"mode": []interface{}{"interactive"}},
			false,
		),
		Entry(
			"non-string role (int) imposes no constraint",
			lib.TaskFrontmatter{"mode": "interactive", "role": 1},
			false,
		),
		Entry(
			"non-string role (bool) imposes no constraint",
			lib.TaskFrontmatter{"mode": "interactive", "role": true},
			false,
		),
		Entry(
			"non-string role (list) imposes no constraint",
			lib.TaskFrontmatter{"mode": "interactive", "role": []interface{}{"human"}},
			false,
		),
		Entry(
			"explicit null role is refused like an absent role",
			lib.TaskFrontmatter{"mode": "interactive", "role": nil},
			true,
		),
	)

	It("names the slug and reports an absent role", func() {
		err := (publisher.ModeRoleRule{
			Slug:        "shutdown-k3s",
			Frontmatter: lib.TaskFrontmatter{"mode": "interactive"},
		}).Validate(context.Background())
		Expect(err).To(MatchError(ContainSubstring("shutdown-k3s")))
		Expect(err.Error()).To(ContainSubstring("interactive"))
		Expect(err.Error()).To(ContainSubstring("absent"))
	})

	It("names the slug and the agent role", func() {
		err := (publisher.ModeRoleRule{
			Slug:        "shutdown-k3s",
			Frontmatter: lib.TaskFrontmatter{"mode": "interactive", "role": "agent"},
		}).Validate(context.Background())
		Expect(err).To(MatchError(ContainSubstring("shutdown-k3s")))
		Expect(err.Error()).To(ContainSubstring("interactive"))
		Expect(err.Error()).To(ContainSubstring("agent"))
	})

	It("takes the slug from the input rather than hardcoding it", func() {
		err := (publisher.ModeRoleRule{
			Slug:        "weekly-review",
			Frontmatter: lib.TaskFrontmatter{"mode": "interactive"},
		}).Validate(context.Background())
		Expect(err.Error()).To(ContainSubstring("weekly-review"))
		Expect(err.Error()).NotTo(ContainSubstring("shutdown-k3s"))
	})
})
