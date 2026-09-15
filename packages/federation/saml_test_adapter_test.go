package federation

import (
	"net/http"
	"time"

	"github.com/crewjam/saml"
)

type samlTestServiceProvider struct{ metadata *saml.EntityDescriptor }

func (provider *samlTestServiceProvider) GetServiceProvider(_ *http.Request, _ string) (*saml.EntityDescriptor, error) {
	return provider.metadata, nil
}

type samlTestSessionProvider struct{}

func (samlTestSessionProvider) GetSession(_ http.ResponseWriter, _ *http.Request, _ *saml.IdpAuthnRequest) *saml.Session {
	return &saml.Session{ID: "session", CreateTime: time.Now(), ExpireTime: time.Now().Add(time.Hour), NameID: "subject", NameIDFormat: string(saml.PersistentNameIDFormat), UserName: "viewer@example.com", CustomAttributes: []saml.Attribute{{Name: "objectGUID", Values: []saml.AttributeValue{{Type: "xs:string", Value: "directory-1"}}}}}
}
