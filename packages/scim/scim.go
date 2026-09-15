package scim

import (
	"crypto/sha256"
	"crypto/subtle"
	"mime"
	"net/http"
	"strings"
	"time"
)

const (
	scimUserSchema      = "urn:ietf:params:scim:schemas:core:2.0:User"
	scimEnterpriseUser  = "urn:ietf:params:scim:schemas:extension:enterprise:2.0:User"
	scimPatchSchema     = "urn:ietf:params:scim:api:messages:2.0:PatchOp"
	scimListSchema      = "urn:ietf:params:scim:api:messages:2.0:ListResponse"
	scimResourceSchema  = "urn:ietf:params:scim:schemas:core:2.0:ResourceType"
	scimSchemaSchema    = "urn:ietf:params:scim:schemas:core:2.0:Schema"
	scimServiceSchema   = "urn:ietf:params:scim:schemas:core:2.0:ServiceProviderConfig"
	maxSCIMBody         = 1 << 20
	maxSCIMPageSize     = 200
	maxSCIMPatchActions = 32
	maxSCIMQueryLength  = 4096
	// MaxBodySize is the maximum accepted identity protocol document size.
	MaxBodySize = maxSCIMBody
)

type scimAPI struct {
	config     Config
	repository Repository
}

// Register installs the complete SCIM 2.0 HTTP interface on mux.
func Register(mux *http.ServeMux, config Config, repository Repository) {
	api := &scimAPI{config: config, repository: repository}
	api.register(mux)
}

func (api *scimAPI) register(mux *http.ServeMux) {
	mux.Handle(serviceProviderPattern, api.protect(http.HandlerFunc(api.serviceProviderConfig)))
	mux.Handle(resourceTypesPattern, api.protect(http.HandlerFunc(api.resourceTypes)))
	mux.Handle(resourceTypePattern, api.protect(http.HandlerFunc(api.resourceType)))
	mux.Handle(schemasPattern, api.protect(http.HandlerFunc(api.schemas)))
	mux.Handle(schemaPattern, api.protect(http.HandlerFunc(api.schema)))
	mux.Handle(usersPattern, api.protect(http.HandlerFunc(api.listUsers)))
	mux.Handle(createUserPattern, api.protect(http.HandlerFunc(api.createProfile)))
	mux.Handle(userPattern, api.protect(http.HandlerFunc(api.getProfile)))
	mux.Handle(replaceUserPattern, api.protect(http.HandlerFunc(api.replaceProfile)))
	mux.Handle(patchUserPattern, api.protect(http.HandlerFunc(api.patchProfile)))
	mux.Handle(deleteUserPattern, api.protect(http.HandlerFunc(api.deleteProfile)))
}

func (api *scimAPI) protect(next http.Handler) http.Handler { //nolint:cyclop // Authentication failures stay in wire-protocol order.
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer = scimResponseWriter{ResponseWriter: writer, json: scimWantsJSON(request)}
		if api.config.Token == "" {
			writeSCIMError(writer, http.StatusNotFound, "", "SCIM provisioning is not configured")
			return
		}
		if !secureRequest(request) {
			writeSCIMError(writer, http.StatusBadRequest, "", "SCIM provisioning requires HTTPS")
			return
		}
		if !api.config.TokenExpiresAt.After(time.Now()) {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="Kinosail SCIM"`)
			writeSCIMError(writer, http.StatusUnauthorized, "", "valid SCIM bearer authentication is required")
			return
		}
		authorizations := request.Header.Values("Authorization")
		if len(authorizations) != 1 {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="Kinosail SCIM"`)
			writeSCIMError(writer, http.StatusBadRequest, "invalidSyntax", "Authorization must appear once")
			return
		}
		authorization := authorizations[0]
		scheme, token, found := strings.Cut(authorization, " ")
		candidate, expected := sha256.Sum256([]byte(token)), sha256.Sum256([]byte(api.config.Token))
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" || len(token) > 256 || strings.ContainsAny(token, " \t\r\n") || subtle.ConstantTimeCompare(candidate[:], expected[:]) != 1 {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="Kinosail SCIM"`)
			writeSCIMError(writer, http.StatusUnauthorized, "", "valid SCIM bearer authentication is required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

type scimResponseWriter struct {
	http.ResponseWriter
	json bool
}

func (writer scimResponseWriter) WriteHeader(status int) {
	if writer.json && writer.Header().Get("Content-Type") == "application/scim+json" {
		writer.Header().Set("Content-Type", "application/json")
	}
	writer.ResponseWriter.WriteHeader(status)
}

func scimWantsJSON(request *http.Request) bool {
	for _, header := range append(request.Header.Values("Accept"), request.Header.Get("Content-Type")) {
		for _, value := range strings.Split(header, ",") {
			mediaType, _, err := mime.ParseMediaType(strings.TrimSpace(value))
			if err == nil && mediaType == "application/json" {
				return true
			}
		}
	}
	return false
}

func (api *scimAPI) listUsers(writer http.ResponseWriter, request *http.Request) {
	options, err := scimListOptions(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	profiles := make([]viewerProfile, 0)
	for _, profile := range api.repository.List() {
		if options.filter == nil || options.filter(profile) {
			profiles = append(profiles, profile)
		}
	}
	first := min(options.start-1, len(profiles))
	last := min(first+options.count, len(profiles))
	resources := make([]map[string]any, 0, last-first)
	for _, profile := range profiles[first:last] {
		resources = append(resources, scimResourceValue(profile, options.selection))
	}
	writeSCIMJSON(writer, map[string]any{"schemas": []string{scimListSchema}, "totalResults": len(profiles), "startIndex": options.start, "itemsPerPage": len(resources), "Resources": resources}, http.StatusOK)
}

func (api *scimAPI) createProfile(writer http.ResponseWriter, request *http.Request) {
	selection, err := scimResourceOptions(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	var payload scimUserPayload
	if !decodeSCIMBody(writer, request, &payload, scimUserFields...) {
		return
	}
	if payload.ID != "" {
		payload.ID = ""
	}
	input, err := payload.profile(true)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	profile, err := api.repository.Create(input) //nolint:contextcheck // The validated SCIM write and related session revocation must commit together.
	if err != nil {
		writeSCIMStoreError(writer, err)
		return
	}
	writeSCIMResource(writer, profile, selection, http.StatusCreated, true)
}

func (api *scimAPI) getProfile(writer http.ResponseWriter, request *http.Request) {
	id, err := scimPathID(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	selection, err := scimResourceOptions(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	profile, found := api.repository.Get(id)
	if !found {
		writeSCIMError(writer, http.StatusNotFound, "", "SCIM Viewer Profile was not found")
		return
	}
	if scimConditional(writer, request, profile, false) {
		return
	}
	writeSCIMResource(writer, profile, selection, http.StatusOK, false)
}

func (api *scimAPI) replaceProfile(writer http.ResponseWriter, request *http.Request) {
	id, err := scimPathID(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	selection, err := scimResourceOptions(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	var payload scimUserPayload
	if !decodeSCIMBody(writer, request, &payload, scimUserFields...) {
		return
	}
	current, found := api.repository.Get(id)
	if !found {
		writeSCIMError(writer, http.StatusNotFound, "", "SCIM Viewer Profile was not found")
		return
	}
	if scimConditional(writer, request, current, true) {
		return
	}
	expected := strings.TrimSpace(request.Header.Get("If-Match"))
	input, err := payload.profile(true)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	profile, err := api.repository.Update(id, input, expected) //nolint:contextcheck // The validated SCIM write must finish durably after client cancellation.
	if err != nil {
		writeSCIMStoreError(writer, err)
		return
	}
	writeSCIMResource(writer, profile, selection, http.StatusOK, false)
}

func (api *scimAPI) patchProfile(writer http.ResponseWriter, request *http.Request) { //nolint:cyclop // Request validation precedes the single atomic patch operation.
	id, err := scimPathID(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	selection, err := scimResourceOptions(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	profile, found := api.repository.Get(id)
	if !found {
		writeSCIMError(writer, http.StatusNotFound, "", "SCIM Viewer Profile was not found")
		return
	}
	if scimConditional(writer, request, profile, true) {
		return
	}
	expected := strings.TrimSpace(request.Header.Get("If-Match"))
	var payload scimPatchPayload
	if !decodeSCIMBody(writer, request, &payload, "schemas", "operations") {
		return
	}
	if err := validateSCIMSchema(payload.Schemas, scimPatchSchema); err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	if len(payload.Operations) == 0 || len(payload.Operations) > maxSCIMPatchActions {
		writeSCIMError(writer, http.StatusBadRequest, "invalidSyntax", "a bounded SCIM PatchOp document is required")
		return
	}
	input, err := applySCIMPatch(profile, payload.Operations)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	profile, err = api.repository.Update(id, input, expected) //nolint:contextcheck // The validated SCIM write must finish durably after client cancellation.
	if err != nil {
		writeSCIMStoreError(writer, err)
		return
	}
	writeSCIMResource(writer, profile, selection, http.StatusOK, false)
}

func (api *scimAPI) deleteProfile(writer http.ResponseWriter, request *http.Request) {
	id, err := scimPathID(request)
	if err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	if _, err := scimQueryValues(request); err != nil {
		writeSCIMValidationError(writer, err)
		return
	}
	profile, found := api.repository.Get(id)
	if !found {
		writeSCIMError(writer, http.StatusNotFound, "", "SCIM Viewer Profile was not found")
		return
	}
	if scimConditional(writer, request, profile, true) {
		return
	}
	expected := strings.TrimSpace(request.Header.Get("If-Match"))
	if err := api.repository.Delete(id, expected); err != nil { //nolint:contextcheck // Deprovisioning, precondition, and session revocation must commit together.
		writeSCIMStoreError(writer, err)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func secureRequest(request *http.Request) bool {
	return request.TLS != nil || strings.EqualFold(strings.TrimSpace(strings.Split(request.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}
