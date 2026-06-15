package server

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"github.com/threestoneliu/kubernetes-agent/internal/store"
)

// clusterView is the public projection of a stored cluster. The
// kubeconfig blob is never returned over the API — it would
// trivially leak the credential if a backup or proxy was
// misconfigured.
type clusterView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Server    string `json:"server"`
	User      string `json:"user"`
	CreatedAt int64  `json:"created_at"`
	UpdatedAt int64  `json:"updated_at"`
}

func toClusterView(c store.Cluster) clusterView {
	return clusterView{
		ID:        c.ID,
		Name:      c.Name,
		Server:    c.Server,
		User:      c.User,
		CreatedAt: c.CreatedAt.Unix(),
		UpdatedAt: c.UpdatedAt.Unix(),
	}
}

func listClustersHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rows, err := d.DB.ListClusters(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		out := make([]clusterView, 0, len(rows))
		for _, c := range rows {
			out = append(out, toClusterView(c))
		}
		writeJSON(w, http.StatusOK, map[string]any{"clusters": out})
	}
}

type createClusterReq struct {
	Name       string `json:"name"`
	Kubeconfig string `json:"kubeconfig"`
}

func createClusterHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createClusterReq
		if !decodeJSON(w, r, &req) {
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "name is required", false)
			return
		}
		if req.Kubeconfig == "" {
			writeError(w, http.StatusBadRequest, "validation_error", "kubeconfig is required", false)
			return
		}
		// Parse the kubeconfig client-side so a bad upload fails
		// fast (400) rather than at the first tool call (500).
		cfg, err := clientcmd.Load([]byte(req.Kubeconfig))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_kubeconfig", err.Error(), false)
			return
		}
		server, user := firstServerAndUser(cfg)
		// Encrypt before persisting so a leaked database file or
		// backup doesn't expose the cluster credential.
		blob, err := d.AEAD.Encrypt([]byte(req.Kubeconfig))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		row := store.Cluster{
			ID:             uuid.NewString(),
			Name:           req.Name,
			Server:         server,
			User:           user,
			KubeconfigBlob: blob,
		}
		if err := d.DB.CreateCluster(r.Context(), row); err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		if d.Factory != nil {
			d.Factory.Invalidate(row.ID)
		}
		writeJSON(w, http.StatusCreated, toClusterView(row))
	}
}

func deleteClusterHandler(d Deps) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		err := d.DB.DeleteCluster(r.Context(), id)
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "not_found", "cluster not found", false)
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal", err.Error(), true)
			return
		}
		if d.Factory != nil {
			d.Factory.Invalidate(id)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// firstServerAndUser returns the first server URL + user from a
// parsed kubeconfig. Used to populate the list view's "server"
// field so the user can tell clusters apart at a glance.
func firstServerAndUser(cfg *clientcmdapi.Config) (string, string) {
	if cfg == nil || len(cfg.Clusters) == 0 {
		return "", ""
	}
	// Prefer the current-context's cluster if it exists, so the
	// displayed server matches what the user actually targets.
	if cfg.CurrentContext != "" {
		if ctx, ok := cfg.Contexts[cfg.CurrentContext]; ok && ctx != nil {
			if c, ok := cfg.Clusters[ctx.Cluster]; ok && c != nil {
				user := ""
				if a, ok := cfg.AuthInfos[ctx.AuthInfo]; ok && a != nil {
					user = ctx.AuthInfo
				}
				return c.Server, user
			}
		}
	}
	for _, c := range cfg.Clusters {
		return c.Server, ""
	}
	return "", ""
}
