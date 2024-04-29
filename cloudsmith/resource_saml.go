package cloudsmith

import (
	"context"
	"fmt"
	"strings"

	"github.com/cloudsmith-io/cloudsmith-api-go"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func samlImport(ctx context.Context, d *schema.ResourceData, m interface{}) ([]*schema.ResourceData, error) {
	idParts := strings.Split(d.Id(), ".")
	if len(idParts) != 2 {
		return nil, fmt.Errorf(
			"invalid import ID, must be of the form <organization_slug>.<saml_slug_perm>, got: %s", d.Id(),
		)
	}

	d.Set("organization", idParts[0])
	d.SetId(idParts[1])
	return []*schema.ResourceData{d}, nil
}

func samlCreate(d *schema.ResourceData, m interface{}) error {
	pc := m.(*providerConfig)

	organization := requiredString(d, "organization")
	req := pc.APIClient.OrgsApi.OrgsSamlGroupSyncCreate(pc.Auth, organization)
	req = req.Data(cloudsmith.OrganizationGroupSyncRequest{
		IdpKey:       requiredString(d, "idp_key"),
		IdpValue:     requiredString(d, "idp_value"),
		Role:         optionalString(d, "role"), // default to Member
		Team:         requiredString(d, "team"),
		Organization: requiredString(d, "organization"),
	})

	saml, _, err := pc.APIClient.OrgsApi.OrgsSamlGroupSyncCreateExecute(req)
	if err != nil {
		return err
	}

	d.SetId(saml.GetSlugPerm())

	checkerFunc := func() error {
		req := pc.APIClient.OrgsApi.OrgsSamlGroupSyncList(pc.Auth, organization)
		_, resp, err := pc.APIClient.OrgsApi.OrgsSamlGroupSyncListExecute(req)
		if err != nil {
			if resp != nil {
				if is404(resp) {
					return errKeepWaiting
				}
				if resp.StatusCode == 422 {
					return fmt.Errorf("team does not exist, please check that the team exist")
				}
			}
			return err
		}
		return nil
	}

	if err := waiter(checkerFunc, defaultCreationTimeout, defaultCreationInterval); err != nil {
		return fmt.Errorf("error waiting for SAML group sync (%s) to be created: %w", d.Id(), err)
	}

	return samlRead(d, m)
}

func samlRead(d *schema.ResourceData, m interface{}) error {
	pc := m.(*providerConfig)

	organization := requiredString(d, "organization")

	req := pc.APIClient.OrgsApi.OrgsSamlGroupSyncList(pc.Auth, organization)

	// TODO: add a proper loop here to ensure we always get all privs,
	// regardless of how many are configured.
	req = req.Page(1)
	req = req.PageSize(500) // Max page size is 500

	saml, resp, err := pc.APIClient.OrgsApi.OrgsSamlGroupSyncListExecute(req)
	if err != nil {
		if is404(resp) {
			d.SetId("")
			return nil
		}
		return err
	}

	// Iterate over the saml array to find the matching item
	for _, item := range saml {
		if item.GetSlugPerm() == d.Id() {
			d.Set("idp_key", item.IdpKey)
			d.Set("idp_value", item.IdpValue)
			d.Set("role", item.Role)
			d.Set("team", item.Team)
			d.Set("slug_perm", item.SlugPerm)

			// namespace is not returned from the saml group endpoint so we rely on the input value
			d.Set("organization", organization)
			return nil
		}
	}

	// If no matching item is found, unset the ID and return
	d.SetId("")
	return nil
}

func samlDelete(d *schema.ResourceData, m interface{}) error {
	pc := m.(*providerConfig)
	organization := requiredString(d, "organization")

	req := pc.APIClient.OrgsApi.OrgsSamlGroupSyncDelete(pc.Auth, organization, d.Id())
	_, err := pc.APIClient.OrgsApi.OrgsSamlGroupSyncDeleteExecute(req)
	if err != nil {
		return err
	}

	checkerFunc := func() error {
		req := pc.APIClient.OrgsApi.OrgsSamlGroupSyncList(pc.Auth, organization)
		_, resp, err := pc.APIClient.OrgsApi.OrgsSamlGroupSyncListExecute(req)
		if err != nil {
			if resp != nil {
				if is404(resp) {
					return nil
				}
			}
			return err
		}
		return nil
	}

	if err := waiter(checkerFunc, defaultDeletionTimeout, defaultDeletionInterval); err != nil {
		return fmt.Errorf("error waiting for SAML group sync (%s) to be deleted: %w", d.Id(), err)
	}
	return nil
}

// This is a workaround for not having a proper update endpoint for SAML group sync, we are recreating the entry based on new+old values
func samlUpdate(d *schema.ResourceData, m interface{}) error {
	if err := samlDelete(d, m); err != nil {
		return err
	}
	return samlCreate(d, m)
}

func resourceSAML() *schema.Resource {
	return &schema.Resource{
		Create: samlCreate,
		Read:   samlRead,
		Update: samlUpdate,
		Delete: samlDelete,
		Importer: &schema.ResourceImporter{
			StateContext: samlImport,
		},
		Schema: map[string]*schema.Schema{
			"organization": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"idp_key": {
				Type:     schema.TypeString,
				Required: true,
			},
			"idp_value": {
				Type:     schema.TypeString,
				Required: true,
			},
			"role": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "Member",
				ValidateFunc: validation.StringInSlice([]string{"Member", "Manager"}, false),
			},
			"team": {
				Type:     schema.TypeString,
				Required: true,
			},
			"slug_perm": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}
