package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"github.com/xanzy/go-gitlab"
	"gitlab.com/gitlab-org/terraform-provider-gitlab/internal/provider/client"
	"gitlab.com/gitlab-org/terraform-provider-gitlab/internal/provider/utils"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ resource.Resource                = &gitlabPagesDomainResource{}
	_ resource.ResourceWithConfigure   = &gitlabPagesDomainResource{}
	_ resource.ResourceWithImportState = &gitlabPagesDomainResource{}
)

func init() {
	registerResource(NewGitLabPagesDomainResource)
}

func NewGitLabPagesDomainResource() resource.Resource {
	return &gitlabPagesDomainResource{}
}

type gitlabPagesDomainResource struct {
	client *gitlab.Client
}

type gitLabPagesDomainResourceModel struct {
	ID              string `tfsdk:"id"`
	Domain          string `tfsdk:"domain"`
	Project         string `tfsdk:"project"`
	AutoSslEnabled  bool   `tfsdk:"auto_ssl_enabled"`
	Key             string `tfsdk:"key"`
	URL             string `tfsdk:"url"`
	CertificateData struct {
		Certificate string `tfsdk:"certificate"`
		Expired     bool   `tfsdk:"expired"`
	} `tfsdk:"certificate"`
	Verified           bool   `tfsdk:"verified"`
	VerificationString string `tfsdk:"verification_code"`
}

// Metadata returns the resource name
func (d *gitlabPagesDomainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_pages_domain"
}

// Schema defines the schema for the resource
func (d *gitlabPagesDomainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `The ` + "`gitlab_pages_domain`" + ` resource allows connecting custom domains and TLS certificates in GitLab Pages.

**Upstream API**: [GitLab REST API docs](https://docs.gitlab.com/ee/api/pages_domains.html)`,

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of this Terraform resource, in the format of project:domain",
				PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"domain": schema.StringAttribute{
				MarkdownDescription: "The custom domain indicated by the user.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"project": schema.StringAttribute{
				MarkdownDescription: "The ID or [URL-encoded path of the project](https://docs.gitlab.com/ee/api/index.html#namespaced-path-encoding) owned by the authenticated user.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"auto_ssl_enabled": schema.BoolAttribute{
				MarkdownDescription: "Enables [automatic generation](https://docs.gitlab.com/ee/user/project/pages/custom_domains_ssl_tls_certification/lets_encrypt_integration.html) of SSL certificates issued by Let’s Encrypt for custom domains.",
				Optional:            true,
			},
			"key": schema.StringAttribute{
				MarkdownDescription: "The certificate key in PEM format.",
				Optional:            true,
			},
			"url": schema.StringAttribute{
				MarkdownDescription: "The URL for the given domain.",
				Computed:            true,
			},
			"verified": schema.BoolAttribute{
				MarkdownDescription: "The certificate data.",
				Computed:            true,
			},
			"verification_code": schema.StringAttribute{
				MarkdownDescription: "The verification code for the domain.",
				Computed:            true,
				Sensitive:           true,
			},
		},
		Blocks: map[string]schema.Block{
			// all blocks are `optional`, even when not set as such.
			"certificate": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"certificate": schema.StringAttribute{
						MarkdownDescription: "The certificate in PEM format with intermediates following in most specific to least specific order.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
					},
					"expired": schema.BoolAttribute{
						MarkdownDescription: "Whether the certificate is expired.",
						Optional:            true,
						Computed:            true,
						PlanModifiers:       []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
					},
				},
			},
		},
	}
}

// Configure adds the client implementation to the resource
func (d *gitlabPagesDomainResource) Configure(_ context.Context, req resource.ConfigureRequest, _ *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	d.client = req.ProviderData.(*gitlab.Client)
}

func (d *gitlabPagesDomainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	// Get plan information into our struct
	var data gitLabPagesDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create our resource
	options := &gitlab.CreatePagesDomainOptions{
		Domain: &data.Domain,
	}
	if data.AutoSslEnabled {
		options.AutoSslEnabled = &data.AutoSslEnabled
	}
	if data.CertificateData.Certificate != "" {
		options.Certificate = &data.CertificateData.Certificate
	}
	if data.Key != "" {
		options.Key = &data.Key
	}

	pagesDomain, _, err := d.client.PagesDomains.CreatePagesDomain(data.Project, options)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error creating pages domain for project %s", data.Project),
			err.Error(),
		)
		return
	}

	data.pagesDomainToStateModel(pagesDomain)

	// Create the ID attribute (used for imports, among other things)
	data.ID = utils.BuildTwoPartID(&data.Project, &data.Domain)

	tflog.Debug(ctx, "created pages domain", map[string]interface{}{
		"url": data.URL, "project": data.Project,
	})

	// Set our plan object into state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *gitlabPagesDomainResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data gitLabPagesDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, domain, err := utils.ParseTwoPartID(data.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid resource ID format",
			fmt.Sprintf("The resource ID '%s' has an invalid format. It should be '<project>:<domain>'. Error: %s", data.ID, err.Error()),
		)
		return
	}

	pagesDomain, _, err := d.client.PagesDomains.GetPagesDomain(projectID, domain)
	if err != nil {
		if client.Is404(err) {
			tflog.Debug(ctx, "pages domain doesn't exist, removing from state", map[string]interface{}{
				"url": data.URL, "project": data.Project,
			})
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("GitLab API error occured", fmt.Sprintf("Unable to read pages domain details: %s", err.Error()))
		return
	}

	data.pagesDomainToStateModel(pagesDomain)
}

func (d *gitlabPagesDomainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

	// Get data information into our struct
	var data gitLabPagesDomainResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Create our resource
	options := &gitlab.UpdatePagesDomainOptions{}
	if data.AutoSslEnabled {
		options.AutoSslEnabled = &data.AutoSslEnabled
	}
	if data.CertificateData.Certificate != "" {
		options.Certificate = &data.CertificateData.Certificate
	}
	if data.Key != "" {
		options.Key = &data.Key
	}

	pagesDomain, _, err := d.client.PagesDomains.UpdatePagesDomain(data.Project, data.Domain, options)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Error creating pages domain for project %s", data.Project),
			err.Error(),
		)
		return
	}

	data.pagesDomainToStateModel(pagesDomain)

	// Create the ID attribute (used for imports, among other things)
	data.ID = utils.BuildTwoPartID(&data.Project, &data.Domain)

	tflog.Debug(ctx, "updated pages domain", map[string]interface{}{
		"url": data.URL, "project": data.Project,
	})

	// Set our plan object into state
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (d *gitlabPagesDomainResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data gitLabPagesDomainResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectID, domain, err := utils.ParseTwoPartID(data.ID)
	if err != nil {
		resp.Diagnostics.AddError(
			"Invalid resource ID format",
			fmt.Sprintf("The resource ID '%s' has an invalid format. It should be '<project>:<domain>'. Error: %s", data.ID, err.Error()),
		)
		return
	}

	if _, err := d.client.PagesDomains.DeletePagesDomain(projectID, domain); err != nil {
		resp.Diagnostics.AddError(
			"GitLab API Error occurred",
			fmt.Sprintf("Unable to delete pages domain: %s", err.Error()),
		)
	}
}

func (r *gitlabPagesDomainResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}

func (r *gitLabPagesDomainResourceModel) pagesDomainToStateModel(pages *gitlab.PagesDomain) {
	// attributes from api response
	r.Domain = pages.Domain
	r.AutoSslEnabled = pages.AutoSslEnabled
	r.URL = pages.URL
	r.VerificationString = pages.VerificationCode
	r.Verified = pages.Verified

	r.CertificateData.Expired = pages.Certificate.Expired

	//Not yet implemented in go-gitlab.
	//r.CertificateData.Certificate = pages.Certificate.Certificate
}
