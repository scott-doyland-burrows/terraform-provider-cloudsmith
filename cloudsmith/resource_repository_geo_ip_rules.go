package cloudsmith

import (
	"fmt"
	"github.com/cloudsmith-io/cloudsmith-api-go"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

const Namespace string = "namespace"
const Repository string = "repository"
const CidrAllow string = "cidr_allow"
const CidrDeny string = "cidr_deny"
const CountryCodeAllow string = "country_code_allow"
const CountryCodeDeny string = "country_code_deny"

func resourceRepositoryGeoIpRulesCreate(d *schema.ResourceData, m interface{}) error {
	pc := m.(*providerConfig)

	namespace := requiredString(d, Namespace)
	repository := requiredString(d, Repository)

	// Ensure that Geo/IP rules are enabled for the Repository
	req := pc.APIClient.ReposApi.ReposGeoipEnable(pc.Auth, namespace, repository)
	_, err := pc.APIClient.ReposApi.ReposGeoipEnableExecute(req)
	if err != nil {
		return err
	}

	// The actual "create" is just the same as "update" for this resource.
	return resourceRepositoryGeoIpRulesUpdate(d, m)
}

func resourceRepositoryGeoIpRulesRead(d *schema.ResourceData, m interface{}) error {
	pc := m.(*providerConfig)

	namespace := requiredString(d, Namespace)
	repository := requiredString(d, Repository)

	req := pc.APIClient.ReposApi.ReposGeoipRead(pc.Auth, namespace, repository)

	geoIpRules, resp, err := pc.APIClient.ReposApi.ReposGeoipReadExecute(req)
	if err != nil {
		if is404(resp) {
			d.SetId("")
			return nil
		}

		return err
	}

	cidr := geoIpRules.GetCidr()
	countryCode := geoIpRules.GetCountryCode()

	_ = d.Set(CidrAllow, flattenStrings(cidr.GetAllow()))
	_ = d.Set(CidrDeny, flattenStrings(cidr.GetDeny()))
	_ = d.Set(CountryCodeAllow, flattenStrings(countryCode.GetAllow()))
	_ = d.Set(CountryCodeDeny, flattenStrings(countryCode.GetDeny()))

	// namespace and repository are not returned from the read
	// endpoint, so we can use the values stored in resource state. We rely on
	// ForceNew to ensure if either changes a new resource is created.
	_ = d.Set(Namespace, namespace)
	_ = d.Set(Repository, repository)

	return nil
}

func resourceRepositoryGeoIpRulesUpdate(d *schema.ResourceData, m interface{}) error {
	pc := m.(*providerConfig)

	namespace := requiredString(d, Namespace)
	repository := requiredString(d, Repository)

	req := pc.APIClient.ReposApi.ReposGeoipUpdate(pc.Auth, namespace, repository)
	req = req.Data(cloudsmith.ReposGeoipRead200Response{
		CountryCode: &cloudsmith.ReposGeoipRead200ResponseCountryCode{
			Allow: expandStrings(d, CountryCodeAllow),
			Deny:  expandStrings(d, CountryCodeDeny),
		},
		Cidr: &cloudsmith.ReposGeoipRead200ResponseCountryCode{
			Allow: expandStrings(d, CidrAllow),
			Deny:  expandStrings(d, CidrDeny),
		},
	})

	_, err := pc.APIClient.ReposApi.ReposGeoipUpdateExecute(req)
	if err != nil {
		return err
	}

	d.SetId(fmt.Sprintf("%s_%s_geo_ip_rules", namespace, repository))

	return resourceRepositoryGeoIpRulesRead(d, m)
}

func resourceRepositoryGeoIpRulesDelete(d *schema.ResourceData, m interface{}) error {
	pc := m.(*providerConfig)

	namespace := requiredString(d, "namespace")
	repository := requiredString(d, "repository")

	// There isn't a DELETE endpoint, so just update the rules to be empty.
	req := pc.APIClient.ReposApi.ReposGeoipUpdate(pc.Auth, namespace, repository)
	req = req.Data(cloudsmith.ReposGeoipRead200Response{
		CountryCode: &cloudsmith.ReposGeoipRead200ResponseCountryCode{
			Allow: []string{},
			Deny:  []string{},
		},
		Cidr: &cloudsmith.ReposGeoipRead200ResponseCountryCode{
			Allow: []string{},
			Deny:  []string{},
		},
	})
	_, err := pc.APIClient.ReposApi.ReposGeoipUpdateExecute(req)
	if err != nil {
		return err
	}

	return nil
}

//nolint:funlen
func resourceRepositoryGeoIpRules() *schema.Resource {
	return &schema.Resource{
		Create: resourceRepositoryGeoIpRulesCreate,
		Read:   resourceRepositoryGeoIpRulesRead,
		Update: resourceRepositoryGeoIpRulesUpdate,
		Delete: resourceRepositoryGeoIpRulesDelete,

		Schema: map[string]*schema.Schema{
			CidrAllow: {
				Type:        schema.TypeSet,
				Description: "The list of IP Addresses for which to allow access, expressed in CIDR notation.",
				Required:    true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringIsNotEmpty,
				},
			},
			CidrDeny: {
				Type:        schema.TypeSet,
				Description: "The list of IP Addresses for which to deny access, expressed in CIDR notation.",
				Required:    true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringIsNotEmpty,
				},
			},
			CountryCodeAllow: {
				Type:        schema.TypeSet,
				Description: "The list of countries for which to allow access, expressed in ISO 3166-1 country codes.",
				Required:    true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringIsNotEmpty,
				},
			},
			CountryCodeDeny: {
				Type:        schema.TypeSet,
				Description: "The list of countries for which to deny access, expressed in ISO 3166-1 country codes.",
				Required:    true,
				Elem: &schema.Schema{
					Type:         schema.TypeString,
					ValidateFunc: validation.StringIsNotEmpty,
				},
			},
			Namespace: {
				Type:         schema.TypeString,
				Description:  "Organization to which the Repository belongs.",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
			Repository: {
				Type:         schema.TypeString,
				Description:  "Repository to which these Geo/IP rules belong.",
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringIsNotEmpty,
			},
		},
	}
}
