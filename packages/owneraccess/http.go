package owneraccess

import (
	"errors"
	"mime"
	"net/http"
	"strings"

	"github.com/MikeO7/kinosail/packages/httpguard"
)

// HTTP keeps browser and API mutations on the same validated management operations.
type HTTP struct {
	Manager *Manager
	OwnerID func(*http.Request) string
	JSON    func(http.ResponseWriter, any, int)
	Error   func(http.ResponseWriter, *http.Request, string, int)
}

func (h HTTP) Status(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if h.Manager == nil {
		h.Error(w, r, "private management is unavailable", http.StatusServiceUnavailable)
		return
	}
	h.JSON(w, h.Manager.Status(), http.StatusOK)
}

// Change accepts a fixed operation chosen by the route, never by request data.
func (h HTTP) Change(operation string, api bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		if h.Manager == nil {
			h.Error(w, r, "private management is unavailable", http.StatusServiceUnavailable)
			return
		}
		if (operation == "enable" || operation == "pair") && ProfileID(r) != "" {
			h.Error(w, r, "pair and enable management from your home network", http.StatusForbidden)
			return
		}
		var value string
		var err error
		if api {
			switch operation {
			case "enable":
				var input struct {
					Endpoint string `json:"endpoint"`
				}
				if !h.readJSON(w, r, &input) {
					return
				}
				value = input.Endpoint
			case "pair":
				var input struct {
					Label string `json:"label"`
				}
				if !h.readJSON(w, r, &input) {
					return
				}
				value = input.Label
			case "revoke":
				var input struct {
					PublicKey string `json:"publicKey"`
				}
				if !h.readJSON(w, r, &input) {
					return
				}
				value = input.PublicKey
			case "disable":
				var input struct{}
				if !h.readJSON(w, r, &input) {
					return
				}
			default:
				h.Error(w, r, "invalid management operation", http.StatusBadRequest)
				return
			}
		} else {
			value, err = formValue(w, r, operation)
			if err != nil {
				h.Error(w, r, err.Error(), http.StatusBadRequest)
				return
			}
		}
		var profile string
		switch operation {
		case "enable":
			err = h.Manager.Enable(value)
		case "disable":
			err = h.Manager.Disable()
		case "pair":
			profile, err = h.Manager.Pair(value, h.OwnerID(r))
		case "revoke":
			err = h.Manager.Revoke(value)
		default:
			err = errors.New("invalid management operation")
		}
		if err != nil {
			h.Error(w, r, err.Error(), http.StatusBadRequest)
			return
		}
		if api {
			if profile != "" {
				h.JSON(w, map[string]string{"profile": profile}, http.StatusCreated)
			} else {
				h.JSON(w, h.Manager.Status(), http.StatusOK)
			}
		} else if profile != "" {
			w.Header().Set("Content-Type", "application/x-wireguard-profile")
			w.Header().Set("Content-Disposition", `attachment; filename="kinosail-owner.conf"`)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(profile))
		} else {
			http.Redirect(w, r, "/settings/management", http.StatusSeeOther)
		}
	}
}

func formValue(w http.ResponseWriter, r *http.Request, operation string) (string, error) {
	if !httpguard.FormEncoded(r) || r.URL.RawQuery != "" || r.URL.ForceQuery || r.ContentLength > 4096 {
		return "", errors.New("management form is invalid")
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if r.ParseForm() != nil {
		return "", errors.New("management form is invalid")
	}
	key := map[string]string{"enable": "endpoint", "pair": "label", "revoke": "publicKey", "disable": ""}[operation]
	for name, values := range r.PostForm {
		if len(values) != 1 || (name != key && name != "_csrf") || len(strings.Join(values, "")) > 1024 {
			return "", errors.New("management form is invalid")
		}
	}
	if key != "" && len(r.PostForm[key]) != 1 {
		return "", errors.New("management form is incomplete")
	}
	return r.PostForm.Get(key), nil
}

func (h HTTP) readJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" || r.URL.RawQuery != "" || r.URL.ForceQuery || httpguard.DecodeUniqueJSON(http.MaxBytesReader(w, r.Body, 4096), 4096, target) != nil {
		h.Error(w, r, "management request is invalid", http.StatusBadRequest)
		return false
	}
	return true
}
