//go:build acceptance
// +build acceptance

package provider

import (
	"fmt"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/acctest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
)

func TestAccDataSourceGitlabUsers_basic(t *testing.T) {
	rInt := acctest.RandInt()
	testAccCreateUsersWithPrefix(t, 12, fmt.Sprintf("ds-%d-acctest-a-", rInt))
	testUsersGroupB := testAccCreateUsersWithPrefix(t, 12, fmt.Sprintf("ds-%d-acctest-b-", rInt))

	resource.ParallelTest(t, resource.TestCase{
		ProviderFactories: providerFactories,
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(`
					data "gitlab_users" "test" {
					  search = "ds-%d-acctest-"

					  sort     = "desc"
					  order_by = "name"
					}
				`, rInt),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.gitlab_users.test", "users.#", "24"),
					resource.TestCheckResourceAttrWith("data.gitlab_users.test", "users.0.username", func(value string) error {
						if !strings.HasPrefix(value, fmt.Sprintf("ds-%d-acctest-b-", rInt)) {
							return fmt.Errorf("expected first user to be of group a with prefix `ds-acctest-a-` got `%s` instead", value)
						}
						return nil
					}),
				),
			},
			{
				Config: fmt.Sprintf(`
					data "gitlab_users" "test" {
					  search = "ds-%d-acctest-b-"
					}
				`, rInt),
				Check: resource.TestCheckResourceAttr("data.gitlab_users.test", "users.#", fmt.Sprintf("%d", len(testUsersGroupB))),
			},
		},
	})
}
