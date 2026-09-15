package scim

import (
	"bytes"
	"encoding/json"
)

func parseEnterprise(data json.RawMessage) (scimEnterpriseProfile, error) { //nolint:cyclop // The standard extension remains strictly bounded and typed.
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return scimEnterpriseProfile{}, nil
	}
	var value struct {
		EmployeeNumber string          `json:"employeeNumber"`
		CostCenter     string          `json:"costCenter"`
		Organization   string          `json:"organization"`
		Division       string          `json:"division"`
		Department     string          `json:"department"`
		Manager        json.RawMessage `json:"manager"`
	}
	if err := unmarshalSCIMObject(data, &value, "employeeNumber", "costCenter", "organization", "division", "department", "manager"); err != nil {
		return scimEnterpriseProfile{}, &scimValidationError{detail: "enterprise user extension is invalid"}
	}
	manager, err := parseSCIMManager(value.Manager)
	if err != nil {
		return scimEnterpriseProfile{}, err
	}
	return normalizeEnterprise(scimEnterpriseProfile{EmployeeNumber: value.EmployeeNumber, CostCenter: value.CostCenter, Organization: value.Organization, Division: value.Division, Department: value.Department, Manager: manager})
}

func normalizeEnterprise(value scimEnterpriseProfile) (scimEnterpriseProfile, error) {
	var err error
	for name, target := range map[string]*string{"employeeNumber": &value.EmployeeNumber, "costCenter": &value.CostCenter, "organization": &value.Organization, "division": &value.Division, "department": &value.Department, "manager value": &value.Manager.Value, "manager display name": &value.Manager.DisplayName} {
		*target, err = scimString(*target, 256, false, name)
		if err != nil {
			return scimEnterpriseProfile{}, err
		}
	}
	if value.Manager.Ref, err = scimString(value.Manager.Ref, 2048, false, "manager reference"); err != nil {
		return scimEnterpriseProfile{}, err
	}
	return value, nil
}

func parseSCIMManager(data json.RawMessage) (scimProfileManager, error) {
	if len(data) == 0 || bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		return scimProfileManager{}, nil
	}
	var text string
	if json.Unmarshal(data, &text) == nil {
		return scimProfileManager{Value: text}, nil
	}
	var manager scimProfileManager
	if err := unmarshalSCIMObject(data, &manager, "value", "$ref", "displayName"); err != nil {
		return scimProfileManager{}, &scimValidationError{detail: "enterprise user manager is invalid"}
	}
	return manager, nil
}
