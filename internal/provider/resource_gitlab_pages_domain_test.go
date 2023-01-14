//go:build acceptance
// +build acceptance

package provider

import (
	"errors"
	"fmt"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"gitlab.com/gitlab-org/terraform-provider-gitlab/internal/provider/client"
	"gitlab.com/gitlab-org/terraform-provider-gitlab/internal/provider/testutil"
)

func TestAcc_GitlabPagesDomain_basic(t *testing.T) {

	// Set up project environment.
	project := testutil.CreateProject(t)
	domain := "example.com"

	resource.ParallelTest(t, resource.TestCase{
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		CheckDestroy:             testAcc_GitlabPagesDomain_CheckDestroy(project.ID, domain),
		Steps: []resource.TestStep{
			// Create a basic protected environment.
			{
				Config: fmt.Sprintf(`
				resource "gitlab_pages_domain" "this" {
					project     = %d
					domain      = "%s"
				}`, project.ID, domain),

				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("gitlab_pages_domain.this", "project"),
					resource.TestCheckResourceAttrSet("gitlab_pages_domain.this", "domain"),
				),
			},
			// Verify upstream attributes with an import.
			{
				ResourceName:      "gitlab_pages_domain.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
			// Add optional attributes
			{
				Config: fmt.Sprintf(`
				resource "gitlab_pages_domain" "this" {
					project     = %d
					domain      = %s

					auto_ssl_enabled = true
					key              = "example-key"

					certificate {
						certificate = "example-certificate"
					}
				}`, project.ID, domain),

				// Check computed attributes.
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("gitlab_pages_domain.this", "certificate.expired"),
				),
			},
			// Verify upstream attributes with an import.
			{
				ResourceName:      "gitlab_pages_domain.this",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func testAcc_GitlabPagesDomain_CheckDestroy(projectID int, domain string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		_, _, err := testutil.TestGitlabClient.PagesDomains.GetPagesDomain(projectID, domain)
		if err == nil {
			return errors.New("Pages Domain still exists")
		}
		if !client.Is404(err) {
			return fmt.Errorf("Error calling API to get the Pages Domain: %w", err)
		}
		return nil
	}
}
