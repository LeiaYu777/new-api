package common

import (
	"net/url"
	"strings"
)

const (
	defaultOpenSourceModificationSummary = "Lean deployment changes, configuration changes, and service customization."
	defaultOpenSourceLicenseURL          = "https://www.gnu.org/licenses/agpl-3.0.html"
)

// OpenSourceInfo contains public AGPL/source-code disclosure metadata.
// It is intentionally environment-only and must not read customer data,
// production secrets, database DSNs, or local server paths.
type OpenSourceInfo struct {
	Project             string `json:"project"`
	UpstreamProject     string `json:"upstream_project"`
	UpstreamMaintainer  string `json:"upstream_maintainer"`
	UpstreamLicense     string `json:"upstream_license"`
	BasedOn             string `json:"based_on"`
	SourceCodeURL       string `json:"source_code_url"`
	LicenseURL          string `json:"license_url"`
	ModifierName        string `json:"modifier_name"`
	ModifiedVersion     string `json:"modified_version"`
	ModifiedDate        string `json:"modified_date"`
	ModificationSummary string `json:"modification_summary"`
	LegalContactEmail   string `json:"legal_contact_email"`
	Notice              string `json:"notice"`
}

func BuildOpenSourceInfo() OpenSourceInfo {
	upstreamProject := trimEnvOrDefault("UPSTREAM_PROJECT_NAME", "New API")
	upstreamMaintainer := trimEnvOrDefault("UPSTREAM_MAINTAINER", "QuantumNous")
	upstreamLicense := trimEnvOrDefault("UPSTREAM_LICENSE", "GNU Affero General Public License v3.0")
	basedOnProject := trimEnvOrDefault("BASED_ON_PROJECT", "One API")
	basedOnLicense := trimEnvOrDefault("BASED_ON_LICENSE", "MIT License")

	return OpenSourceInfo{
		Project:             "Modified New API",
		UpstreamProject:     upstreamProject,
		UpstreamMaintainer:  upstreamMaintainer,
		UpstreamLicense:     upstreamLicense,
		BasedOn:             basedOnProject + ", " + basedOnLicense,
		SourceCodeURL:       publicHTTPURLOrEmpty(GetEnvOrDefaultString("SOURCE_CODE_URL", "")),
		LicenseURL:          defaultOpenSourceLicenseURL,
		ModifierName:        strings.TrimSpace(GetEnvOrDefaultString("MODIFIER_NAME", "")),
		ModifiedVersion:     strings.TrimSpace(GetEnvOrDefaultString("MODIFIED_VERSION", "")),
		ModifiedDate:        strings.TrimSpace(GetEnvOrDefaultString("MODIFIED_DATE", "")),
		ModificationSummary: trimEnvOrDefault("MODIFICATION_SUMMARY", defaultOpenSourceModificationSummary),
		LegalContactEmail:   strings.TrimSpace(GetEnvOrDefaultString("LEGAL_CONTACT_EMAIL", "")),
		Notice:              "This service is a modified version of New API. The corresponding source code is available to users of this service under AGPLv3.",
	}
}

func trimEnvOrDefault(env string, defaultValue string) string {
	value := strings.TrimSpace(GetEnvOrDefaultString(env, defaultValue))
	if value == "" {
		return defaultValue
	}
	return value
}

func publicHTTPURLOrEmpty(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil {
		return ""
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return ""
	}
	if parsed.Host == "" || parsed.User != nil {
		return ""
	}
	return value
}
