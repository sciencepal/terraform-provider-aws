package opensearch

import (
	"context"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/opensearchservice"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/structure"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/hashicorp/terraform-provider-aws/internal/conns"
	"github.com/hashicorp/terraform-provider-aws/internal/errs/sdkdiag"
	"github.com/hashicorp/terraform-provider-aws/internal/tfresource"
	"github.com/hashicorp/terraform-provider-aws/internal/verify"
)

func ResourceDomainPolicy() *schema.Resource {
	return &schema.Resource{
		CreateWithoutTimeout: resourceDomainPolicyUpsert,
		ReadWithoutTimeout:   resourceDomainPolicyRead,
		UpdateWithoutTimeout: resourceDomainPolicyUpsert,
		DeleteWithoutTimeout: resourceDomainPolicyDelete,

		Timeouts: &schema.ResourceTimeout{
			Update: schema.DefaultTimeout(180 * time.Minute),
			Delete: schema.DefaultTimeout(90 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"domain_name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"access_policies": {
				Type:             schema.TypeString,
				Required:         true,
				ValidateFunc:     validation.StringIsJSON,
				DiffSuppressFunc: verify.SuppressEquivalentPolicyDiffs,
				StateFunc: func(v interface{}) string {
					json, _ := structure.NormalizeJsonString(v)
					return json
				},
			},
		},
	}
}

func resourceDomainPolicyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).OpenSearchConn()

	ds, err := FindDomainByName(ctx, conn, d.Get("domain_name").(string))

	if !d.IsNewResource() && tfresource.NotFound(err) {
		log.Printf("[WARN] OpenSearch Domain Policy (%s) not found, removing from state", d.Id())
		d.SetId("")
		return diags
	}

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OpenSearch Domain Policy (%s): %s", d.Id(), err)
	}

	policies, err := verify.PolicyToSet(d.Get("access_policies").(string), aws.StringValue(ds.AccessPolicies))

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "reading OpenSearch Domain Policy (%s): %s", d.Id(), err)
	}

	d.Set("access_policies", policies)

	return diags
}

func resourceDomainPolicyUpsert(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).OpenSearchConn()
	domainName := d.Get("domain_name").(string)

	policy, err := structure.NormalizeJsonString(d.Get("access_policies").(string))

	if err != nil {
		return sdkdiag.AppendErrorf(diags, "policy (%s) is invalid JSON: %s", policy, err)
	}

	_, err = conn.UpdateDomainConfigWithContext(ctx, &opensearchservice.UpdateDomainConfigInput{
		DomainName:     aws.String(domainName),
		AccessPolicies: aws.String(policy),
	})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "updating OpenSearch Domain Policy (%s): %s", d.Id(), err)
	}

	d.SetId("esd-policy-" + domainName)

	if err := waitForDomainUpdate(ctx, conn, d.Get("domain_name").(string), d.Timeout(schema.TimeoutUpdate)); err != nil {
		return sdkdiag.AppendErrorf(diags, "updating OpenSearch Domain Policy (%s): waiting for completion: %s", d.Id(), err)
	}

	return append(diags, resourceDomainPolicyRead(ctx, d, meta)...)
}

func resourceDomainPolicyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics
	conn := meta.(*conns.AWSClient).OpenSearchConn()

	_, err := conn.UpdateDomainConfigWithContext(ctx, &opensearchservice.UpdateDomainConfigInput{
		DomainName:     aws.String(d.Get("domain_name").(string)),
		AccessPolicies: aws.String(""),
	})
	if err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OpenSearch Domain Policy (%s): %s", d.Id(), err)
	}

	log.Printf("[DEBUG] Waiting for OpenSearch domain policy %q to be deleted", d.Get("domain_name").(string))

	if err := waitForDomainUpdate(ctx, conn, d.Get("domain_name").(string), d.Timeout(schema.TimeoutDelete)); err != nil {
		return sdkdiag.AppendErrorf(diags, "deleting OpenSearch Domain Policy (%s): waiting for completion: %s", d.Id(), err)
	}

	return diags
}
